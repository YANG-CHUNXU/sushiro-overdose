package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TargetSlot represents a timeslot the user wants to book.
type TargetSlot struct {
	StoreID string
	Date    string // compact YYYYMMDD
	Start   string // compact HHMMSS
	End     string // compact HHMMSS
}

// CapturedTokens holds auth parameters intercepted from WeChat mini-program traffic.
type CapturedTokens struct {
	mu              sync.Mutex
	XAppCode        string
	QueryAuth       string
	ReservationAuth string
	UserAgent       string
	Referer         string
	XAppClient      string
	WechatID        string
	PhoneNumber     string
	StoreIDs        []string
	FeishuWebhook   string
}

func newCapturedTokens() *CapturedTokens {
	return &CapturedTokens{}
}

func (t *CapturedTokens) IsComplete() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.IsCompleteUnlocked()
}

// IsCompleteUnlocked checks completeness without locking (caller must hold lock).
func (t *CapturedTokens) IsCompleteUnlocked() bool {
	return t.XAppCode != "" &&
		t.QueryAuth != "" &&
		t.ReservationAuth != "" &&
		t.UserAgent != "" &&
		t.Referer != "" &&
		t.WechatID != "" &&
		t.PhoneNumber != "" &&
		len(t.StoreIDs) > 0
}

func (t *CapturedTokens) Status() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	check := func(v string) string {
		if v != "" {
			return "✅"
		}
		return "⏳"
	}
	return []string{
		fmt.Sprintf("  X-App-Code:       %s %s", check(t.XAppCode), maskToken(t.XAppCode)),
		fmt.Sprintf("  Query Auth:       %s %s", check(t.QueryAuth), maskToken(t.QueryAuth)),
		fmt.Sprintf("  Reservation Auth: %s %s", check(t.ReservationAuth), maskToken(t.ReservationAuth)),
		fmt.Sprintf("  User-Agent:       %s %s", check(t.UserAgent), maskToken(t.UserAgent)),
		fmt.Sprintf("  Referer:          %s %s", check(t.Referer), maskToken(t.Referer)),
		fmt.Sprintf("  Wechat ID:        %s %s", check(t.WechatID), maskToken(t.WechatID)),
		fmt.Sprintf("  Phone Number:     %s %s", check(t.PhoneNumber), maskPhone(t.PhoneNumber)),
		fmt.Sprintf("  Store IDs:        %s %v", check(strings.Join(t.StoreIDs, ",")), t.StoreIDs),
	}
}

func maskToken(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 12 {
		return "***"
	}
	return v[:8] + "..."
}

func localConfigPath() string {
	return filepath.Join(appDirPath(), "config.json")
}

// migrateOldConfig moves the old CWD-based config to ~/.sushiro/ if it exists.
func migrateOldConfig() {
	oldPath := ".sushiro_local.json"
	if _, err := os.Stat(oldPath); err != nil {
		return
	}
	newPath := localConfigPath()
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	os.MkdirAll(appDirPath(), 0o755)
	if os.WriteFile(newPath, data, 0o600) == nil {
		os.Remove(oldPath)
		logMessage(time.Now(), "配置已迁移到 "+newPath)
	}
}

type localConfigJSON struct {
	XAppCode        string   `json:"x_app_code"`
	QueryAuth       string   `json:"query_authorization"`
	ReservationAuth string   `json:"reservation_authorization"`
	UserAgent       string   `json:"user_agent"`
	Referer         string   `json:"referer"`
	XAppClient      string   `json:"x_app_client"`
	WechatID        string   `json:"wechat_id"`
	PhoneNumber     string   `json:"phone_number"`
	StoreIDs        []string `json:"store_ids"`
}

