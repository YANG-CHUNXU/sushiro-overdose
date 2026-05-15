package main

import (
	"encoding/json"
	"net/http"
)

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
