package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"encoding/json"
	"errors"
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
		if errors.Is(err, ErrReservationsEndpointUnavailable) {
			writeJSON(w, map[string]any{
				"items":       loadReservationsFallback(),
				"unavailable": true,
				"message":     "官方当前预约列表接口已变更或不可用；这里显示的是本地保存/补录记录，不代表寿司郎小程序里没有预约。",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, reservations)
}

func loadReservationsFallback() []ReservationRecord {
	state, err := LoadState(StateFilePath())
	if err != nil || state.ActiveReservation == nil {
		return []ReservationRecord{}
	}
	reservation := *state.ActiveReservation
	if strings.TrimSpace(reservation.Status) == "" {
		reservation.Status = "本地记录"
	}
	return []ReservationRecord{reservation}
}

func handleLocalReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var body struct {
		Number    string `json:"number"`
		Date      string `json:"date"`
		Start     string `json:"start"`
		End       string `json:"end"`
		StoreID   string `json:"store_id"`
		StoreName string `json:"store_name"`
		TicketID  int64  `json:"ticket_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}
	number := strings.TrimSpace(body.Number)
	date := normalizeLocalReservationDate(body.Date)
	if number == "" && date == "" {
		writeError(w, http.StatusBadRequest, "至少填写预约号或日期")
		return
	}
	start := normalizeLocalReservationTime(body.Start)
	end := normalizeLocalReservationTime(body.End)
	storeID := strings.TrimSpace(body.StoreID)
	storeName := strings.TrimSpace(body.StoreName)
	if storeID == "" {
		ws := getWebSettings()
		if len(ws.StoreIDs) > 0 {
			storeID = ws.StoreIDs[0]
		}
	}
	if storeName == "" && storeID != "" {
		if client := getWebClient(); client != nil {
			if info, err := client.GetStoreInfo(r.Context(), storeID); err == nil {
				storeName = info.Name
			}
		}
	}
	slotLabel := strings.TrimSpace(body.Date)
	if date != "" {
		slotLabel = date
	}
	if start != "" {
		slotLabel = strings.TrimSpace(slotLabel + " " + FormatCompactTime(start))
		if end != "" {
			slotLabel += "-" + FormatCompactTime(end)
		}
	}
	record := ReservationRecord{
		Status:           "本地补录",
		Number:           number,
		QueueDate:        date,
		Start:            start,
		End:              end,
		StoreID:          storeID,
		MonitoredStoreID: storeID,
		StoreName:        storeName,
		SlotLabel:        slotLabel,
		TicketID:         body.TicketID,
	}
	if err := SaveState(StateFilePath(), State{ActiveReservation: &record, SavedAt: time.Now().Format(time.RFC3339)}); err != nil {
		writeError(w, http.StatusInternalServerError, "保存本地预约失败: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "reservation": record})
}

func normalizeLocalReservationDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 10 && value[4] == '-' && value[7] == '-' {
		return strings.ReplaceAll(value, "-", "")
	}
	if len(value) == 8 {
		return value
	}
	return ""
}

func normalizeLocalReservationTime(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ":", "")
	if len(value) == 4 {
		return value + "00"
	}
	if len(value) == 6 {
		return value
	}
	return ""
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
		if isTicketAlreadyIssuedError(err) {
			plan := LoadNetTicketPlan()
			plan.Enabled = true
			plan.StoreID = storeID
			plan.Status = "issued_unknown"
			plan.FiredDate = time.Now().Format("2006-01-02")
			plan.FiredAt = time.Now().Format(time.RFC3339)
			if recovered, ok := recoverExistingNetTicket(r.Context(), client, &plan); ok {
				_ = SaveNetTicketPlan(recovered)
				writeJSON(w, map[string]any{"ok": true, "ticket": recovered, "recovered": true})
				return
			}
			markNetTicketIssuedUnknown(&plan, friendlyNetTicketError(err))
			writeError(w, http.StatusConflict, plan.LastError)
			return
		}
		writeError(w, http.StatusBadGateway, friendlyNetTicketError(err))
		return
	}
	plan := LoadNetTicketPlan()
	plan.Enabled = true
	plan.StoreID = storeID
	plan.FiredDate = time.Now().Format("2006-01-02")
	plan.FiredAt = time.Now().Format(time.RFC3339)
	applyNetTicketSuccess(r.Context(), client, &plan, ticket)
	_ = SaveNetTicketPlan(plan)
	writeJSON(w, map[string]any{"ok": true, "ticket": ticket})
}

func handleQueueTicketStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	refreshWebClient()
	client := getWebClient()
	if client == nil {
		writeError(w, http.StatusBadRequest, "尚未捕获认证参数，请先完成认证")
		return
	}
	ticket, err := client.GetNetTicketStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, friendlyNetTicketError(err))
		return
	}
	plan := LoadNetTicketPlan()
	if strings.TrimSpace(plan.StoreID) == "" {
		plan.StoreID = ticket.StoreID
	}
	applyNetTicketSuccess(r.Context(), client, &plan, ticket)
	_ = SaveNetTicketPlan(plan)
	writeJSON(w, map[string]any{"ok": true, "ticket": ticket, "plan": plan})
}

// handleNetTicketPlan 读取/设置「定时取号」计划。
func handleNetTicketPlan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, LoadNetTicketPlan())
	case http.MethodPost:
		var body struct {
			Enabled    bool   `json:"enabled"`
			Store      string `json:"store"`
			StoreName  string `json:"store_name"`
			TargetTime string `json:"target_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		plan := LoadNetTicketPlan()
		plan.Enabled = body.Enabled
		plan.StoreID = strings.TrimSpace(body.Store)
		plan.StoreName = strings.TrimSpace(body.StoreName)
		plan.TargetTime = strings.TrimSpace(body.TargetTime)
		// 重新设定即重置当天执行状态，允许（重新）到点取号。
		plan.FiredDate = ""
		plan.FiredAt = ""
		plan.Number = ""
		plan.TicketID = 0
		plan.LastError = ""
		if body.Enabled {
			plan.Status = "armed"
		} else {
			plan.Status = "idle"
		}
		clearNetTicketFire(time.Now().Format("2006-01-02"))
		if err := SaveNetTicketPlan(plan); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, plan)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST only")
	}
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
