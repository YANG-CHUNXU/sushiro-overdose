package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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

type APIDiscoveryConfig struct {
	Enabled    bool `json:"enabled"`
	MaxRecords int  `json:"max_records"`
}

type APIDiscoveryRecord struct {
	Timestamp                 string   `json:"timestamp"`
	Method                    string   `json:"method"`
	Host                      string   `json:"host"`
	Path                      string   `json:"path"`
	QueryKeys                 []string `json:"query_keys,omitempty"`
	Status                    int      `json:"status"`
	UpstreamProto             string   `json:"upstream_proto,omitempty"`
	RequestBodyKeys           []string `json:"request_body_keys,omitempty"`
	ResponseKind              string   `json:"response_kind,omitempty"`
	ResponseKeys              []string `json:"response_keys,omitempty"`
	ResponseArrayLen          int      `json:"response_array_len,omitempty"`
	ResponseArrayItemKeys     []string `json:"response_array_item_keys,omitempty"`
	ResponseDataKind          string   `json:"response_data_kind,omitempty"`
	ResponseDataKeys          []string `json:"response_data_keys,omitempty"`
	ResponseDataArrayLen      int      `json:"response_data_array_len,omitempty"`
	ResponseDataArrayItemKeys []string `json:"response_data_array_item_keys,omitempty"`
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

func BuildAPIDiscoveryRecord(method string, target *url.URL, status int, upstreamProto string, requestBodyKeys []string, responseBody []byte) APIDiscoveryRecord {
	record := APIDiscoveryRecord{
		Timestamp:       time.Now().Format(time.RFC3339),
		Method:          strings.ToUpper(strings.TrimSpace(method)),
		Status:          status,
		UpstreamProto:   strings.TrimSpace(upstreamProto),
		RequestBodyKeys: cleanDiscoveryKeys(requestBodyKeys),
	}
	if target != nil {
		record.Host = strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
		record.Path = sanitizeDiscoveryPath(target.EscapedPath())
		record.QueryKeys = queryKeyList(target.Query())
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

func sanitizeAPIDiscoveryRecord(record APIDiscoveryRecord) APIDiscoveryRecord {
	if record.Timestamp == "" {
		record.Timestamp = time.Now().Format(time.RFC3339)
	}
	record.Method = strings.ToUpper(strings.TrimSpace(record.Method))
	record.Host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(record.Host)), ".")
	record.Path = sanitizeDiscoveryPath(record.Path)
	record.UpstreamProto = strings.TrimSpace(record.UpstreamProto)
	record.QueryKeys = cleanDiscoveryKeys(record.QueryKeys)
	record.RequestBodyKeys = cleanDiscoveryKeys(record.RequestBodyKeys)
	record.ResponseKeys = cleanDiscoveryKeys(record.ResponseKeys)
	record.ResponseArrayItemKeys = cleanDiscoveryKeys(record.ResponseArrayItemKeys)
	record.ResponseDataKeys = cleanDiscoveryKeys(record.ResponseDataKeys)
	record.ResponseDataArrayItemKeys = cleanDiscoveryKeys(record.ResponseDataArrayItemKeys)
	return record
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