func saveLocalConfig(tokens *CapturedTokens) error {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	data := localConfigJSON{
		XAppCode:        tokens.XAppCode,
		QueryAuth:       tokens.QueryAuth,
		ReservationAuth: tokens.ReservationAuth,
		UserAgent:       tokens.UserAgent,
		Referer:         tokens.Referer,
		XAppClient:      tokens.XAppClient,
		WechatID:        tokens.WechatID,
		PhoneNumber:     tokens.PhoneNumber,
		StoreIDs:        tokens.StoreIDs,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(appDirPath(), 0o755)
	return os.WriteFile(localConfigPath(), raw, 0o600)
}

func loadLocalConfig() (*CapturedTokens, error) {
	raw, err := os.ReadFile(localConfigPath())
	if err != nil {
		return nil, err
	}
	var data localConfigJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &CapturedTokens{
		XAppCode:        data.XAppCode,
		QueryAuth:       data.QueryAuth,
		ReservationAuth: data.ReservationAuth,
		UserAgent:       data.UserAgent,
		Referer:         data.Referer,
		XAppClient:      data.XAppClient,
		WechatID:        data.WechatID,
		PhoneNumber:     data.PhoneNumber,
		StoreIDs:        data.StoreIDs,
		FeishuWebhook:   loadFeishuConfig(),
	}, nil
}

func (t *CapturedTokens) validateForQuery() error {
	missing := t.missingFields(false)
	if len(missing) > 0 {
		return fmt.Errorf("认证参数不完整，缺少: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (t *CapturedTokens) validateForReservation() error {
	missing := t.missingFields(true)
	if len(missing) > 0 {
		return fmt.Errorf("预约参数不完整，缺少: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (t *CapturedTokens) missingFields(reservation bool) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	fields := []struct {
		name  string
		value string
	}{
		{"X-App-Code", t.XAppCode},
		{"查询认证", t.QueryAuth},
		{"User-Agent", t.UserAgent},
		{"Referer", t.Referer},
	}
	if reservation {
		fields = append(fields,
			struct {
				name  string
				value string
			}{"预约认证", t.ReservationAuth},
			struct {
				name  string
				value string
			}{"微信ID", t.WechatID},
			struct {
				name  string
				value string
			}{"手机号", t.PhoneNumber},
		)
	}

	var missing []string
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(t.StoreIDs) == 0 {
		missing = append(missing, "门店")
	}
	return missing
}

func deleteLocalConfig() {
	os.Remove(localConfigPath())
}

// Feishu webhook is stored separately so it survives token refreshes.
func feishuConfigPath() string {
	return filepath.Join(appDirPath(), "feishu.json")
}

func loadFeishuConfig() string {
	data, err := os.ReadFile(feishuConfigPath())
	if err != nil {
		return ""
	}
	var cfg struct {
		Webhook string `json:"webhook"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Webhook)
}

func saveFeishuConfig(webhook string) {
	os.MkdirAll(appDirPath(), 0o755)
	cfg := struct {
		Webhook string `json:"webhook"`
	}{Webhook: strings.TrimSpace(webhook)}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(feishuConfigPath(), data, 0o600)
}

func (t *CapturedTokens) toSettings() Settings {
	return t.toSettingsWithPrefs(LoadPreferences())
}

func (t *CapturedTokens) toSettingsWithPrefs(prefs UserPreferences) Settings {
	t.mu.Lock()
	defer t.mu.Unlock()

	timezone := "Asia/Shanghai"
	location, _ := time.LoadLocation(timezone)

	storeIDs := t.StoreIDs
	if len(prefs.SelectedStores) > 0 {
		storeIDs = prefs.SelectedStores
	}

	adult := prefs.Adult
	if adult <= 0 {
		adult = 2
	}

	tableType := prefs.TableType
	if tableType == "" {
		tableType = "T"
	}

	// 偏好里预填的手机号/微信ID 优先于捕获值，方便取号前提前配置。
	phoneNumber := t.PhoneNumber
	if v := strings.TrimSpace(prefs.PhoneNumber); v != "" {
		phoneNumber = v
	}
	wechatID := t.WechatID
	if v := strings.TrimSpace(prefs.WechatID); v != "" {
		wechatID = v
	}

	return Settings{
		StoreIDs:           storeIDs,
		Adult:              adult,
		Child:              prefs.Child,
		TableType:          tableType,
		Debug:              true,
		PhoneNumber:        phoneNumber,
		WechatID:           wechatID,
		XAppCode:           t.XAppCode,
		QueryAuthorization: t.QueryAuth,
		ReservationAuth:    t.ReservationAuth,
		XAppClient:         defaultString(t.XAppClient, "miniapp"),
		UserAgent:          t.UserAgent,
		Referer:            t.Referer,
		StateFile:          stateFilePath(),
		Timezone:           timezone,
		Location:           location,
		PollInterval:       60 * time.Second,
		AvailableStatuses:  map[string]struct{}{"AVAILABLE": {}},
		BaseURL:            "https://crm-cn-prd.sushiro.com.cn",
		FeishuWebhook:      t.FeishuWebhook,
	}
}
