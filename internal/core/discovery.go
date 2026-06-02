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
	defaultDiscoveryLimit   = 500
	maxDiscoveryLimit       = 2000
)

var apiDiscoveryMu sync.Mutex

var discoveryErrorTextRedactors = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bwx[-_][a-z0-9._-]{3,}\b`),
	regexp.MustCompile(`(?i)\b[a-z0-9._-]{4,}[-_][a-z0-9._-]{3,}\b`),
	regexp.MustCompile(`\b[A-Za-z0-9+/=]{24,}\b`),
}

type APIDiscoveryConfig struct {
	Enabled    bool `json:"enabled"`
	MaxRecords int  `json:"max_records"`
}

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

func APIDiscoveryConfigPath() string {
	return filepath.Join(AppDirPath(), apiDiscoveryConfigFile)
}

func APIDiscoveryRecordsPath() string {
	return filepath.Join(AppDirPath(), apiDiscoveryRecordsFile)
}

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

func NormalizeAPIDiscoveryConfig(cfg APIDiscoveryConfig) APIDiscoveryConfig {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = defaultDiscoveryLimit
	}
	if cfg.MaxRecords > maxDiscoveryLimit {
		cfg.MaxRecords = maxDiscoveryLimit
	}
	return cfg
}

func APIDiscoveryEnabled() bool {
	return LoadAPIDiscoveryConfig().Enabled
}

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
		notes = append(notes, "api_auth 请求未看到 Authorization header；如果返回 401/403，优先重新登录/重新捕获认证。")
	}
	if text := strings.ToLower(strings.Join(discoveryResponseErrorValues(record.ResponseErrorFields), " ")); text != "" {
		for _, needle := range []string{"openid", "unionid", "session", "token", "login", "auth"} {
			if strings.Contains(text, needle) {
				notes = append(notes, "响应错误文本提到登录态/openid/session/token，优先判断 PC 微信登录态或认证参数。")
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
