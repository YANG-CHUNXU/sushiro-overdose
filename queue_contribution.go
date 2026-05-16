package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	queueObservationFile        = "queue_observations.jsonl"
	queueSessionFile            = "queue_sessions.jsonl"
	queueStatsFile              = "queue_stats.json"
	contributionConfigFile      = "contribution.json"
	defaultCollectorURL         = "https://queue.sushiro-overdose.com/v1/submit"
	defaultContributionMinCount = 5
	queueContributionSchema     = 1
)

type QueueObservation struct {
	Timestamp       string `json:"ts"`
	StoreID         string `json:"store_id"`
	DisplayCalledNo int    `json:"display_called_no"`
	OnlineOpen      bool   `json:"online_open,omitempty"`
}

type QueueSession struct {
	StoreID                string `json:"store_id"`
	TakenAt                string `json:"taken_at"`
	TicketNo               int    `json:"ticket_no,omitempty"`
	DisplayCalledNoAtTake  int    `json:"display_called_no_at_take,omitempty"`
	CheckedInAt            string `json:"checked_in_at,omitempty"`
	CalledForUserAt        string `json:"called_for_user_at,omitempty"`
	CalledNoWhenUserCalled int    `json:"called_no_when_user_called,omitempty"`
	ActualWaitMinutes      int    `json:"actual_wait_minutes,omitempty"`
	PartySize              int    `json:"party_size,omitempty"`
	TableType              string `json:"table_type,omitempty"`
	ExpiredOrMissed        bool   `json:"expired_or_missed,omitempty"`
	Manual                 bool   `json:"manual,omitempty"`
}

type QueueContributionConfig struct {
	Enabled             bool   `json:"enabled"`
	CollectorURL        string `json:"collector_url"`
	AnonymousInstallID  string `json:"anonymous_install_id"`
	MinSamplesPerBucket int    `json:"min_samples_per_bucket"`
	LastUploadAt        string `json:"last_upload_at,omitempty"`
}

type QueueContributionPayload struct {
	SchemaVersion       int                     `json:"schema_version"`
	ClientVersion       string                  `json:"client_version"`
	GeneratedAt         string                  `json:"generated_at"`
	InstallIDHash       string                  `json:"install_id_hash"`
	MinSamplesPerBucket int                     `json:"min_samples_per_bucket"`
	Stats               []QueueContributionStat `json:"stats"`
}

type QueueContributionStat struct {
	StoreID                 string   `json:"store_id"`
	Weekday                 int      `json:"weekday"`
	TimeBucket              string   `json:"time_bucket"`
	TableType               string   `json:"table_type"`
	PartySizeBucket         string   `json:"party_size_bucket"`
	Samples                 int      `json:"samples"`
	WaitP50Minutes          *float64 `json:"wait_p50_minutes,omitempty"`
	WaitP80Minutes          *float64 `json:"wait_p80_minutes,omitempty"`
	CheckinToCallP50Minutes *float64 `json:"checkin_to_call_p50_minutes,omitempty"`
	MissedRate              float64  `json:"missed_rate"`
}

type QueueContributionPreview struct {
	Config  QueueContributionConfig  `json:"config"`
	Local   QueueLocalDataSummary    `json:"local"`
	Payload QueueContributionPayload `json:"payload"`
	Privacy QueuePrivacyPreview      `json:"privacy"`
	Ready   bool                     `json:"ready"`
}

type QueueLocalDataSummary struct {
	ObservationRecords int `json:"observation_records"`
	SessionRecords     int `json:"session_records"`
	UsableSessions     int `json:"usable_sessions"`
	AggregatedBuckets  int `json:"aggregated_buckets"`
}

type QueuePrivacyPreview struct {
	IncludedFields []string `json:"included_fields"`
	ExcludedFields []string `json:"excluded_fields"`
	Warnings       []string `json:"warnings,omitempty"`
}

type queueContributionBucket struct {
	storeID         string
	weekday         int
	timeBucket      string
	tableType       string
	partySizeBucket string
	waits           []float64
	checkinWaits    []float64
	missed          int
	total           int
}

func queueObservationPath() string {
	return filepath.Join(appDirPath(), queueObservationFile)
}

func queueSessionPath() string {
	return filepath.Join(appDirPath(), queueSessionFile)
}

func queueStatsPath() string {
	return filepath.Join(appDirPath(), queueStatsFile)
}

func contributionConfigPath() string {
	return filepath.Join(appDirPath(), contributionConfigFile)
}

func defaultQueueContributionConfig() QueueContributionConfig {
	return QueueContributionConfig{
		Enabled:             false,
		CollectorURL:        defaultCollectorURL,
		AnonymousInstallID:  newAnonymousInstallID(),
		MinSamplesPerBucket: defaultContributionMinCount,
	}
}

