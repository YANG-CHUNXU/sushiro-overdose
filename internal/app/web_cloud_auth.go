package app

import (
	"encoding/json"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

type cloudAuthSaveRequest struct {
	BaseURL string `json:"base_url"`
}

func handleCloudAuth(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		verify := truthyQuery(r.URL.Query().Get("verify"))
		writeJSON(w, BuildCloudAuthStatus(r.Context(), verify))
	case http.MethodPost:
		var body cloudAuthSaveRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		current := LoadCloudAuthConfig()
		next := current
		next.BaseURL = body.BaseURL
		if normalizeCloudBaseURL(next.BaseURL) != normalizeCloudBaseURL(current.BaseURL) {
			next.SessionToken = ""
			next.UserLogin = ""
			next.UserName = ""
			next.AvatarURL = ""
			next.ConnectedAt = ""
			next.ExpiresAt = ""
			next.LastVerifiedAt = ""
		}
		if err := SaveCloudAuthConfig(next); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, BuildCloudAuthStatus(r.Context(), false))
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST only")
	}
}

func handleCloudAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	cfg := LoadCloudAuthConfig()
	if !cfg.configured() {
		writeError(w, http.StatusBadRequest, "请先填写 Cloudflare Worker URL")
		return
	}
	state, err := newCloudOAuthState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成登录 state 失败")
		return
	}
	q := neturl.Values{}
	q.Set("return_to", localCloudCallbackURL(r))
	q.Set("state", state)
	loginURL, err := cfg.endpoint("/auth/github/start", q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func handleCloudAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	q := r.URL.Query()
	if errMsg := strings.TrimSpace(q.Get("error")); errMsg != "" {
		redirectCloudAuthResult(w, r, "云端登录失败："+errMsg)
		return
	}
	state := strings.TrimSpace(q.Get("state"))
	if !consumeCloudOAuthState(state) {
		redirectCloudAuthResult(w, r, "云端登录 state 已失效，请重试")
		return
	}
	token := strings.TrimSpace(q.Get("token"))
	if token == "" {
		redirectCloudAuthResult(w, r, "云端登录回调缺少 session")
		return
	}
	cfg := LoadCloudAuthConfig()
	cfg.SessionToken = token
	cfg.UserLogin = strings.TrimSpace(q.Get("login"))
	cfg.UserName = strings.TrimSpace(q.Get("name"))
	cfg.AvatarURL = strings.TrimSpace(q.Get("avatar_url"))
	cfg.ExpiresAt = strings.TrimSpace(q.Get("expires_at"))
	cfg.ConnectedAt = time.Now().Format(time.RFC3339)
	cfg.LastVerifiedAt = ""
	if me, err := fetchCloudMe(r.Context(), cfg); err == nil {
		cfg.UserLogin = me.User.Login
		cfg.UserName = me.User.Name
		cfg.AvatarURL = me.User.AvatarURL
		cfg.ExpiresAt = me.Session.ExpiresAt
		cfg.LastVerifiedAt = time.Now().Format(time.RFC3339)
	}
	if err := SaveCloudAuthConfig(cfg); err != nil {
		redirectCloudAuthResult(w, r, "保存云端会话失败："+err.Error())
		return
	}
	redirectCloudAuthResult(w, r, "")
}

func handleCloudAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	cfg := LoadCloudAuthConfig()
	cfg.SessionToken = ""
	cfg.UserLogin = ""
	cfg.UserName = ""
	cfg.AvatarURL = ""
	cfg.ConnectedAt = ""
	cfg.ExpiresAt = ""
	cfg.LastVerifiedAt = ""
	if err := SaveCloudAuthConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, BuildCloudAuthStatus(r.Context(), false))
}

func handleCloudAuthTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	status := BuildCloudAuthStatus(r.Context(), true)
	if status.LastError != "" {
		writeJSONStatus(w, http.StatusBadGateway, status)
		return
	}
	writeJSON(w, status)
}

func redirectCloudAuthResult(w http.ResponseWriter, r *http.Request, errMsg string) {
	var target string
	if errMsg != "" {
		target = "/?cloud_error=" + neturl.QueryEscape(errMsg) + "#se"
	} else {
		target = "/?cloud_connected=1#se"
	}
	http.Redirect(w, r, target, http.StatusFound)
}
