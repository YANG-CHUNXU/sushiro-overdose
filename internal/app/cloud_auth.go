package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

const (
	cloudAuthConfigFile           = "cloud_auth.json"
	cloudAuthURLEnv               = "SUSHIRO_CLOUD_URL"
	cloudAuthSessionTokenEnv      = "SUSHIRO_CLOUD_SESSION_TOKEN"
	defaultCloudAuthBaseURL       = "https://sushiro-cloud.ryujo.online"
	cloudAuthBaselineProbeStoreID = "3006"
	cloudAuthTimeout              = 12 * time.Second
	cloudOAuthStateTTL            = 10 * time.Minute
	cloudAuthSessionTokenSize     = 32
)

type CloudAuthConfig struct {
	BaseURL        string `json:"base_url"`
	SessionToken   string `json:"session_token,omitempty"`
	UserLogin      string `json:"user_login,omitempty"`
	UserName       string `json:"user_name,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	ConnectedAt    string `json:"connected_at,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	LastVerifiedAt string `json:"last_verified_at,omitempty"`
}

type CloudAuthStatus struct {
	Configured          bool   `json:"configured"`
	Connected           bool   `json:"connected"`
	BaseURL             string `json:"base_url,omitempty"`
	UserLogin           string `json:"user_login,omitempty"`
	UserName            string `json:"user_name,omitempty"`
	AvatarURL           string `json:"avatar_url,omitempty"`
	ConnectedAt         string `json:"connected_at,omitempty"`
	ExpiresAt           string `json:"expires_at,omitempty"`
	LastVerifiedAt      string `json:"last_verified_at,omitempty"`
	SessionFromEnv      bool   `json:"session_from_env,omitempty"`
	BaselineConnected   bool   `json:"baseline_connected,omitempty"`
	BaselineStoreCount  int    `json:"baseline_store_count,omitempty"`
	BaselineLatestCount int    `json:"baseline_latest_count,omitempty"`
	BaselineRollupCount int    `json:"baseline_rollup_count,omitempty"`
	BaselineUpdatedAt   string `json:"baseline_updated_at,omitempty"`
	LastError           string `json:"last_error,omitempty"`
	ProviderMessage     string `json:"provider_message,omitempty"`
}

type cloudMeResponse struct {
	OK      bool            `json:"ok"`
	User    cloudUser       `json:"user"`
	Session cloudSession    `json:"session"`
	Error   string          `json:"error,omitempty"`
	Meta    json.RawMessage `json:"meta,omitempty"`
}

