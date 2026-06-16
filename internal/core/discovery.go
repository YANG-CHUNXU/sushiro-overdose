package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiDiscoveryConfigFile  = "api_discovery.json"
	apiDiscoveryRecordsFile = "api_discovery.jsonl"
	// 默认/上限记录条数。discovery 会把每次抓到的请求元数据追加到 jsonl，超过上限时裁剪旧记录，
	// 防止长期运行后文件无限增长。
	defaultDiscoveryLimit = 500
	maxDiscoveryLimit     = 2000
)

// apiDiscoveryMu 保护对 api_discovery.jsonl 的并发读写（抓包器追加 + UI 读取/清理可能并发）。
var apiDiscoveryMu sync.Mutex

// discoveryErrorTextRedactors 用于对官方错误响应里的疑似敏感串做兜底脱敏：
//   - wx-xxx / wx_xxx：微信相关标识（openid/session 形态）。
//   - 形如 word-word 的复合 token：常见的各种 id/sessionkey。
//   - 24 位以上的 base64 串：长 token/jwt。
//
// 这是黑名单兜底，因为官方错误文案可能把敏感值拼进 message，仅靠 key 名无法覆盖。
var discoveryErrorTextRedactors = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bwx[-_][a-z0-9._-]{3,}\b`),
	regexp.MustCompile(`(?i)\b[a-z0-9._-]{4,}[-_][a-z0-9._-]{3,}\b`),
	regexp.MustCompile(`\b[A-Za-z0-9+/=]{24,}\b`),
}

// APIDiscoveryConfig 是 api_discovery.json 的结构。Enabled 默认 false——discovery 是调试辅助，
// 默认关闭以避免记录请求元数据带来的隐私/性能开销，需要时手动打开。
// MaxRecords 控制保留多少条历史记录。
type APIDiscoveryConfig struct {
	Enabled    bool `json:"enabled"`
	MaxRecords int  `json:"max_records"`
}

// APIDiscoveryRecord 是单次请求的「元数据快照」（不存响应体本身，只存结构摘要）。
// 设计取舍：只记 key 名 + 字段类型 + 长度等元数据，不记具体值——既能复盘接口结构、定位
// 缺哪个参数，又避免把凭证/PII 落盘。字段语义：
//   - QueryFields/RequestBodyFields：key → 类型标签（如 string/number/empty），不是真实值。
//   - Response* 系列：响应 JSON 的结构摘要（顶层 keys、数组长度、data 字段嵌套结构）。
//   - ResponseErrorFields：从响应里抽取的 error/message/code 等错误字段值（会脱敏）。
//   - Diagnosis：基于状态码和字段缺失自动生成的排查建议（中文）。
type APIDiscoveryRecord struct {
	Timestamp                 string            `json:"timestamp"`
	Method                    string            `json:"method"`
	Host                      string            `json:"host"`
	Path                      string            `json:"path"`
	QueryKeys                 []string          `json:"query_keys,omitempty"`
	QueryFields               map[string]string `json:"query_fields,omitempty"`
	Status                    int               `json:"status"`
	UpstreamProto             string            `json:"upstream_proto,omitempty"`
	RequestHeaderKeys         []string          `json:"request_header_keys,omitempty"`
	RequestBodyKeys           []string          `json:"request_body_keys,omitempty"`
	RequestBodyFields         map[string]string `json:"request_body_fields,omitempty"`
	ResponseKind              string            `json:"response_kind,omitempty"`
	ResponseKeys              []string          `json:"response_keys,omitempty"`
	ResponseArrayLen          int               `json:"response_array_len,omitempty"`
	ResponseArrayItemKeys     []string          `json:"response_array_item_keys,omitempty"`
	ResponseDataKind          string            `json:"response_data_kind,omitempty"`
	ResponseDataKeys          []string          `json:"response_data_keys,omitempty"`
	ResponseDataArrayLen      int               `json:"response_data_array_len,omitempty"`
	ResponseDataArrayItemKeys []string          `json:"response_data_array_item_keys,omitempty"`
	ResponseErrorFields       map[string]string `json:"response_error_fields,omitempty"`
	Diagnosis                 []string          `json:"diagnosis,omitempty"`
}

// APIDiscoveryJSONSummary 是 SummarizeAPIDiscoveryJSON 产出的中间结构，描述一段 JSON 的形状
// （顶层类型、keys、数组长度、data 子结构），用于填充 APIDiscoveryRecord 的 Response* 字段。
type APIDiscoveryJSONSummary struct {
	Kind              string
	Keys              []string
	ArrayLen          int
	ArrayItemKeys     []string
	DataKind          string
	DataKeys          []string
	DataArrayLen      int
	DataArrayItemKeys []string
}

// APIDiscoveryConfigPath 返回 discovery 配置文件路径（~/.sushiro/api_discovery.json）。
func APIDiscoveryConfigPath() string {
	return filepath.Join(AppDirPath(), apiDiscoveryConfigFile)
}

// APIDiscoveryRecordsPath 返回 discovery 记录文件路径（~/.sushiro/api_discovery.jsonl，每行一条 JSON）。
func APIDiscoveryRecordsPath() string {
	return filepath.Join(AppDirPath(), apiDiscoveryRecordsFile)
}

// LoadAPIDiscoveryConfig 读 discovery 配置；文件缺失或解析失败都返回「默认关闭」的安全配置，
// 保证开箱即用且默认不记录。
func LoadAPIDiscoveryConfig() APIDiscoveryConfig {
	cfg := APIDiscoveryConfig{MaxRecords: defaultDiscoveryLimit}
	data, err := os.ReadFile(APIDiscoveryConfigPath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return APIDiscoveryConfig{MaxRecords: defaultDiscoveryLimit}
	}
	return NormalizeAPIDiscoveryConfig(cfg)
}

// SaveAPIDiscoveryConfig 写 discovery 配置（0600，避免用户开关状态被改）。
func SaveAPIDiscoveryConfig(cfg APIDiscoveryConfig) error {
	cfg = NormalizeAPIDiscoveryConfig(cfg)
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(APIDiscoveryConfigPath(), data, 0o600)
}

// NormalizeAPIDiscoveryConfig 把 MaxRecords 钳制到 [default, max] 区间，<=0 给默认，>max 截断。
func NormalizeAPIDiscoveryConfig(cfg APIDiscoveryConfig) APIDiscoveryConfig {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = defaultDiscoveryLimit
	}
	if cfg.MaxRecords > maxDiscoveryLimit {
		cfg.MaxRecords = maxDiscoveryLimit
	}
	return cfg
}

// APIDiscoveryEnabled 读配置判断 discovery 是否开启。默认关——只有用户显式 enable 才记录请求元数据。
func APIDiscoveryEnabled() bool {
	return LoadAPIDiscoveryConfig().Enabled
}

// BuildAPIDiscoveryRecord 把一次请求的各项元数据组装成一条 APIDiscoveryRecord。
// 它只处理传入的「keys/类型」和响应体摘要，不接触原始凭证值——body 里只抽结构不存值。
// 调用方应在代理层拿到请求/响应后调用它，再用 RecordAPIDiscovery 落盘。
func BuildAPIDiscoveryRecord(method string, target *url.URL, status int, upstreamProto string, requestHeaderKeys, requestBodyKeys []string, requestBodyFields map[string]string, responseBody []byte) APIDiscoveryRecord {
	record := APIDiscoveryRecord{
		Timestamp:         time.Now().Format(time.RFC3339),
		Method:            strings.ToUpper(strings.TrimSpace(method)),
		Status:            status,
		UpstreamProto:     strings.TrimSpace(upstreamProto),
		RequestHeaderKeys: cleanDiscoveryKeys(requestHeaderKeys),
		RequestBodyKeys:   cleanDiscoveryKeys(requestBodyKeys),
		RequestBodyFields: cleanDiscoveryFieldKinds(requestBodyFields),
	}
	if target != nil {
		record.Host = strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
		record.Path = sanitizeDiscoveryPath(target.EscapedPath())
		record.QueryKeys = queryKeyList(target.Query())
		record.QueryFields = queryFieldKinds(target.Query())
	}
	summary := SummarizeAPIDiscoveryJSON(responseBody)
	record.ResponseKind = summary.Kind
	record.ResponseKeys = summary.Keys
	record.ResponseArrayLen = summary.ArrayLen
	record.ResponseArrayItemKeys = summary.ArrayItemKeys
	record.ResponseDataKind = summary.DataKind
	record.ResponseDataKeys = summary.DataKeys
	record.ResponseDataArrayLen = summary.DataArrayLen
	record.ResponseDataArrayItemKeys = summary.DataArrayItemKeys
	record.ResponseErrorFields = responseErrorFields(responseBody)
	record.Diagnosis = APIDiscoveryDiagnosis(record)
	return record
}

// RecordAPIDiscovery 把一条记录追加到 jsonl。Enabled=false 时直接静默返回（不记录、不报错）。
// 落盘前先 sanitize（去敏感、归一化），文件权限 0600。写完检查是否超过 MaxRecords，超了就裁旧。
// 全程持 apiDiscoveryMu，保证追加+裁剪之间的读操作看到一致状态。
func RecordAPIDiscovery(record APIDiscoveryRecord) error {
	cfg := LoadAPIDiscoveryConfig()
	if !cfg.Enabled {
		return nil
	}
	record = sanitizeAPIDiscoveryRecord(record)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	apiDiscoveryMu.Lock()
	defer apiDiscoveryMu.Unlock()
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	// O_APPEND 保证多协程/多进程追加时行与行不交错（单次 Write 对普通文件是原子的）。
	f, err := os.OpenFile(APIDiscoveryRecordsPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return trimAPIDiscoveryRecordsLocked(cfg.MaxRecords)
}

func LoadAPIDiscoveryRecords(limit int) ([]APIDiscoveryRecord, error) {
	if limit <= 0 {
		limit = defaultDiscoveryLimit
	}
	if limit > maxDiscoveryLimit {
		limit = maxDiscoveryLimit
	}
	apiDiscoveryMu.Lock()
	defer apiDiscoveryMu.Unlock()
	return loadAPIDiscoveryRecordsLocked(limit)
}

func ClearAPIDiscoveryRecords() error {
	apiDiscoveryMu.Lock()
	defer apiDiscoveryMu.Unlock()
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(APIDiscoveryRecordsPath(), nil, 0o600)
}

func APIDiscoveryRecordCount() int {
	apiDiscoveryMu.Lock()
	defer apiDiscoveryMu.Unlock()
	path := APIDiscoveryRecordsPath()
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

func APIDiscoveryPayloadKeys(body []byte) []string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err == nil {
		switch value := decoded.(type) {
		case map[string]any:
			return mapKeys(value)
		case []any:
			if len(value) > 0 {
				if item, ok := value[0].(map[string]any); ok {
					return mapKeys(item)
				}
			}
		}
	}
	if bytes.Contains(body, []byte("=")) {
		if values, err := url.ParseQuery(string(body)); err == nil && len(values) > 0 {
			return queryKeyList(values)
		}
	}
	return nil
}

func APIDiscoveryPayloadFieldKinds(body []byte) map[string]string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err == nil {
		if object, ok := decoded.(map[string]any); ok {
			fields := make(map[string]string, len(object))
			for key, value := range object {
				if key = strings.TrimSpace(key); key != "" {
					fields[key] = discoveryFieldKind(value)
				}
			}
			return cleanDiscoveryFieldKinds(fields)
		}
		return nil
	}
	if bytes.Contains(body, []byte("=")) {
		if values, err := url.ParseQuery(string(body)); err == nil && len(values) > 0 {
			return queryFieldKinds(values)
		}
	}
	return nil
}

func APIDiscoveryHeaderKeys(header http.Header) []string {
	keys := make([]string, 0, len(header))
	for key := range header {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	return cleanDiscoveryKeys(keys)
}

func queryFieldKinds(values url.Values) map[string]string {
	if len(values) == 0 {
		return nil
	}
	fields := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if len(value) == 0 {
			fields[key] = "missing"
			continue
		}
		fields[key] = discoveryStringKind(value[0])
	}
	return cleanDiscoveryFieldKinds(fields)
}

func cleanDiscoveryFieldKinds(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for key, kind := range fields {
		key = strings.TrimSpace(key)
		kind = strings.TrimSpace(kind)
		if key == "" || kind == "" {
			continue
		}
		if len(key) > 96 {
			key = key[:96]
		}
		out[key] = kind
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func discoveryFieldKind(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return discoveryStringKind(v)
	case bool:
		return "bool"
	case json.Number:
		return "number"
	case float64, float32, int, int64, int32:
		return "number"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "unknown"
	}
}

func discoveryStringKind(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "empty"
	}
	switch strings.ToLower(value) {
	case "null", "undefined", "nil":
		return "null_string"
	case "true", "false":
		return "bool_string"
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "number_string"
	}
	return "string"
}

// shouldRedactDiscoveryPathSegment 判断 URL 路径里的某一段是否像「带 ID/手机号」的敏感片段，
// 是的话在 sanitizeDiscoveryPathSegments 里替换成 {id}（如 /store/123456 → /store/{id}）。
// 规则：手机号、纯数字段、>=8 位且含数字的 token 形态段，都视为应脱敏。
func shouldRedactDiscoveryPathSegment(segment string) bool {
	segment = strings.TrimSpace(segment)
	if len(segment) < 4 {
		return false
	}
	if phoneRedactor.MatchString(segment) {
		return true
	}
	allDigits := true
	hasDigit := false
	tokenish := true
	for _, r := range segment {
		if r < '0' || r > '9' {
			allDigits = false
		} else {
			hasDigit = true
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			tokenish = false
		}
	}
	if allDigits {
		return true
	}
	return len(segment) >= 8 && hasDigit && tokenish
}

func redactDiscoveryPathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if shouldRedactDiscoveryPathSegment(part) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

func queryKeyList(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	return cleanDiscoveryKeys(keys)
}

func cleanDiscoveryKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if len(key) > 96 {
			key = key[:96]
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func responseErrorFields(body []byte) map[string]string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return nil
	}
	fields := map[string]string{}
	collectResponseErrorFields(fields, object, "")
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func collectResponseErrorFields(out map[string]string, object map[string]any, prefix string) {
	for key, value := range object {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		lower := strings.ToLower(key)
		if isDiscoveryErrorKey(lower) {
			if text := sanitizeDiscoveryErrorValue(value); text != "" {
				out[fullKey] = text
			}
			continue
		}
		if lower == "data" || lower == "result" || lower == "payload" {
			if nested, ok := value.(map[string]any); ok {
				collectResponseErrorFields(out, nested, fullKey)
			}
		}
	}
}

func isDiscoveryErrorKey(lower string) bool {
	switch lower {
	case "code", "error", "message", "msg", "status", "path", "reason", "title", "detail", "timestamp":
		return true
	default:
		return strings.Contains(lower, "error") || strings.Contains(lower, "message")
	}
}

func sanitizeDiscoveryErrorValue(value any) string {
	var text string
	switch v := value.(type) {
	case nil:
		text = "null"
	case string:
		text = v
	case json.Number:
		text = v.String()
	case bool:
		text = strconv.FormatBool(v)
	default:
		text = fmt.Sprintf("%v", v)
	}
	text = SanitizeDiagnosticLine(text)
	for _, re := range discoveryErrorTextRedactors {
		text = re.ReplaceAllString(text, "***")
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 256 {
		text = text[:256] + "..."
	}
	return text
}

// APIDiscoveryDiagnosis 基于一条记录的状态码、路径、字段缺失情况，生成中文排查建议。
// 它是「经验规则」集合：401/403→看登录态；422→看参数；init/store 类路径缺经纬度/城市码→提示补定位；
// api_auth 路径没带 Authorization→提示重新登录。返回的建议供 UI 展示，帮用户快速定位失败原因。
func APIDiscoveryDiagnosis(record APIDiscoveryRecord) []string {
	notes := []string{}
	status := record.Status
	path := strings.ToLower(record.Path)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		notes = append(notes, "疑似登录态或授权缺失/过期：重点看 Authorization、openid/session/token 是否存在。")
	} else if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity {
		notes = append(notes, "疑似请求参数缺失或格式异常：重点看 query/body 字段是否为空或 null。")
	} else if status >= 500 {
		notes = append(notes, "官方返回 5xx：若集中出现在 init/home/store/location/login 接口，通常仍要回看初始化参数和 PC 微信环境。")
	}
	if strings.Contains(path, "init") || strings.Contains(path, "home") || strings.Contains(path, "store") || strings.Contains(path, "location") || strings.Contains(path, "city") {
		if !discoveryRecordHasAnyField(record, "latitude", "lat") || !discoveryRecordHasAnyField(record, "longitude", "lng", "lon") {
			notes = append(notes, "请求里未看到完整经纬度字段；如果该接口依赖定位，PC 微信可能没有传完整定位参数。")
		}
		if !discoveryRecordHasAnyField(record, "citycode", "city_code", "adcode", "ad_code", "districtcode", "district_code") {
			notes = append(notes, "请求里未看到 cityCode/adCode/districtCode；如果接口按城市初始化，可能缺行政区编码。")
		}
	}
	if strings.Contains(path, "/api_auth/") && !discoveryRecordHasHeader(record, "authorization") {
		notes = append(notes, "api_auth 请求未看到 Authorization header；如果返回 401/403，优先重新登录/重新捕获凭证。")
	}
	if text := strings.ToLower(strings.Join(discoveryResponseErrorValues(record.ResponseErrorFields), " ")); text != "" {
		for _, needle := range []string{"openid", "unionid", "session", "token", "login", "auth"} {
			if strings.Contains(text, needle) {
				notes = append(notes, "响应错误文本提到登录态/openid/session/token，优先判断 PC 微信登录态或凭证参数。")
				break
			}
		}
		for _, needle := range []string{"latitude", "longitude", "location", "city", "adcode"} {
			if strings.Contains(text, needle) {
				notes = append(notes, "响应错误文本提到定位/城市字段，优先判断初始化定位参数。")
				break
			}
		}
	}
	return UniqueNonEmptyStrings(notes)
}

func discoveryRecordHasHeader(record APIDiscoveryRecord, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, got := range record.RequestHeaderKeys {
		if strings.ToLower(got) == key {
			return true
		}
	}
	return false
}

func discoveryRecordHasAnyField(record APIDiscoveryRecord, keys ...string) bool {
	want := map[string]bool{}
	for _, key := range keys {
		want[strings.ToLower(strings.TrimSpace(key))] = true
	}
	for key, kind := range record.QueryFields {
		if want[strings.ToLower(key)] && kind != "empty" && kind != "null" && kind != "null_string" && kind != "missing" {
			return true
		}
	}
	for key, kind := range record.RequestBodyFields {
		if want[strings.ToLower(key)] && kind != "empty" && kind != "null" && kind != "null_string" && kind != "missing" {
			return true
		}
	}
	return false
}

func discoveryResponseErrorValues(fields map[string]string) []string {
	out := make([]string, 0, len(fields))
	for _, value := range fields {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sanitizeAPIDiscoveryRecord(record APIDiscoveryRecord) APIDiscoveryRecord {
	if record.Timestamp == "" {
		record.Timestamp = time.Now().Format(time.RFC3339)
	}
	record.Method = strings.ToUpper(strings.TrimSpace(record.Method))
	record.Host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(record.Host)), ".")
	record.Path = sanitizeDiscoveryPath(record.Path)
	record.UpstreamProto = strings.TrimSpace(record.UpstreamProto)
	record.QueryKeys = cleanDiscoveryKeys(record.QueryKeys)
	record.QueryFields = cleanDiscoveryFieldKinds(record.QueryFields)
	record.RequestHeaderKeys = cleanDiscoveryKeys(record.RequestHeaderKeys)
	record.RequestBodyKeys = cleanDiscoveryKeys(record.RequestBodyKeys)
	record.RequestBodyFields = cleanDiscoveryFieldKinds(record.RequestBodyFields)
	record.ResponseKeys = cleanDiscoveryKeys(record.ResponseKeys)
	record.ResponseArrayItemKeys = cleanDiscoveryKeys(record.ResponseArrayItemKeys)
	record.ResponseDataKeys = cleanDiscoveryKeys(record.ResponseDataKeys)
	record.ResponseDataArrayItemKeys = cleanDiscoveryKeys(record.ResponseDataArrayItemKeys)
	record.ResponseErrorFields = sanitizeDiscoveryErrorFields(record.ResponseErrorFields)
	record.Diagnosis = UniqueNonEmptyStrings(record.Diagnosis)
	return record
}

func sanitizeDiscoveryErrorFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		key = strings.TrimSpace(key)
		value = sanitizeDiscoveryErrorValue(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeDiscoveryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if len(path) > 256 {
		path = path[:256]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return redactDiscoveryPathSegments(path)
}

// loadAPIDiscoveryRecordsLocked 读 jsonl 并返回最近 limit 条。必须在持 apiDiscoveryMu 时调用（*Locked 后缀）。
// 采用「边读边截断」的滑动窗口：一旦累计超过 limit 就 copy 出尾部 limit 条，避免大文件全量入内存。
func loadAPIDiscoveryRecordsLocked(limit int) ([]APIDiscoveryRecord, error) {
	f, err := os.Open(APIDiscoveryRecordsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	records := make([]APIDiscoveryRecord, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record APIDiscoveryRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		records = append(records, sanitizeAPIDiscoveryRecord(record))
		if len(records) > limit {
			copy(records, records[len(records)-limit:])
			records = records[:limit]
		}
	}
	if err := scanner.Err(); err != nil {
		return records, err
	}
	return records, nil
}

// trimAPIDiscoveryRecordsLocked 在记录数达到 limit 时裁掉最旧的，把文件重写为最近 limit 条。
// 必须持锁调用。len(records) < limit 时不做任何 IO，直接返回。
func trimAPIDiscoveryRecordsLocked(limit int) error {
	if limit <= 0 {
		limit = defaultDiscoveryLimit
	}
	records, err := loadAPIDiscoveryRecordsLocked(limit)
	if err != nil || len(records) < limit {
		return err
	}
	data := bytes.Buffer{}
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			continue
		}
		data.Write(line)
		data.WriteByte('\n')
	}
	return os.WriteFile(APIDiscoveryRecordsPath(), data.Bytes(), 0o600)
}

func SummarizeAPIDiscoveryJSON(body []byte) APIDiscoveryJSONSummary {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return APIDiscoveryJSONSummary{Kind: "empty"}
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return APIDiscoveryJSONSummary{Kind: "non_json"}
	}
	summary := APIDiscoveryJSONSummary{Kind: discoveryValueKind(decoded)}
	switch value := decoded.(type) {
	case map[string]any:
		summary.Keys = mapKeys(value)
		if data, ok := value["data"]; ok {
			fillDiscoveryDataSummary(&summary, data)
		}
	case []any:
		summary.ArrayLen = len(value)
		summary.ArrayItemKeys = firstObjectKeys(value)
	}
	return summary
}

func fillDiscoveryDataSummary(summary *APIDiscoveryJSONSummary, data any) {
	summary.DataKind = discoveryValueKind(data)
	switch value := data.(type) {
	case map[string]any:
		summary.DataKeys = mapKeys(value)
	case []any:
		summary.DataArrayLen = len(value)
		summary.DataArrayItemKeys = firstObjectKeys(value)
	}
}

func discoveryValueKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	case json.Number, float64, float32, int, int64, int32:
		return "number"
	default:
		return "unknown"
	}
}

func firstObjectKeys(values []any) []string {
	for _, item := range values {
		if object, ok := item.(map[string]any); ok {
			return mapKeys(object)
		}
	}
	return nil
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
