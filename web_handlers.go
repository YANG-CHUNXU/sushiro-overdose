package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type calendarStoreResult struct {
	StoreID   string `json:"store_id"`
	StoreName string `json:"store_name"`
	Slots     []Slot `json:"slots"`
	Error     string `json:"error,omitempty"`
}

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
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(strings.Replace(indexHTML, "{{CSRF_TOKEN}}", getWebCSRFToken(), 1)))
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
	storeIDs := requestedCalendarStores(query["store"], query.Get("stores"), ws.StoreIDs)
	onlyAvailable := query.Get("available") == "1" || strings.EqualFold(query.Get("available"), "true")
	period := strings.ToLower(strings.TrimSpace(query.Get("period")))

	reg := GetStoreRegistry()
	results := make([]calendarStoreResult, 0, len(storeIDs))
	combined := make([]Slot, 0)
	for _, storeID := range storeIDs {
		slots, err := client.GetTimeslots(r.Context(), storeID)
		storeInfo, _ := client.GetStoreInfo(r.Context(), storeID)
		displayName := reg.DisplayName(storeID, storeInfo.Name)
		if displayName == "" {
			displayName = storeID
		}
		result := calendarStoreResult{StoreID: storeID, StoreName: displayName}
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		appendHistory(slots, storeID)
		result.Slots = filterCalendarSlots(slots, onlyAvailable, period)
		results = append(results, result)
		combined = append(combined, result.Slots...)
	}

	result := map[string]any{
		"store_id":   results[0].StoreID,
		"store_name": results[0].StoreName,
		"slots":      results[0].Slots,
		"stores":     results,
		"fetched_at": time.Now().Format(time.RFC3339),
		"filters": map[string]any{
			"available": onlyAvailable,
			"period":    period,
		},
	}
	writeJSON(w, result)
	bus.publish("calendar", mustJSON(result))
}

func requestedCalendarStores(storeValues []string, storesValue string, allowed []string) []string {
	allowedSet := map[string]bool{}
	for _, storeID := range allowed {
		allowedSet[storeID] = true
	}
	var raw []string
	raw = append(raw, storeValues...)
	raw = append(raw, strings.Split(storesValue, ",")...)
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] || !allowedSet[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	if len(out) == 0 && len(allowed) > 0 {
		out = append(out, allowed[0])
	}
	return out
}

func filterCalendarSlots(slots []Slot, onlyAvailable bool, period string) []Slot {
	out := make([]Slot, 0, len(slots))
	for _, slot := range slots {
		if onlyAvailable && strings.ToUpper(slot.Availability) != "AVAILABLE" {
			continue
		}
		if !calendarSlotMatchesPeriod(slot, period) {
			continue
		}
		out = append(out, slot)
	}
	return out
}

func calendarSlotMatchesPeriod(slot Slot, period string) bool {
	switch period {
	case "", "all":
		return true
	case "lunch":
		seconds := parseTimeSeconds(slot.Start)
		return seconds >= 10*3600 && seconds < 16*3600
	case "dinner":
		return parseTimeSeconds(slot.Start) >= 16*3600
	default:
		return true
	}
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
		prefs = NormalizePreferences(prefs)
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

func handleRepairProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	report := RepairProxy()
	status := http.StatusOK
	if !report.OK {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(report)
}

func handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var options UninstallOptions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&options)
	}
	if !uninstallOptionsSelected(options) {
		options.All = true
		options.Certificates = true
		options.SystemCert = true
	}
	repair := dryRunRepairProxyReport()
	if !options.DryRun {
		repair = RepairProxy()
	}
	uninstall := UninstallLocalData(options)
	status := http.StatusOK
	if !repair.OK || !uninstall.OK {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":        repair.OK && uninstall.OK,
		"repair":    repair,
		"uninstall": uninstall,
	})
}

func uninstallOptionsSelected(options UninstallOptions) bool {
	return options.All || options.Config || options.Notify || options.Feishu ||
		options.Preferences || options.Stores || options.State || options.History ||
		options.PID || options.ProxyMarker || options.Certificates || options.SystemCert
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

func handleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	topN := defaultInsightTopN
	if raw := strings.TrimSpace(r.URL.Query().Get("top")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			topN = parsed
		}
	}
	analysis, err := LoadSlotHistoryInsights(topN, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, analysis)
}

// ---- Sniper ----

func handleSniperStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Date    string         `json:"date"`
		Time    string         `json:"time"`
		StoreID string         `json:"store_id"`
		Targets []SniperTarget `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	targets := req.Targets
	if len(targets) == 0 && req.Date != "" && req.Time != "" {
		ws := getWebSettings()
		targets = parseSniperArgs(req.Date, req.Time, req.StoreID, ws.StoreIDs)
	}
	if err := engine.StartSniper(targets); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "狙击计划已开始"})
}

func handleSniperPlan(w http.ResponseWriter, r *http.Request) {
	ws := getWebSettings()
	loc := time.Local
	if ws.Location != nil {
		loc = ws.Location
	}
	switch r.Method {
	case http.MethodGet:
		plan, err := LoadSniperPlan(loc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, RefreshSniperPlan(plan, time.Now().In(loc), loc))
	case http.MethodPost, http.MethodPut:
		var req struct {
			Targets []SniperTarget `json:"targets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		targets := normalizeSniperTargetsForSettings(req.Targets, ws)
		plan := NormalizeSniperPlan(targets, loc)
		if err := SaveSniperPlan(plan, loc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "plan": RefreshSniperPlan(plan, time.Now().In(loc), loc)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
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
	fmt.Fprintf(w, "event: engine\ndata: %s\n\n", mustJSON(engine.GetState()))
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
