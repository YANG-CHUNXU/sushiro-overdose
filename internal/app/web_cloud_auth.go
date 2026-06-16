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
		// 切换 Worker BaseURL 等于切换云端实例：旧实例签发的 session token 在新实例无效，
		// 因此一旦 baseURL 变化（归一化后比较），就清空所有会话字段，强制重新走 OAuth 登录。
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

// handleCloudAuthStart 发起云端（Cloudflare Worker + GitHub）OAuth 登录。
// 流程：本地生成一次性 state（防 CSRF）→ 把 return_to 与 state 拼到 Worker 的 /auth/github/start → 302 跳转过去。
// state 会被记下，回调时必须原样带回并消费成功，否则视作伪造的回调。
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

// handleCloudAuthCallback 处理 OAuth 回调，把云端返回的会话落盘。
// 顺序很关键：先看 Worker 是否回传 error；再 consumeCloudOAuthState 校验并「消费」state
// （一次性，防重放与伪造）；通过后才接受 token。token 拿到后再调 fetchCloudMe 用服务端权威数据
// 覆盖回调里用户可填的字段（login/name/avatar/expires），避免被回调参数污染。
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
	// consumeCloudOAuthState：state 必须存在且本次消费掉，拒绝重复使用 / 无匹配的回调（CSRF 防护核心）。
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
	// 用 Worker 服务端 /me 复核并覆盖用户信息：回调参数可被中间人篡改，服务端凭 token 取回的数据才是权威来源。
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
