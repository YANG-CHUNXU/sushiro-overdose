package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func handleContribution(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, LoadQueueContributionConfig())
	case http.MethodPost, http.MethodPut:
		var cfg QueueContributionConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}
		if err := SaveQueueContributionConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "config": LoadQueueContributionConfig()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
	}
}

func handleContributionPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, BuildQueueContributionPreview(time.Now()))
}

func handleContributionUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	resp, err := UploadQueueContribution(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "collector": resp, "config": LoadQueueContributionConfig()})
}
