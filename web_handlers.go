package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ---- SSE event bus ----

type eventBus struct {
	mu      sync.RWMutex
	chans   map[chan string]struct{}
}

var bus = &eventBus{chans: map[chan string]struct{}{}}

func (b *eventBus) subscribe() chan string {
	ch := make(chan string, 32)
	b.mu.Lock()
	b.chans[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.chans, ch)
	b.mu.Unlock()
}

func (b *eventBus) publish(eventType, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.chans {
		select {
		case ch <- msg:
		default:
		}
	}
}

// ---- Handlers ----

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	pid := readPID()
	status := map[string]any{
		"version": Version,
		"running": isRunning(),
		"pid":     pid,
	}
	writeJSON(w, status)
}

func handleCalendar(w http.ResponseWriter, r *http.Request) {
	if len(webSettings.StoreIDs) == 0 {
		writeError(w, http.StatusServiceUnavailable, "暂无配置，请先运行 sushiro-overdose")
		return
	}

	client := NewClient(webSettings)

	query := r.URL.Query()
	storeID := query.Get("store")
	if storeID == "" {
		storeID = webSettings.StoreIDs[0]
	}

	slots, err := client.GetTimeslots(r.Context(), storeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Enrich with store info
	storeInfo, _ := client.GetStoreInfo(r.Context(), storeID)
	reg := GetStoreRegistry()
	displayName := reg.DisplayName(storeID, storeInfo.Name)

	result := map[string]any{
		"store_id":   storeID,
		"store_name": displayName,
		"slots":      slots,
		"fetched_at": time.Now().Format(time.RFC3339),
	}
	writeJSON(w, result)
	bus.publish("calendar", mustJSON(result))
}

func handleStores(w http.ResponseWriter, r *http.Request) {
	if len(webSettings.StoreIDs) == 0 {
		writeJSON(w, []map[string]string{})
		return
	}
	client := NewClient(webSettings)
	reg := GetStoreRegistry()
	stores := make([]map[string]string, 0)

	for _, id := range webSettings.StoreIDs {
		info, err := client.GetStoreInfo(r.Context(), id)
		name := id
		address := ""
		if err == nil {
			name = info.Name
			address = info.Address
		}
		stores = append(stores, map[string]string{
			"id":        id,
			"name":      name,
			"nickname":  reg.DisplayName(id, name),
			"address":   address,
		})
	}
	writeJSON(w, stores)
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		writeJSON(w, cfg)
		return
	}
}

func handleSniperStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Date string `json:"date"`
		Time string `json:"time"` // "1930-2030"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// This is a simplified API — actual sniper runs in background
	writeJSON(w, map[string]string{"status": "started", "date": req.Date, "time": req.Time})
}

func handleReservations(w http.ResponseWriter, r *http.Request) {
	if len(webSettings.StoreIDs) == 0 {
		writeJSON(w, []ReservationRecord{})
		return
	}
	client := NewClient(webSettings)
	reservations, err := client.GetReservations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, reservations)
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := bus.subscribe()
	defer bus.unsubscribe(ch)

	// Send initial ping
	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			w.Write([]byte(msg))
			flusher.Flush()
		}
	}
}

// ---- Helpers ----

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
