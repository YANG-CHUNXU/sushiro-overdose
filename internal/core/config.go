package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Settings struct {
	StoreIDs           []string
	Adult              int
	Child              int
	TableType          string
	Debug              bool
	PhoneNumber        string
	WechatID           string
	XAppCode           string
	QueryAuthorization string
	ReservationAuth    string
	XAppClient         string
	UserAgent          string
	Referer            string
	FeishuWebhook      string
	StateFile          string
	Timezone           string
	Location           *time.Location
	PollInterval       time.Duration
	AvailableStatuses  map[string]struct{}
	BaseURL            string
}

func (s Settings) NumPersons() int {
	return s.Adult + s.Child
}

type rawConfig struct {
	StoreIDs            []string `json:"store_ids"`
	StoreID             string   `json:"store_id"`
	Adult               *int     `json:"adult"`
	Child               *int     `json:"child"`
	TableType           string   `json:"table_type"`
	Debug               bool     `json:"debug"`
	PhoneNumber         string   `json:"phone_number"`
	WechatID            string   `json:"wechat_id"`
	XAppCode            string   `json:"x_app_code"`
	QueryAuthorization  string   `json:"query_authorization"`
	ReservationAuth     string   `json:"reservation_authorization"`
	Authorization       string   `json:"authorization"`
	XAppClient          string   `json:"x_app_client"`
	UserAgent           string   `json:"user_agent"`
	Referer             string   `json:"referer"`
	FeishuWebhook       string   `json:"feishu_webhook"`
	StateFile           string   `json:"state_file"`
	Timezone            string   `json:"timezone"`
	PollIntervalSeconds *float64 `json:"poll_interval_seconds"`
	AvailableStatuses   []string `json:"available_statuses"`
	BaseURL             string   `json:"base_url"`
}

