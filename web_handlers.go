package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// ---- SSE event bus ----

type eventBus struct {
	mu    sync.RWMutex
	chans map[chan string]struct{}
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

// ---- Static ----

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

// ---- Status ----

func handleStatus(w http.ResponseWriter, r *http.Request) {
	pid := readPID()
	hasConfig := HasValidConfig()
	status := map[string]any{
		"version":    Version,
		"running":    isRunning(),
		"pid":        pid,
		"has_config": hasConfig,
		"platform":   runtime.GOOS,
		"engine":     engine.GetState(),
	}
	writeJSON(w, status)
}

// ---- Calendar ----

func handleCalendar(w http.ResponseWriter, r *http.Request) {
	ws := getWebSettings()
	if len(ws.StoreIDs) == 0 {
		writeError(w, http.StatusServiceUnavailable, "暂无配置，请先完成参数捕获")
		return
	}

	client := getWebClient()
	query := r.URL.Query()
	storeID := query.Get("store")
	if storeID == "" {
		storeID = ws.StoreIDs[0]
	}

	slots, err := client.GetTimeslots(r.Context(), storeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

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

// ---- Stores ----

func handleStores(w http.ResponseWriter, r *http.Request) {
	ws := getWebSettings()
	if len(ws.StoreIDs) == 0 {
		writeJSON(w, []map[string]string{})
		return
	}
	client := getWebClient()
	reg := GetStoreRegistry()
	stores := make([]map[string]string, 0)

	for _, id := range ws.StoreIDs {
		info, err := client.GetStoreInfo(r.Context(), id)
		name := id
		address := ""
		if err == nil {
			name = info.Name
			address = info.Address
		}
		stores = append(stores, map[string]string{
			"id":       id,
			"name":     name,
			"nickname": reg.DisplayName(id, name),
			"address":  address,
		})
	}
	writeJSON(w, stores)
}

// ---- Preferences ----

func handlePreferences(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, LoadPreferences())
	case http.MethodPost, http.MethodPut:
		var prefs UserPreferences
		if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		if prefs.Adult <= 0 && prefs.Child <= 0 {
			prefs.Adult = 2
		}
		if prefs.TableType == "" {
			prefs.TableType = "T"
		}
		if err := SavePreferences(prefs); err != nil {
			writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		refreshWebClient()
		writeJSON(w, map[string]any{"ok": true, "preferences": prefs})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}

// ---- Notification config ----

func handleNotifyConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		writeJSON(w, cfg)
	case http.MethodPost, http.MethodPut:
		var cfg notifyConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		if err := saveNotifyConfig(&cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		setNotifier(BuildNotifierFromConfig())
		writeJSON(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}

// ---- Reservations ----

func handleReservations(w http.ResponseWriter, r *http.Request) {
	ws := getWebSettings()
	if len(ws.StoreIDs) == 0 {
		writeJSON(w, []ReservationRecord{})
		return
	}
	client := getWebClient()
	reservations, err := client.GetReservations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, reservations)
}

// ---- Engine control ----

func handleEngineState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, engine.GetState())
}

func handleEngineCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	if err := engine.StartCapture(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "捕获已开始"})
}

func handleEngineBooking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	refreshWebClient()
	if err := engine.StartBooking(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "抢号已开始"})
}

func handleEngineStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	engine.Stop()
	writeJSON(w, map[string]any{"ok": true, "message": "已停止"})
}

func handleEngineLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, engine.GetLogs())
}

// ---- Sniper ----

func handleSniperStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Date string `json:"date"`
		Time string `json:"time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusNotImplemented, "狙击模式暂时请通过命令行使用: sushiro-overdose sniper")
}

// ---- SSE ----

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
