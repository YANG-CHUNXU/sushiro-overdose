package app

import (
	"encoding/json"
	"net/http"
	"time"
)

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
	if len(req.Targets) > 0 {
		ws := getWebSettings()
		valid, rejected := validateSniperTargetsForSettings(req.Targets, ws)
		if len(valid) == 0 {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{
				"error":    "没有有效狙击目标",
				"rejected": rejected,
			})
			return
		}
		targets = valid
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
		targets, rejected := validateSniperTargetsForSettings(req.Targets, ws)
		if len(req.Targets) > 0 && len(targets) == 0 {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{
				"error":    "没有有效狙击目标",
				"rejected": rejected,
			})
			return
		}
		plan, err := SaveSniperPlanReplacingTargets(targets, loc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "plan": RefreshSniperPlan(plan, time.Now().In(loc), loc), "rejected": rejected})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}