func LoadSettings(configPath string) (Settings, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Settings{}, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Settings{}, fmt.Errorf("invalid JSON in config file %s: %w", configPath, err)
	}

	if value := os.Getenv("SUSHIRO_PHONE_NUMBER"); value != "" {
		raw.PhoneNumber = value
	}
	if value := os.Getenv("SUSHIRO_WECHAT_ID"); value != "" {
		raw.WechatID = value
	}
	if value := os.Getenv("SUSHIRO_X_APP_CODE"); value != "" {
		raw.XAppCode = value
	}
	if value := os.Getenv("SUSHIRO_QUERY_AUTHORIZATION"); value != "" {
		raw.QueryAuthorization = value
	} else if value := os.Getenv("SUSHIRO_AUTHORIZATION"); value != "" && raw.QueryAuthorization == "" {
		raw.QueryAuthorization = value
	}
	if value := os.Getenv("SUSHIRO_RESERVATION_AUTHORIZATION"); value != "" {
		raw.ReservationAuth = value
	} else if value := os.Getenv("SUSHIRO_AUTHORIZATION"); value != "" && raw.ReservationAuth == "" {
		raw.ReservationAuth = value
	}
	if value := os.Getenv("SUSHIRO_FEISHU_WEBHOOK"); value != "" {
		raw.FeishuWebhook = value
	}

	if raw.Authorization != "" {
		if raw.QueryAuthorization == "" {
			raw.QueryAuthorization = raw.Authorization
		}
		if raw.ReservationAuth == "" {
			raw.ReservationAuth = raw.Authorization
		}
	}

	storeIDs := normalizeStoreIDs(raw.StoreIDs, raw.StoreID)
	if len(storeIDs) == 0 {
		return Settings{}, fmt.Errorf("at least one store ID must be configured in store_ids")
	}

	adult := 2
	if raw.Adult != nil {
		adult = *raw.Adult
	}
	child := 0
	if raw.Child != nil {
		child = *raw.Child
	}
	tableType := strings.TrimSpace(raw.TableType)
	if tableType == "" {
		tableType = "T"
	}
	xAppClient := strings.TrimSpace(raw.XAppClient)
	if xAppClient == "" {
		xAppClient = "miniapp"
	}
	timezone := strings.TrimSpace(raw.Timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Settings{}, fmt.Errorf("load timezone %s: %w", timezone, err)
	}

	pollIntervalSeconds := 60.0
	if raw.PollIntervalSeconds != nil {
		pollIntervalSeconds = *raw.PollIntervalSeconds
	}
	if pollIntervalSeconds <= 0 {
		return Settings{}, fmt.Errorf("poll_interval_seconds must be greater than zero")
	}

	stateFile := strings.TrimSpace(raw.StateFile)
	if stateFile == "" {
		stateFile = ".sushiro_state.json"
	}
	if !filepath.IsAbs(stateFile) {
		stateFile = filepath.Join(filepath.Dir(configPath), stateFile)
	}
	stateFile, err = filepath.Abs(stateFile)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve state file: %w", err)
	}

	baseURL := strings.TrimSpace(raw.BaseURL)
	if baseURL == "" {
		baseURL = "https://crm-cn-prd.sushiro.com.cn"
	}

	availableStatuses := normalizeStatusSet(raw.AvailableStatuses)
	if len(availableStatuses) == 0 {
		availableStatuses["AVAILABLE"] = struct{}{}
	}

	settings := Settings{
		StoreIDs:           storeIDs,
		Adult:              adult,
		Child:              child,
		TableType:          tableType,
		Debug:              raw.Debug,
		PhoneNumber:        strings.TrimSpace(raw.PhoneNumber),
		WechatID:           strings.TrimSpace(raw.WechatID),
		XAppCode:           strings.TrimSpace(raw.XAppCode),
		QueryAuthorization: strings.TrimSpace(raw.QueryAuthorization),
		ReservationAuth:    strings.TrimSpace(raw.ReservationAuth),
		XAppClient:         xAppClient,
		UserAgent:          EffectiveUserAgent(raw.UserAgent),
		Referer:            strings.TrimSpace(raw.Referer),
		FeishuWebhook:      strings.TrimSpace(raw.FeishuWebhook),
		StateFile:          stateFile,
		Timezone:           timezone,
		Location:           location,
		PollInterval:       time.Duration(pollIntervalSeconds * float64(time.Second)),
		AvailableStatuses:  availableStatuses,
		BaseURL:            strings.TrimRight(baseURL, "/"),
	}

	if err := settings.Validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func normalizeStoreIDs(values []string, fallback string) []string {
	if len(values) == 0 && strings.TrimSpace(fallback) != "" {
		values = []string{fallback}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeStatusSet(values []string) map[string]struct{} {
	statuses := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		statuses[normalized] = struct{}{}
	}
	return statuses
}

func (s Settings) Validate() error {
	required := map[string]string{
		"phone_number":              s.PhoneNumber,
		"wechat_id":                 s.WechatID,
		"x_app_code":                s.XAppCode,
		"query_authorization":       s.QueryAuthorization,
		"reservation_authorization": s.ReservationAuth,
		"user_agent":                s.UserAgent,
		"referer":                   s.Referer,
	}
	missing := make([]string, 0)
	for key, value := range required {
		if value == "" || strings.HasPrefix(value, "REPLACE_") {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("missing required config values: %s", strings.Join(missing, ", "))
	}
	if len(s.StoreIDs) == 0 {
		return fmt.Errorf("at least one store ID must be configured in store_ids")
	}
	if s.Adult < 0 || s.Child < 0 {
		return fmt.Errorf("adult and child must be zero or greater")
	}
	if s.NumPersons() <= 0 {
		return fmt.Errorf("adult + child must be greater than zero")
	}
	if s.PollInterval <= 0 {
		return fmt.Errorf("poll_interval_seconds must be greater than zero")
	}
	return nil
}

func EnsureBearer(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}
