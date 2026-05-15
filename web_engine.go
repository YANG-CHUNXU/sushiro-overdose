package main

import (
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