type cloudUser struct {
	ID        int64  `json:"id,omitempty"`
	Login     string `json:"login,omitempty"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type cloudSession struct {
	ExpiresAt string `json:"expires_at,omitempty"`
}

var cloudOAuthStates struct {
	sync.Mutex
	values map[string]time.Time
}

func cloudAuthConfigPath() string {
	return filepath.Join(AppDirPath(), cloudAuthConfigFile)
}

func LoadCloudAuthConfig() CloudAuthConfig {
	var cfg CloudAuthConfig
	if data, err := os.ReadFile(cloudAuthConfigPath()); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if value := strings.TrimSpace(os.Getenv(cloudAuthURLEnv)); value != "" {
		cfg.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv(cloudAuthSessionTokenEnv)); value != "" {
		cfg.SessionToken = value
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultCloudAuthBaseURL
	}
	cfg.BaseURL = normalizeCloudBaseURL(cfg.BaseURL)
	cfg.SessionToken = strings.TrimSpace(cfg.SessionToken)
	cfg.UserLogin = strings.TrimSpace(cfg.UserLogin)
	cfg.UserName = strings.TrimSpace(cfg.UserName)
	cfg.AvatarURL = strings.TrimSpace(cfg.AvatarURL)
	cfg.ConnectedAt = strings.TrimSpace(cfg.ConnectedAt)
	cfg.ExpiresAt = strings.TrimSpace(cfg.ExpiresAt)
	cfg.LastVerifiedAt = strings.TrimSpace(cfg.LastVerifiedAt)
	return cfg
}

func SaveCloudAuthConfig(cfg CloudAuthConfig) error {
	cfg.BaseURL = normalizeCloudBaseURL(cfg.BaseURL)
	cfg.SessionToken = strings.TrimSpace(cfg.SessionToken)
	cfg.UserLogin = strings.TrimSpace(cfg.UserLogin)
	cfg.UserName = strings.TrimSpace(cfg.UserName)
	cfg.AvatarURL = strings.TrimSpace(cfg.AvatarURL)
	cfg.ConnectedAt = strings.TrimSpace(cfg.ConnectedAt)
	cfg.ExpiresAt = strings.TrimSpace(cfg.ExpiresAt)
	cfg.LastVerifiedAt = strings.TrimSpace(cfg.LastVerifiedAt)
	if err := validateCloudBaseURL(cfg.BaseURL, true); err != nil {
		return err
	}
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cloudAuthConfigPath(), data, 0o600)
}

func normalizeCloudBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := neturl.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

func validateCloudBaseURL(raw string, allowEmpty bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("缺少 Cloudflare Worker URL")
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("Cloudflare Worker URL 无效")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("Cloudflare Worker URL 只支持 http/https")
	}
	return nil
}

func (cfg CloudAuthConfig) configured() bool {
	return strings.TrimSpace(cfg.BaseURL) != ""
}

func (cfg CloudAuthConfig) connected() bool {
	return cfg.configured() && strings.TrimSpace(cfg.SessionToken) != ""
}

func (cfg CloudAuthConfig) cacheKey() string {
	return "cloud\x00" + strings.TrimSpace(cfg.BaseURL) + "\x00" + strings.TrimSpace(cfg.SessionToken)
}

func (cfg CloudAuthConfig) endpoint(path string, query neturl.Values) (string, error) {
	if err := validateCloudBaseURL(cfg.BaseURL, false); err != nil {
		return "", err
	}
	base, err := neturl.Parse(strings.TrimRight(cfg.BaseURL, "/") + "/")
	if err != nil {
		return "", err
	}
	path = strings.TrimLeft(path, "/")
	u, err := base.Parse(path)
	if err != nil {
		return "", err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func BuildCloudAuthStatus(ctx context.Context, verify bool) CloudAuthStatus {
	cfg := LoadCloudAuthConfig()
	status := cloudAuthStatusFromConfig(cfg)
	if !verify || !cfg.connected() {
		return status
	}
	me, err := fetchCloudMe(ctx, cfg)
	if err != nil {
		status.LastError = err.Error()
		return status
	}
	cfg.UserLogin = me.User.Login
	cfg.UserName = me.User.Name
	cfg.AvatarURL = me.User.AvatarURL
	cfg.ExpiresAt = me.Session.ExpiresAt
	cfg.LastVerifiedAt = time.Now().Format(time.RFC3339)
	if cfg.ConnectedAt == "" {
		cfg.ConnectedAt = cfg.LastVerifiedAt
	}
	_ = SaveCloudAuthConfig(cfg)
	status = cloudAuthStatusFromConfig(cfg)
	if err := applyCloudBaselineProbe(ctx, cfg, &status); err != nil {
		status.LastError = err.Error()
		return status
	}
	return status
}

func cloudAuthStatusFromConfig(cfg CloudAuthConfig) CloudAuthStatus {
	return CloudAuthStatus{
		Configured:      cfg.configured(),
		Connected:       cfg.connected(),
		BaseURL:         cfg.BaseURL,
		UserLogin:       cfg.UserLogin,
		UserName:        cfg.UserName,
		AvatarURL:       cfg.AvatarURL,
		ConnectedAt:     cfg.ConnectedAt,
		ExpiresAt:       cfg.ExpiresAt,
		LastVerifiedAt:  cfg.LastVerifiedAt,
		SessionFromEnv:  strings.TrimSpace(os.Getenv(cloudAuthSessionTokenEnv)) != "",
		ProviderMessage: "GitHub 登录只用于读取线上排队基准；数据库密钥不会写入本机。",
	}
}

func applyCloudBaselineProbe(ctx context.Context, cfg CloudAuthConfig, status *CloudAuthStatus) error {
	export, err := fetchQueueBaselineFromCloud(ctx, cfg, cloudAuthBaselineProbeStoreID, time.Now())
	if err != nil {
		return err
	}
	status.BaselineConnected = true
	status.BaselineStoreCount = export.Stats.StoreCount
	status.BaselineLatestCount = export.Stats.LatestCount
	status.BaselineRollupCount = export.Stats.RollupCount
	status.BaselineUpdatedAt = export.Stats.SourceUpdatedAt
	if status.BaselineStoreCount == 0 {
		status.BaselineStoreCount = countQueueBaselineStores(export)
	}
	if status.BaselineLatestCount == 0 {
		status.BaselineLatestCount = len(export.Latest)
	}
	if status.BaselineRollupCount == 0 {
		status.BaselineRollupCount = len(export.Rollups)
	}
	if status.BaselineUpdatedAt == "" {
		status.BaselineUpdatedAt = export.GeneratedAt
	}
	if status.BaselineLatestCount+status.BaselineRollupCount == 0 {
		status.ProviderMessage = "GitHub 会话可用，线上基准接口已响应；探测门店暂未返回基准数据。"
		return nil
	}
	status.ProviderMessage = "GitHub 登录和线上排队基准均已验证；图表会按门店自动读取 Turso 基准。"
	return nil
}

func countQueueBaselineStores(export QueueBaselineExport) int {
	seen := map[int]bool{}
	for _, store := range export.Stores {
		if store.StoreID > 0 {
			seen[store.StoreID] = true
		}
	}
	for _, latest := range export.Latest {
		if latest.StoreID > 0 {
			seen[latest.StoreID] = true
		}
	}
	for _, rollup := range export.Rollups {
		if rollup.StoreID > 0 {
			seen[rollup.StoreID] = true
		}
	}
	return len(seen)
}

func fetchCloudMe(ctx context.Context, cfg CloudAuthConfig) (cloudMeResponse, error) {
	var out cloudMeResponse
	if err := cloudGETJSON(ctx, cfg, "/api/me", nil, &out); err != nil {
		return out, err
	}
	if !out.OK {
		if out.Error != "" {
			return out, errors.New(out.Error)
		}
		return out, fmt.Errorf("云端会话验证失败")
	}
	return out, nil
}

func cloudGETJSON(ctx context.Context, cfg CloudAuthConfig, path string, query neturl.Values, out any) error {
	return cloudJSON(ctx, http.MethodGet, cfg, path, query, nil, out)
}

func cloudPOSTJSON(ctx context.Context, cfg CloudAuthConfig, path string, query neturl.Values, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return cloudJSON(ctx, http.MethodPost, cfg, path, query, body, out)
}

func cloudJSON(ctx context.Context, method string, cfg CloudAuthConfig, path string, query neturl.Values, body []byte, out any) error {
	if !cfg.connected() {
		return fmt.Errorf("尚未登录云端数据服务")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, cloudAuthTimeout)
	defer cancel()
	endpoint, err := cfg.endpoint(path, query)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(timeoutCtx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.SessionToken))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sushiro-overdose-cloud-client")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 全国基准导出当前约 6-8MB 且随门店数增长；上限给足余量，并在截断时给出明确错误
	// 而不是让 JSON 解析报“unexpected end of JSON input”。
	const cloudRespLimit = 64 * 1024 * 1024
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, cloudRespLimit+1))
	if err != nil {
		return err
	}
	if len(respBody) > cloudRespLimit {
		return fmt.Errorf("云端响应超过 %dMB 上限，已拒绝解析", cloudRespLimit/(1024*1024))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("云端服务 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("解析云端响应失败: %w", err)
	}
	return nil
}

func newCloudOAuthState() (string, error) {
	buf := make([]byte, cloudAuthSessionTokenSize)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(buf)
	now := time.Now()
	cloudOAuthStates.Lock()
	if cloudOAuthStates.values == nil {
		cloudOAuthStates.values = map[string]time.Time{}
	}
	for key, expires := range cloudOAuthStates.values {
		if now.After(expires) {
			delete(cloudOAuthStates.values, key)
		}
	}
	cloudOAuthStates.values[state] = now.Add(cloudOAuthStateTTL)
	cloudOAuthStates.Unlock()
	return state, nil
}

func consumeCloudOAuthState(state string) bool {
	state = strings.TrimSpace(state)
	if state == "" {
		return false
	}
	now := time.Now()
	cloudOAuthStates.Lock()
	defer cloudOAuthStates.Unlock()
	expires, ok := cloudOAuthStates.values[state]
	if ok {
		delete(cloudOAuthStates.values, state)
	}
	return ok && now.Before(expires)
}

func localCloudCallbackURL(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = "127.0.0.1:8081"
	}
	return "http://" + host + "/api/cloud/auth/callback"
}