func LoadQueueContributionConfig() QueueContributionConfig {
	data, err := os.ReadFile(contributionConfigPath())
	if err != nil {
		return defaultQueueContributionConfig()
	}
	var cfg QueueContributionConfig
	if json.Unmarshal(data, &cfg) != nil {
		return defaultQueueContributionConfig()
	}
	return NormalizeQueueContributionConfig(cfg)
}

func SaveQueueContributionConfig(cfg QueueContributionConfig) error {
	cfg = NormalizeQueueContributionConfig(cfg)
	if err := os.MkdirAll(appDirPath(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(contributionConfigPath(), data, 0o600)
}

func NormalizeQueueContributionConfig(cfg QueueContributionConfig) QueueContributionConfig {
	cfg.CollectorURL = strings.TrimSpace(cfg.CollectorURL)
	if cfg.CollectorURL == "" {
		cfg.CollectorURL = defaultCollectorURL
	}
	cfg.AnonymousInstallID = strings.TrimSpace(cfg.AnonymousInstallID)
	if cfg.AnonymousInstallID == "" {
		cfg.AnonymousInstallID = newAnonymousInstallID()
	}
	if cfg.MinSamplesPerBucket <= 0 {
		cfg.MinSamplesPerBucket = defaultContributionMinCount
	}
	if cfg.MinSamplesPerBucket < 3 {
		cfg.MinSamplesPerBucket = 3
	}
	if cfg.MinSamplesPerBucket > 100 {
		cfg.MinSamplesPerBucket = 100
	}
	return cfg
}

func newAnonymousInstallID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func BuildQueueContributionPreview(now time.Time) QueueContributionPreview {
	cfg := LoadQueueContributionConfig()
	sessions := loadQueueSessions()
	observations := countJSONLLines(queueObservationPath())
	payload, usable := BuildQueueContributionPayload(cfg, sessions, now)
	privacy := QueuePrivacyPreview{
		IncludedFields: []string{
			"store_id", "weekday", "time_bucket", "table_type", "party_size_bucket",
			"samples", "wait_p50_minutes", "wait_p80_minutes", "checkin_to_call_p50_minutes", "missed_rate",
		},
		ExcludedFields: []string{
			"authorization", "phone_number", "wechat_id", "ticket_no", "display_called_no",
			"called_no_when_user_called", "taken_at", "checked_in_at", "called_for_user_at", "single_session_trace",
		},
	}
	if len(payload.Stats) == 0 {
		privacy.Warnings = append(privacy.Warnings, "当前没有达到样本门槛的聚合桶，不会上传任何排队统计")
	}
	return QueueContributionPreview{
		Config: cfg,
		Local: QueueLocalDataSummary{
			ObservationRecords: observations,
			SessionRecords:     len(sessions),
			UsableSessions:     usable,
			AggregatedBuckets:  len(payload.Stats),
		},
		Payload: payload,
		Privacy: privacy,
		Ready:   cfg.Enabled && len(payload.Stats) > 0,
	}
}

func BuildQueueContributionPayload(cfg QueueContributionConfig, sessions []QueueSession, now time.Time) (QueueContributionPayload, int) {
	cfg = NormalizeQueueContributionConfig(cfg)
	if now.IsZero() {
		now = time.Now()
	}
	buckets := map[string]*queueContributionBucket{}
	usable := 0
	for _, session := range sessions {
		b, ok := queueSessionBucket(session)
		if !ok {
			continue
		}
		usable++
		key := strings.Join([]string{b.storeID, fmt.Sprint(b.weekday), b.timeBucket, b.tableType, b.partySizeBucket}, "|")
		acc := buckets[key]
		if acc == nil {
			acc = &b
			buckets[key] = acc
		}
		acc.total++
		if session.ExpiredOrMissed {
			acc.missed++
		}
		if wait, ok := queueSessionWaitMinutes(session); ok {
			acc.waits = append(acc.waits, wait)
		}
		if wait, ok := queueSessionCheckinToCallMinutes(session); ok {
			acc.checkinWaits = append(acc.checkinWaits, wait)
		}
	}

	stats := make([]QueueContributionStat, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.total < cfg.MinSamplesPerBucket {
			continue
		}
		stat := QueueContributionStat{
			StoreID:         bucket.storeID,
			Weekday:         bucket.weekday,
			TimeBucket:      bucket.timeBucket,
			TableType:       bucket.tableType,
			PartySizeBucket: bucket.partySizeBucket,
			Samples:         bucket.total,
			MissedRate:      float64(bucket.missed) / float64(bucket.total),
		}
		stat.WaitP50Minutes = floatPtr(queueQuantile(bucket.waits, 0.50))
		stat.WaitP80Minutes = floatPtr(queueQuantile(bucket.waits, 0.80))
		stat.CheckinToCallP50Minutes = floatPtr(queueQuantile(bucket.checkinWaits, 0.50))
		stats = append(stats, stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		a, b := stats[i], stats[j]
		if a.StoreID != b.StoreID {
			return a.StoreID < b.StoreID
		}
		if a.Weekday != b.Weekday {
			return a.Weekday < b.Weekday
		}
		if a.TimeBucket != b.TimeBucket {
			return a.TimeBucket < b.TimeBucket
		}
		if a.TableType != b.TableType {
			return a.TableType < b.TableType
		}
		return a.PartySizeBucket < b.PartySizeBucket
	})
	return QueueContributionPayload{
		SchemaVersion:       queueContributionSchema,
		ClientVersion:       Version,
		GeneratedAt:         now.Format(time.RFC3339),
		InstallIDHash:       hashInstallID(cfg.AnonymousInstallID),
		MinSamplesPerBucket: cfg.MinSamplesPerBucket,
		Stats:               stats,
	}, usable
}

func UploadQueueContribution(ctx context.Context) (map[string]any, error) {
	cfg := LoadQueueContributionConfig()
	if !cfg.Enabled {
		return nil, fmt.Errorf("匿名贡献未开启")
	}
	payload, _ := BuildQueueContributionPayload(cfg, loadQueueSessions(), time.Now())
	if len(payload.Stats) == 0 {
		return nil, fmt.Errorf("没有达到样本门槛的聚合数据")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.CollectorURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "sushiro-overdose/"+Version)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg, ok := out["error"].(string); ok && msg != "" {
			return out, errors.New(msg)
		}
		return out, errors.New(resp.Status)
	}
	cfg.LastUploadAt = time.Now().Format(time.RFC3339)
	_ = SaveQueueContributionConfig(cfg)
	return out, nil
}

func loadQueueSessions() []QueueSession {
	f, err := os.Open(queueSessionPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := []QueueSession{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var session QueueSession
		if json.Unmarshal([]byte(line), &session) == nil {
			out = append(out, session)
		}
	}
	return out
}

func countJSONLLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

func queueSessionBucket(session QueueSession) (queueContributionBucket, bool) {
	storeID := strings.TrimSpace(session.StoreID)
	if storeID == "" {
		return queueContributionBucket{}, false
	}
	takenAt, err := time.Parse(time.RFC3339, strings.TrimSpace(session.TakenAt))
	if err != nil {
		return queueContributionBucket{}, false
	}
	if _, ok := queueSessionWaitMinutes(session); !ok && !session.ExpiredOrMissed {
		return queueContributionBucket{}, false
	}
	return queueContributionBucket{
		storeID:         storeID,
		weekday:         isoWeekday(takenAt.Weekday()),
		timeBucket:      halfHourBucket(takenAt),
		tableType:       normalizeContributionTableType(session.TableType),
		partySizeBucket: partySizeBucket(session.PartySize),
	}, true
}

func queueSessionWaitMinutes(session QueueSession) (float64, bool) {
	if session.ActualWaitMinutes > 0 {
		return float64(session.ActualWaitMinutes), true
	}
	takenAt, err := time.Parse(time.RFC3339, strings.TrimSpace(session.TakenAt))
	if err != nil {
		return 0, false
	}
	calledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(session.CalledForUserAt))
	if err != nil || calledAt.Before(takenAt) {
		return 0, false
	}
	return calledAt.Sub(takenAt).Minutes(), true
}

func queueSessionCheckinToCallMinutes(session QueueSession) (float64, bool) {
	checkedInAt, err := time.Parse(time.RFC3339, strings.TrimSpace(session.CheckedInAt))
	if err != nil {
		return 0, false
	}
	calledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(session.CalledForUserAt))
	if err != nil || calledAt.Before(checkedInAt) {
		return 0, false
	}
	return calledAt.Sub(checkedInAt).Minutes(), true
}

func halfHourBucket(t time.Time) string {
	minute := 0
	if t.Minute() >= 30 {
		minute = 30
	}
	return fmt.Sprintf("%02d:%02d", t.Hour(), minute)
}

func partySizeBucket(size int) string {
	switch {
	case size <= 0:
		return "unknown"
	case size <= 2:
		return "1-2"
	case size <= 4:
		return "3-4"
	default:
		return "5+"
	}
}

func normalizeContributionTableType(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	switch v {
	case "T", "TABLE":
		return "T"
	case "C", "COUNTER":
		return "C"
	default:
		return "unknown"
	}
}

func queueQuantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	out := append([]float64(nil), values...)
	sort.Float64s(out)
	if len(out) == 1 {
		return out[0]
	}
	pos := q * float64(len(out)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return out[lo]
	}
	weight := pos - float64(lo)
	return out[lo]*(1-weight) + out[hi]*weight
}

func floatPtr(v float64) *float64 {
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

func hashInstallID(id string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(id)))
	return hex.EncodeToString(sum[:])
}
