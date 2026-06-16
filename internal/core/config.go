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

// Settings 是运行期生效的全部配置，由 LoadSettings 从 config.json + 环境变量合成，
// 或由 CapturedTokens.ToSettings 从抓包得到的凭证生成。关键字段语义：
//   - QueryAuthorization / ReservationAuth 分别对应两个权限不同的 token：
//     查询类接口（getStoreById/timeslots）用前者；写操作（createReservation/createNetTicket
//     等 api_auth 接口）用后者。两者可能不同；老配置里只有一个 Authorization 时会被同时兜底成同一个。
//   - AvailableStatuses 决定哪些 Availability 取值的 Slot 视为「可约」，默认 {"AVAILABLE"}。
//   - Location 是 Timezone 解析后的 *time.Location，所有跨天/日期解析都以它为准。
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

// NumPersons 返回成人 + 儿童总人数；调用方（如 GetTimeslots 的 numpersons 参数）依赖它非零，
// 因此 Validate 会强制 Adult + Child > 0。
func (s Settings) NumPersons() int {
	return s.Adult + s.Child
}

// rawConfig 对应 config.json 的原始结构。与 Settings 的区别：这里用指针类型
// （Adult *int / Child *int / PollIntervalSeconds *float64）来区分「用户没填」和「用户填了 0」，
// 以便给前者套默认值而保留后者的显式 0（如 child=0 是合法的「只要成人」）。
// Authorization（裸字段）是为兼容老配置：仅当没单独配 query/reservation 时再回退到它。
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

// LoadSettings 从 configPath 读 config.json，叠加环境变量覆盖，再套默认值与校验。
// 覆盖优先级（高 → 低）：专用环境变量 > 通用 SUSHIRO_AUTHORIZATION（兼容老写法）> config.json。
// 注意 query/reservation 两套 token 都可能被同一个 SUSHIRO_AUTHORIZATION 兜底，但专用变量非空时不覆盖。
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
		// 专用变量优先；仅在它为空时才回退到通用 SUSHIRO_AUTHORIZATION（兼容老写法）。
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

	// 老配置兼容：config.json 里只有一个 authorization 字段时，把它同时用作查询和预约凭证。
	// 只在对应字段仍为空时回填，避免覆盖上面环境变量已经设置的专用值。
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

	// 默认值：成人 2 人（寿司门店常见最小桌），桌型 T（普通桌），客户端 miniapp（微信小程序），
	// 时区 Asia/Shanghai（寿司大陆门店）。这些值只在用户未填时生效。
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

	// 轮询间隔默认 60s；<=0 视为非法（防止忙轮询把接口打爆或卡死）。
	pollIntervalSeconds := 60.0
	if raw.PollIntervalSeconds != nil {
		pollIntervalSeconds = *raw.PollIntervalSeconds
	}
	if pollIntervalSeconds <= 0 {
		return Settings{}, fmt.Errorf("poll_interval_seconds must be greater than zero")
	}

	// 状态文件默认放在 config.json 同目录；相对路径以 config.json 所在目录为基准，再转绝对路径。
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

	// 生产环境 base URL；末尾斜杠会被 TrimRight 去掉，便于拼接路径。
	baseURL := strings.TrimSpace(raw.BaseURL)
	if baseURL == "" {
		baseURL = "https://crm-cn-prd.sushiro.com.cn"
	}

	// 可约状态集合：用户没配时默认只有 AVAILABLE；normalizeStatusSet 会统一转大写做匹配。
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

// normalizeStoreIDs 合并 store_ids 数组与老的单值 store_id（兼容旧配置），去空白、去重、保序。
// fallback 只在数组为空且 fallback 非空时启用。
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

// normalizeStatusSet 把状态字符串规整成大写集合，用于和 Slot.Availability 做大小写不敏感匹配。
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

// Validate 检查必填项是否齐全。注意 REPLACE_ 前缀被视为「未填」——这是模板里占位符的约定
// （如 REPLACE_ME），避免用户忘了改占位符就跑起来。
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

// EnsureBearer 保证 Authorization 头带 "Bearer " 前缀；已经带（大小写不敏感）则原样返回。
// 官方接口要求 Bearer 方案，缺前缀会 401。
func EnsureBearer(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}
