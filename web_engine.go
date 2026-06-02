package main

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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
		"sampling":   sampler.GetState(),
	}
	writeJSON(w, status)
}

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

// handleQueueTicket 远程取号（实验性）。需要已捕获的认证态。
func handleQueueTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var body struct {
		Store string `json:"store"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	storeID := strings.TrimSpace(body.Store)
	if storeID == "" {
		writeError(w, http.StatusBadRequest, "缺少门店 ID")
		return
	}
	refreshWebClient()
	client := getWebClient()
	if client == nil {
		writeError(w, http.StatusBadRequest, "尚未捕获认证参数，请先完成认证再取号")
		return
	}
	ticket, err := client.CreateNetTicket(r.Context(), storeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "ticket": ticket})
}

// handleCancelReservation 取消预约/排队号（按 ticketId，复用 cancelReservation 端点）。
func handleCancelReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var body struct {
		TicketID int64 `json:"ticket_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TicketID == 0 {
		writeError(w, http.StatusBadRequest, "缺少有效的 ticket_id")
		return
	}
	refreshWebClient()
	client := getWebClient()
	if client == nil {
		writeError(w, http.StatusBadRequest, "尚未捕获认证参数，无法取消")
		return
	}
	if err := client.CancelReservation(r.Context(), body.TicketID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

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

func handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, CheckLatestRelease(r.Context()))
}
