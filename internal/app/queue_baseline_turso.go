package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	queueBaselineTursoURLEnv       = "SUSHIRO_BASELINE_TURSO_URL"
	queueBaselineTursoTokenEnv     = "SUSHIRO_BASELINE_TURSO_TOKEN"
	queueBaselineTursoFallbackURL  = "TURSO_DATABASE_URL"
	queueBaselineTursoFallbackAuth = "TURSO_AUTH_TOKEN"
	queueBaselineRemoteConfigFile  = "queue_baseline_remote.json"
	queueBaselineTursoTimeout      = 12 * time.Second
	queueBaselineRemoteCacheTTL    = 15 * time.Minute
)

type QueueBaselineRemoteConfig struct {
	DatabaseURL string `json:"database_url"`
	AuthToken   string `json:"auth_token"`
}

type queueBaselineTursoConfig struct {
	DatabaseURL string
	AuthToken   string
}

type queueBaselineRemoteCacheEntry struct {
	key      string
	loadedAt time.Time
	export   QueueBaselineExport
}

var queueBaselineRemoteCache struct {
	sync.Mutex
	entries map[string]queueBaselineRemoteCacheEntry
}

func queueBaselineRemoteConfigPath() string {
	return filepath.Join(AppDirPath(), queueBaselineRemoteConfigFile)
}

func LoadQueueBaselineRemoteConfig() QueueBaselineRemoteConfig {
	var cfg QueueBaselineRemoteConfig
	if data, err := os.ReadFile(queueBaselineRemoteConfigPath()); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if value := strings.TrimSpace(os.Getenv(queueBaselineTursoURLEnv)); value != "" {
		cfg.DatabaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv(queueBaselineTursoTokenEnv)); value != "" {
		cfg.AuthToken = value
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		cfg.DatabaseURL = strings.TrimSpace(os.Getenv(queueBaselineTursoFallbackURL))
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		cfg.AuthToken = strings.TrimSpace(os.Getenv(queueBaselineTursoFallbackAuth))
	}
	cfg.DatabaseURL = strings.TrimSpace(cfg.DatabaseURL)
	cfg.AuthToken = strings.TrimSpace(cfg.AuthToken)
	return cfg
}

func SaveQueueBaselineRemoteConfig(cfg QueueBaselineRemoteConfig) error {
	cfg.DatabaseURL = strings.TrimSpace(cfg.DatabaseURL)
	cfg.AuthToken = strings.TrimSpace(cfg.AuthToken)
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(queueBaselineRemoteConfigPath(), data, 0o600)
}

func loadQueueBaselineTursoConfig() queueBaselineTursoConfig {
	cfg := LoadQueueBaselineRemoteConfig()
	return queueBaselineTursoConfig{DatabaseURL: cfg.DatabaseURL, AuthToken: cfg.AuthToken}
}

func (c queueBaselineTursoConfig) configured() bool {
	return strings.TrimSpace(c.DatabaseURL) != "" && strings.TrimSpace(c.AuthToken) != ""
}

func (c queueBaselineTursoConfig) cacheKey() string {
	return strings.TrimSpace(c.DatabaseURL) + "\x00" + strings.TrimSpace(c.AuthToken)
}

func queueBaselineStoreCacheKey(baseKey, storeID string) string {
	return baseKey + "\x00store\x00" + strings.TrimSpace(storeID)
}

func getQueueBaselineRemoteCache(key string, now time.Time) (QueueBaselineExport, bool) {
	queueBaselineRemoteCache.Lock()
	defer queueBaselineRemoteCache.Unlock()
	if queueBaselineRemoteCache.entries == nil {
		return QueueBaselineExport{}, false
	}
	entry, ok := queueBaselineRemoteCache.entries[key]
	if !ok || entry.loadedAt.IsZero() || now.Sub(entry.loadedAt) >= queueBaselineRemoteCacheTTL {
		return QueueBaselineExport{}, false
	}
	return entry.export, true
}

func setQueueBaselineRemoteCache(key string, now time.Time, export QueueBaselineExport) {
	queueBaselineRemoteCache.Lock()
	defer queueBaselineRemoteCache.Unlock()
	if queueBaselineRemoteCache.entries == nil {
		queueBaselineRemoteCache.entries = map[string]queueBaselineRemoteCacheEntry{}
	}
	queueBaselineRemoteCache.entries[key] = queueBaselineRemoteCacheEntry{
		key:      key,
		loadedAt: now,
		export:   export,
	}
}

func queueBaselineRemoteStatusFromConfig(cfg queueBaselineTursoConfig) QueueBaselineRemoteStatus {
	return QueueBaselineRemoteStatus{
		Configured:  cfg.configured(),
		Provider:    "turso",
		DatabaseURL: redactQueueBaselineDatabaseURL(cfg.DatabaseURL),
		Message:     "使用本机 Turso 直连凭证；仅建议开发/私有环境使用。",
	}
}

func queueBaselineRemoteStatusFromCloud(cfg CloudAuthConfig) QueueBaselineRemoteStatus {
	return QueueBaselineRemoteStatus{
		Configured:    cfg.configured(),
		Provider:      "cloudflare",
		CloudURL:      cfg.BaseURL,
		Authenticated: cfg.connected(),
		UserLogin:     cfg.UserLogin,
		Message:       "使用 Cloudflare Worker 代理线上基准；Turso 凭证只在 Worker secrets 中。",
	}
}

func queueBaselineRemoteStatus() QueueBaselineRemoteStatus {
	if cfg := loadQueueBaselineTursoConfig(); cfg.configured() {
		return queueBaselineRemoteStatusFromConfig(cfg)
	}
	return queueBaselineRemoteStatusFromCloud(LoadCloudAuthConfig())
}

func loadRemoteQueueBaselineCached(ctx context.Context, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	cfg := loadQueueBaselineTursoConfig()
	if cfg.configured() {
		return loadRemoteQueueBaselineTursoCached(ctx, cfg, now)
	}
	cloudCfg := LoadCloudAuthConfig()
	return loadRemoteQueueBaselineCloudCached(ctx, cloudCfg, now)
}

func loadRemoteQueueBaselineForStores(ctx context.Context, storeIDs []string, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	storeIDs = UniqueNonEmptyStrings(storeIDs)
	if len(storeIDs) == 0 {
		return loadRemoteQueueBaselineCached(ctx, now)
	}
	if len(storeIDs) == 1 {
		return loadRemoteQueuePressureBaseline(ctx, storeIDs[0], now)
	}

	var exports []QueueBaselineExport
	var status QueueBaselineRemoteStatus
	for _, storeID := range storeIDs {
		export, currentStatus, err := loadRemoteQueuePressureBaseline(ctx, storeID, now)
		status = mergeQueueBaselineRemoteStatus(status, currentStatus)
		if err != nil {
			status.LastError = err.Error()
			return QueueBaselineExport{}, status, err
		}
		if currentStatus.Used {
			exports = append(exports, export)
		}
	}
	if len(exports) == 0 {
		return QueueBaselineExport{}, status, nil
	}
	merged := mergeQueueBaselineExports(exports, now)
	status.Used = true
	status.GeneratedAt = merged.GeneratedAt
	status.SourceUpdatedAt = merged.Stats.SourceUpdatedAt
	status.StoreCount = merged.Stats.StoreCount
	status.LatestCount = merged.Stats.LatestCount
	status.RollupCount = merged.Stats.RollupCount
	return merged, status, nil
}

func loadRemoteQueueBaselineTursoCached(ctx context.Context, cfg queueBaselineTursoConfig, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	status := queueBaselineRemoteStatusFromConfig(cfg)
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}

	key := cfg.cacheKey()
	if export, ok := getQueueBaselineRemoteCache(key, now); ok {
		status.Used = true
		status.GeneratedAt = export.GeneratedAt
		status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
		status.StoreCount = export.Stats.StoreCount
		status.LatestCount = export.Stats.LatestCount
		status.RollupCount = export.Stats.RollupCount
		return export, status, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, queueBaselineTursoTimeout)
	defer cancel()
	export, err := fetchQueueBaselineFromTurso(timeoutCtx, cfg, now)
	if err != nil {
		status.LastError = err.Error()
		return QueueBaselineExport{}, status, err
	}

	setQueueBaselineRemoteCache(key, now, export)

	status.Used = true
	status.GeneratedAt = export.GeneratedAt
	status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
	status.StoreCount = export.Stats.StoreCount
	status.LatestCount = export.Stats.LatestCount
	status.RollupCount = export.Stats.RollupCount
	return export, status, nil
}

func loadRemoteQueueBaselineCloudCached(ctx context.Context, cfg CloudAuthConfig, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	status := queueBaselineRemoteStatusFromCloud(cfg)
	if !cfg.configured() || !cfg.connected() {
		return QueueBaselineExport{}, status, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}

	key := cfg.cacheKey()
	if export, ok := getQueueBaselineRemoteCache(key, now); ok {
		status.Used = true
		status.GeneratedAt = export.GeneratedAt
		status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
		status.StoreCount = export.Stats.StoreCount
		status.LatestCount = export.Stats.LatestCount
		status.RollupCount = export.Stats.RollupCount
		return export, status, nil
	}

	export, err := fetchQueueBaselineFromCloud(ctx, cfg, "", now)
	if err != nil {
		status.LastError = err.Error()
		return QueueBaselineExport{}, status, err
	}

	setQueueBaselineRemoteCache(key, now, export)

	status.Used = true
	status.GeneratedAt = export.GeneratedAt
	status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
	status.StoreCount = export.Stats.StoreCount
	status.LatestCount = export.Stats.LatestCount
	status.RollupCount = export.Stats.RollupCount
	return export, status, nil
}

func loadRemoteQueuePressureBaseline(ctx context.Context, storeID string, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	cfg := loadQueueBaselineTursoConfig()
	if cfg.configured() {
		return loadRemoteQueuePressureBaselineTurso(ctx, cfg, storeID, now)
	}
	cloudCfg := LoadCloudAuthConfig()
	return loadRemoteQueuePressureBaselineCloud(ctx, cloudCfg, storeID, now)
}

func loadRemoteQueuePressureBaselineTurso(ctx context.Context, cfg queueBaselineTursoConfig, storeID string, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	status := queueBaselineRemoteStatusFromConfig(cfg)
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}

	key := queueBaselineStoreCacheKey(cfg.cacheKey(), strings.TrimSpace(storeID))
	if export, ok := getQueueBaselineRemoteCache(key, now); ok {
		status.Used = true
		status.GeneratedAt = export.GeneratedAt
		status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
		status.StoreCount = export.Stats.StoreCount
		status.LatestCount = export.Stats.LatestCount
		status.RollupCount = export.Stats.RollupCount
		return export, status, nil
	}

	storeInt, err := strconv.Atoi(strings.TrimSpace(storeID))
	if err != nil || storeInt <= 0 {
		status.LastError = "无效门店 ID"
		return QueueBaselineExport{}, status, fmt.Errorf("无效门店 ID: %s", storeID)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, queueBaselineTursoTimeout)
	defer cancel()
	export, err := fetchQueuePressureBaselineForStore(timeoutCtx, cfg, storeInt, now)
	if err != nil {
		status.LastError = err.Error()
		return QueueBaselineExport{}, status, err
	}

	setQueueBaselineRemoteCache(key, now, export)

	status.Used = true
	status.GeneratedAt = export.GeneratedAt
	status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
	status.StoreCount = export.Stats.StoreCount
	status.LatestCount = export.Stats.LatestCount
	status.RollupCount = export.Stats.RollupCount
	return export, status, nil
}

func loadRemoteQueuePressureBaselineCloud(ctx context.Context, cfg CloudAuthConfig, storeID string, now time.Time) (QueueBaselineExport, QueueBaselineRemoteStatus, error) {
	status := queueBaselineRemoteStatusFromCloud(cfg)
	if !cfg.configured() || !cfg.connected() {
		return QueueBaselineExport{}, status, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}

	key := queueBaselineStoreCacheKey(cfg.cacheKey(), strings.TrimSpace(storeID))
	if export, ok := getQueueBaselineRemoteCache(key, now); ok {
		status.Used = true
		status.GeneratedAt = export.GeneratedAt
		status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
		status.StoreCount = export.Stats.StoreCount
		status.LatestCount = export.Stats.LatestCount
		status.RollupCount = export.Stats.RollupCount
		return export, status, nil
	}

	storeID = strings.TrimSpace(storeID)
	if _, err := strconv.Atoi(storeID); err != nil || storeID == "" {
		status.LastError = "无效门店 ID"
		return QueueBaselineExport{}, status, fmt.Errorf("无效门店 ID: %s", storeID)
	}
	export, err := fetchQueueBaselineFromCloud(ctx, cfg, storeID, now)
	if err != nil {
		status.LastError = err.Error()
		return QueueBaselineExport{}, status, err
	}

	setQueueBaselineRemoteCache(key, now, export)

	status.Used = true
	status.GeneratedAt = export.GeneratedAt
	status.SourceUpdatedAt = export.Stats.SourceUpdatedAt
	status.StoreCount = export.Stats.StoreCount
	status.LatestCount = export.Stats.LatestCount
	status.RollupCount = export.Stats.RollupCount
	return export, status, nil
}

func fetchQueueBaselineFromCloud(ctx context.Context, cfg CloudAuthConfig, storeID string, now time.Time) (QueueBaselineExport, error) {
	var export QueueBaselineExport
	path := "/api/queue/baseline/export"
	query := neturl.Values{}
	if strings.TrimSpace(storeID) != "" {
		path = "/api/queue/baseline/store"
		query.Set("store_id", strings.TrimSpace(storeID))
	}
	if err := cloudGETJSON(ctx, cfg, path, query, &export); err != nil {
		return QueueBaselineExport{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if export.GeneratedAt == "" {
		export.GeneratedAt = now.Format(time.RFC3339)
	}
	if export.Source == "" {
		export.Source = "cloudflare"
	}
	if export.BucketMinutes <= 0 {
		export.BucketMinutes = queueDashboardDefaultBucketMins
	}
	if len(export.DateTypes) == 0 {
		export.DateTypes = []string{"weekday", "workday", "weekend", "holiday"}
	}
	return export, nil
}

func mergeQueueBaselineRemoteStatus(base, next QueueBaselineRemoteStatus) QueueBaselineRemoteStatus {
	if !base.Configured && next.Configured {
		base.Configured = true
	}
	if base.Provider == "" {
		base.Provider = next.Provider
	}
	if base.DatabaseURL == "" {
		base.DatabaseURL = next.DatabaseURL
	}
	if base.CloudURL == "" {
		base.CloudURL = next.CloudURL
	}
	if !base.Authenticated && next.Authenticated {
		base.Authenticated = true
	}
	if base.UserLogin == "" {
		base.UserLogin = next.UserLogin
	}
	if base.Message == "" {
		base.Message = next.Message
	}
	if next.LastError != "" {
		base.LastError = next.LastError
	}
	return base
}

func mergeQueueBaselineExports(exports []QueueBaselineExport, now time.Time) QueueBaselineExport {
	if now.IsZero() {
		now = time.Now()
	}
	merged := QueueBaselineExport{
		Version:       1,
		GeneratedAt:   now.Format(time.RFC3339),
		BucketMinutes: queueDashboardDefaultBucketMins,
		DateTypes:     []string{"weekday", "workday", "weekend", "holiday"},
	}
	storeSet := map[int]bool{}
	sourceUpdatedAt := ""
	for _, export := range exports {
		if merged.Source == "" {
			merged.Source = export.Source
		}
		if export.BucketMinutes > 0 {
			merged.BucketMinutes = export.BucketMinutes
		}
		if len(export.DateTypes) > 0 {
			merged.DateTypes = export.DateTypes
		}
		for _, store := range export.Stores {
			merged.Stores = append(merged.Stores, store)
			if store.StoreID > 0 {
				storeSet[store.StoreID] = true
			}
		}
		for _, latest := range export.Latest {
			merged.Latest = append(merged.Latest, latest)
			if latest.StoreID > 0 {
				storeSet[latest.StoreID] = true
			}
		}
		for _, rollup := range export.Rollups {
			merged.Rollups = append(merged.Rollups, rollup)
			if rollup.StoreID > 0 {
				storeSet[rollup.StoreID] = true
			}
		}
		if export.Stats.SourceUpdatedAt > sourceUpdatedAt {
			sourceUpdatedAt = export.Stats.SourceUpdatedAt
		}
	}
	if merged.Source == "" {
		merged.Source = "remote"
	}
	merged.Stats = QueueBaselineStats{
		StoreCount:      len(storeSet),
		LatestCount:     len(merged.Latest),
		RollupCount:     len(merged.Rollups),
		SourceUpdatedAt: sourceUpdatedAt,
	}
	return merged
}

func fetchQueuePressureBaselineForStore(ctx context.Context, cfg queueBaselineTursoConfig, storeID int, now time.Time) (QueueBaselineExport, error) {
	latest, latestUpdatedAt, err := fetchQueueBaselineLatestForStore(ctx, cfg, storeID)
	if err != nil {
		return QueueBaselineExport{}, err
	}
	rollups, sourceUpdatedAt, err := fetchQueueBaselineRollupsForStore(ctx, cfg, storeID)
	if err != nil {
		return QueueBaselineExport{}, err
	}
	if latestUpdatedAt > sourceUpdatedAt {
		sourceUpdatedAt = latestUpdatedAt
	}
	if now.IsZero() {
		now = time.Now()
	}
	storeCount := 0
	if len(latest) > 0 || len(rollups) > 0 {
		storeCount = 1
	}
	return QueueBaselineExport{
		Version:       1,
		GeneratedAt:   now.Format(time.RFC3339),
		Source:        "turso",
		BucketMinutes: queueDashboardDefaultBucketMins,
		DateTypes:     []string{"weekday", "workday", "weekend", "holiday"},
		Latest:        latest,
		Rollups:       rollups,
		Stats: QueueBaselineStats{
			StoreCount:      storeCount,
			LatestCount:     len(latest),
			RollupCount:     len(rollups),
			SourceUpdatedAt: sourceUpdatedAt,
		},
	}, nil
}

func fetchQueueBaselineFromTurso(ctx context.Context, cfg queueBaselineTursoConfig, now time.Time) (QueueBaselineExport, error) {
	stores, err := fetchQueueBaselineStores(ctx, cfg)
	if err != nil {
		return QueueBaselineExport{}, err
	}
	latest, latestUpdatedAt, err := fetchQueueBaselineLatest(ctx, cfg)
	if err != nil {
		return QueueBaselineExport{}, err
	}
	rollups, sourceUpdatedAt, err := fetchQueueBaselineRollups(ctx, cfg)
	if err != nil {
		return QueueBaselineExport{}, err
	}
	if latestUpdatedAt > sourceUpdatedAt {
		sourceUpdatedAt = latestUpdatedAt
	}
	if now.IsZero() {
		now = time.Now()
	}
	return QueueBaselineExport{
		Version:       1,
		GeneratedAt:   now.Format(time.RFC3339),
		Source:        "turso",
		BucketMinutes: queueDashboardDefaultBucketMins,
		DateTypes:     []string{"weekday", "workday", "weekend", "holiday"},
		Stores:        stores,
		Latest:        latest,
		Rollups:       rollups,
		Stats: QueueBaselineStats{
			StoreCount:      len(stores),
			LatestCount:     len(latest),
			RollupCount:     len(rollups),
			SourceUpdatedAt: sourceUpdatedAt,
		},
	}, nil
}

func fetchQueueBaselineStores(ctx context.Context, cfg queueBaselineTursoConfig) ([]QueueBaselineStore, error) {
	result, err := tursoQuery(ctx, cfg, `SELECT
		store_id, name, city, area, address, latitude, longitude, open_date,
		tables_capacity, counters_capacity, last_seen_at
	FROM store_dimension
	WHERE is_active = 1
	ORDER BY store_id`)
	if err != nil {
		return nil, fmt.Errorf("查询全国基准门店失败: %w", err)
	}
	out := make([]QueueBaselineStore, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 11 {
			continue
		}
		out = append(out, QueueBaselineStore{
			StoreID:          tursoInt(row[0]),
			Name:             tursoString(row[1]),
			City:             tursoString(row[2]),
			Area:             tursoString(row[3]),
			Address:          tursoString(row[4]),
			Latitude:         tursoFloatPtr(row[5]),
			Longitude:        tursoFloatPtr(row[6]),
			OpenDate:         tursoString(row[7]),
			TablesCapacity:   tursoInt(row[8]),
			CountersCapacity: tursoInt(row[9]),
			LastSeenAt:       tursoString(row[10]),
		})
	}
	return out, nil
}

func fetchQueueBaselineLatest(ctx context.Context, cfg queueBaselineTursoConfig) ([]QueueBaselineLatest, string, error) {
	latest, sourceUpdatedAt, err := fetchQueueBaselineLatestColumns(ctx, cfg, true)
	if err == nil {
		return latest, sourceUpdatedAt, nil
	}
	latest, sourceUpdatedAt, fallbackErr := fetchQueueBaselineLatestColumns(ctx, cfg, false)
	if fallbackErr == nil {
		return latest, sourceUpdatedAt, nil
	}
	return nil, "", fmt.Errorf("查询当前门店排队失败: %w", err)
}

func fetchQueueBaselineLatestColumns(ctx context.Context, cfg queueBaselineTursoConfig, includeCalled bool) ([]QueueBaselineLatest, string, error) {
	calledColumns := ""
	if includeCalled {
		calledColumns = `,
		display_called_no, group_queues_json`
	}
	result, err := tursoQuery(ctx, cfg, `SELECT
		store_id, collected_at, name, city, area, wait_minutes, group_queues_count,
		store_status, net_ticket_status, reservation_status, online_open,
		wait_time_counter, wait_time_cap`+calledColumns+`
	FROM store_latest
	ORDER BY store_id`)
	if err != nil {
		return nil, "", err
	}
	minColumns := 13
	if includeCalled {
		minColumns = 15
	}
	out := make([]QueueBaselineLatest, 0, len(result.Rows))
	sourceUpdatedAt := ""
	for _, row := range result.Rows {
		if len(row) < minColumns {
			continue
		}
		latest := QueueBaselineLatest{
			StoreID:           tursoInt(row[0]),
			CollectedAt:       tursoString(row[1]),
			Name:              tursoString(row[2]),
			City:              tursoString(row[3]),
			Area:              tursoString(row[4]),
			WaitMinutes:       tursoInt(row[5]),
			GroupQueuesCount:  tursoInt(row[6]),
			StoreStatus:       tursoString(row[7]),
			NetTicketStatus:   tursoString(row[8]),
			ReservationStatus: tursoString(row[9]),
			OnlineOpen:        tursoInt(row[10]) > 0,
			WaitTimeCounter:   tursoInt(row[11]),
			WaitTimeCap:       tursoInt(row[12]),
		}
		if includeCalled {
			latest.DisplayCalledNo = tursoInt(row[13])
			latest.GroupQueuesJSON = tursoString(row[14])
		}
		if latest.CollectedAt > sourceUpdatedAt {
			sourceUpdatedAt = latest.CollectedAt
		}
		out = append(out, latest)
	}
	return out, sourceUpdatedAt, nil
}

func fetchQueueBaselineLatestForStore(ctx context.Context, cfg queueBaselineTursoConfig, storeID int) ([]QueueBaselineLatest, string, error) {
	latest, sourceUpdatedAt, err := fetchQueueBaselineLatestForStoreColumns(ctx, cfg, storeID, true)
	if err == nil {
		return latest, sourceUpdatedAt, nil
	}
	latest, sourceUpdatedAt, fallbackErr := fetchQueueBaselineLatestForStoreColumns(ctx, cfg, storeID, false)
	if fallbackErr == nil {
		return latest, sourceUpdatedAt, nil
	}
	return nil, "", fmt.Errorf("查询门店当前排队失败: %w", err)
}

func fetchQueueBaselineLatestForStoreColumns(ctx context.Context, cfg queueBaselineTursoConfig, storeID int, includeCalled bool) ([]QueueBaselineLatest, string, error) {
	calledColumns := ""
	if includeCalled {
		calledColumns = `,
		display_called_no, group_queues_json`
	}
	result, err := tursoQuery(ctx, cfg, `SELECT
		store_id, collected_at, name, city, area, wait_minutes, group_queues_count,
		store_status, net_ticket_status, reservation_status, online_open,
		wait_time_counter, wait_time_cap`+calledColumns+`
	FROM store_latest
	WHERE store_id = `+strconv.Itoa(storeID)+`
	ORDER BY store_id`)
	if err != nil {
		return nil, "", err
	}
	minColumns := 13
	if includeCalled {
		minColumns = 15
	}
	out := make([]QueueBaselineLatest, 0, len(result.Rows))
	sourceUpdatedAt := ""
	for _, row := range result.Rows {
		if len(row) < minColumns {
			continue
		}
		latest := QueueBaselineLatest{
			StoreID:           tursoInt(row[0]),
			CollectedAt:       tursoString(row[1]),
			Name:              tursoString(row[2]),
			City:              tursoString(row[3]),
			Area:              tursoString(row[4]),
			WaitMinutes:       tursoInt(row[5]),
			GroupQueuesCount:  tursoInt(row[6]),
			StoreStatus:       tursoString(row[7]),
			NetTicketStatus:   tursoString(row[8]),
			ReservationStatus: tursoString(row[9]),
			OnlineOpen:        tursoInt(row[10]) > 0,
			WaitTimeCounter:   tursoInt(row[11]),
			WaitTimeCap:       tursoInt(row[12]),
		}
		if includeCalled {
			latest.DisplayCalledNo = tursoInt(row[13])
			latest.GroupQueuesJSON = tursoString(row[14])
		}
		if latest.CollectedAt > sourceUpdatedAt {
			sourceUpdatedAt = latest.CollectedAt
		}
		out = append(out, latest)
	}
	return out, sourceUpdatedAt, nil
}

func fetchQueueBaselineRollups(ctx context.Context, cfg queueBaselineTursoConfig) ([]QueueBaselineRollup, string, error) {
	rollups, sourceUpdatedAt, err := fetchQueueBaselineRollupsColumns(ctx, cfg, true)
	if err == nil {
		return rollups, sourceUpdatedAt, nil
	}
	rollups, sourceUpdatedAt, fallbackErr := fetchQueueBaselineRollupsColumns(ctx, cfg, false)
	if fallbackErr == nil {
		return rollups, sourceUpdatedAt, nil
	}
	return nil, "", fmt.Errorf("查询全国基准聚合失败: %w", err)
}

func fetchQueueBaselineRollupsColumns(ctx context.Context, cfg queueBaselineTursoConfig, includeCalled bool) ([]QueueBaselineRollup, string, error) {
	calledColumns := ""
	if includeCalled {
		calledColumns = `,
		called_sample_count, called_no_slow, called_no_typical, called_no_fast`
	}
	result, err := tursoQuery(ctx, cfg, `SELECT
		store_id, date_type, weekday, time_bucket, sample_count, open_rate,
		online_open_rate, busy_rate, wait_typical_minutes, wait_safe_minutes,
		wait_max_minutes, queue_groups_typical, queue_groups_safe`+calledColumns+`, confidence, updated_at
	FROM store_bucket_rollups
	ORDER BY store_id, date_type, weekday, time_bucket`)
	if err != nil {
		return nil, "", err
	}
	minColumns := 15
	confidenceIndex := 13
	updatedAtIndex := 14
	if includeCalled {
		minColumns = 19
		confidenceIndex = 17
		updatedAtIndex = 18
	}
	out := make([]QueueBaselineRollup, 0, len(result.Rows))
	sourceUpdatedAt := ""
	for _, row := range result.Rows {
		if len(row) < minColumns {
			continue
		}
		rollup := QueueBaselineRollup{
			StoreID:            tursoInt(row[0]),
			DateType:           tursoString(row[1]),
			Weekday:            tursoInt(row[2]),
			TimeBucket:         tursoString(row[3]),
			SampleCount:        tursoInt(row[4]),
			OpenRate:           tursoFloat(row[5]),
			OnlineOpenRate:     tursoFloat(row[6]),
			BusyRate:           tursoFloat(row[7]),
			WaitTypicalMinutes: tursoFloatPtr(row[8]),
			WaitSafeMinutes:    tursoFloatPtr(row[9]),
			WaitMaxMinutes:     tursoInt(row[10]),
			QueueGroupsTypical: tursoFloatPtr(row[11]),
			QueueGroupsSafe:    tursoFloatPtr(row[12]),
			Confidence:         tursoString(row[confidenceIndex]),
			UpdatedAt:          tursoString(row[updatedAtIndex]),
		}
		if includeCalled {
			rollup.CalledSampleCount = tursoInt(row[13])
			rollup.CalledNoSlow = tursoFloatPtr(row[14])
			rollup.CalledNoTypical = tursoFloatPtr(row[15])
			rollup.CalledNoFast = tursoFloatPtr(row[16])
		}
		if rollup.UpdatedAt > sourceUpdatedAt {
			sourceUpdatedAt = rollup.UpdatedAt
		}
		out = append(out, rollup)
	}
	return out, sourceUpdatedAt, nil
}

func fetchQueueBaselineRollupsForStore(ctx context.Context, cfg queueBaselineTursoConfig, storeID int) ([]QueueBaselineRollup, string, error) {
	rollups, sourceUpdatedAt, err := fetchQueueBaselineRollupsForStoreColumns(ctx, cfg, storeID, true)
	if err == nil {
		return rollups, sourceUpdatedAt, nil
	}
	rollups, sourceUpdatedAt, fallbackErr := fetchQueueBaselineRollupsForStoreColumns(ctx, cfg, storeID, false)
	if fallbackErr == nil {
		return rollups, sourceUpdatedAt, nil
	}
	return nil, "", fmt.Errorf("查询门店基准聚合失败: %w", err)
}

func fetchQueueBaselineRollupsForStoreColumns(ctx context.Context, cfg queueBaselineTursoConfig, storeID int, includeCalled bool) ([]QueueBaselineRollup, string, error) {
	calledColumns := ""
	if includeCalled {
		calledColumns = `,
		called_sample_count, called_no_slow, called_no_typical, called_no_fast`
	}
	result, err := tursoQuery(ctx, cfg, `SELECT
		store_id, date_type, weekday, time_bucket, sample_count, open_rate,
		online_open_rate, busy_rate, wait_typical_minutes, wait_safe_minutes,
		wait_max_minutes, queue_groups_typical, queue_groups_safe`+calledColumns+`, confidence, updated_at
	FROM store_bucket_rollups
	WHERE store_id = `+strconv.Itoa(storeID)+`
	ORDER BY store_id, date_type, weekday, time_bucket`)
	if err != nil {
		return nil, "", err
	}
	minColumns := 15
	confidenceIndex := 13
	updatedAtIndex := 14
	if includeCalled {
		minColumns = 19
		confidenceIndex = 17
		updatedAtIndex = 18
	}
	out := make([]QueueBaselineRollup, 0, len(result.Rows))
	sourceUpdatedAt := ""
	for _, row := range result.Rows {
		if len(row) < minColumns {
			continue
		}
		rollup := QueueBaselineRollup{
			StoreID:            tursoInt(row[0]),
			DateType:           tursoString(row[1]),
			Weekday:            tursoInt(row[2]),
			TimeBucket:         tursoString(row[3]),
			SampleCount:        tursoInt(row[4]),
			OpenRate:           tursoFloat(row[5]),
			OnlineOpenRate:     tursoFloat(row[6]),
			BusyRate:           tursoFloat(row[7]),
			WaitTypicalMinutes: tursoFloatPtr(row[8]),
			WaitSafeMinutes:    tursoFloatPtr(row[9]),
			WaitMaxMinutes:     tursoInt(row[10]),
			QueueGroupsTypical: tursoFloatPtr(row[11]),
			QueueGroupsSafe:    tursoFloatPtr(row[12]),
			Confidence:         tursoString(row[confidenceIndex]),
			UpdatedAt:          tursoString(row[updatedAtIndex]),
		}
		if includeCalled {
			rollup.CalledSampleCount = tursoInt(row[13])
			rollup.CalledNoSlow = tursoFloatPtr(row[14])
			rollup.CalledNoTypical = tursoFloatPtr(row[15])
			rollup.CalledNoFast = tursoFloatPtr(row[16])
		}
		if rollup.UpdatedAt > sourceUpdatedAt {
			sourceUpdatedAt = rollup.UpdatedAt
		}
		out = append(out, rollup)
	}
	return out, sourceUpdatedAt, nil
}

type tursoPipelineRequest struct {
	Requests []tursoStreamRequest `json:"requests"`
}

type tursoStreamRequest struct {
	Type string    `json:"type"`
	Stmt tursoStmt `json:"stmt"`
}

type tursoStmt struct {
	SQL      *string `json:"sql,omitempty"`
	WantRows bool    `json:"want_rows"`
}

type tursoPipelineResponse struct {
	Results []tursoStreamResult `json:"results"`
}

type tursoStreamResult struct {
	Type     string               `json:"type"`
	Response *tursoStreamResponse `json:"response,omitempty"`
	Error    *tursoError          `json:"error,omitempty"`
}

type tursoStreamResponse struct {
	Type   string              `json:"type"`
	Result tursoQueryResultRaw `json:"result,omitempty"`
}

type tursoQueryResultRaw struct {
	Cols []tursoColumn  `json:"cols"`
	Rows [][]tursoValue `json:"rows"`
}

type tursoColumn struct {
	Name *string `json:"name"`
	Type *string `json:"decltype"`
}

type tursoError struct {
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
}

type tursoValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

func tursoQuery(ctx context.Context, cfg queueBaselineTursoConfig, sql string) (tursoQueryResultRaw, error) {
	pipelineURL, host, err := tursoPipelineURL(cfg.DatabaseURL)
	if err != nil {
		return tursoQueryResultRaw{}, err
	}
	payload := tursoPipelineRequest{Requests: []tursoStreamRequest{{
		Type: "execute",
		Stmt: tursoStmt{SQL: &sql, WantRows: true},
	}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return tursoQueryResultRaw{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pipelineURL, bytes.NewReader(body))
	if err != nil {
		return tursoQueryResultRaw{}, err
	}
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.AuthToken))
	req.Header.Set("x-libsql-client-version", "sushiro-overdose-queue-baseline")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tursoQueryResultRaw{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tursoQueryResultRaw{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return tursoQueryResultRaw{}, fmt.Errorf("Turso HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded tursoPipelineResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return tursoQueryResultRaw{}, err
	}
	if len(decoded.Results) == 0 {
		return tursoQueryResultRaw{}, fmt.Errorf("Turso 返回为空")
	}
	result := decoded.Results[0]
	if result.Error != nil {
		if result.Error.Code != nil {
			return tursoQueryResultRaw{}, fmt.Errorf("%s: %s", *result.Error.Code, result.Error.Message)
		}
		return tursoQueryResultRaw{}, errors.New(result.Error.Message)
	}
	if result.Response == nil || result.Response.Type != "execute" {
		return tursoQueryResultRaw{}, fmt.Errorf("Turso 返回类型异常")
	}
	return result.Response.Result, nil
}

func tursoPipelineURL(rawURL string) (string, string, error) {
	parsed, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme == "libsql" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", "", fmt.Errorf("不支持的 Turso URL scheme: %s", parsed.Scheme)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	host := parsed.Host
	pipelineURL, err := neturl.JoinPath(parsed.String(), "/v2/pipeline")
	if err != nil {
		return "", "", err
	}
	return pipelineURL, host, nil
}

func tursoString(value tursoValue) string {
	if value.Type == "null" || len(value.Value) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(value.Value, &text); err == nil {
		return text
	}
	return strings.Trim(string(value.Value), `"`)
}

func tursoInt(value tursoValue) int {
	if value.Type == "null" || len(value.Value) == 0 {
		return 0
	}
	if value.Type == "integer" || value.Type == "text" {
		if n, err := strconv.Atoi(tursoString(value)); err == nil {
			return n
		}
	}
	f := tursoFloat(value)
	return int(f)
}

func tursoFloatPtr(value tursoValue) *float64 {
	if value.Type == "null" || len(value.Value) == 0 {
		return nil
	}
	f := tursoFloat(value)
	return &f
}

func tursoFloat(value tursoValue) float64 {
	if value.Type == "null" || len(value.Value) == 0 {
		return 0
	}
	if value.Type == "integer" || value.Type == "text" {
		if f, err := strconv.ParseFloat(tursoString(value), 64); err == nil {
			return f
		}
	}
	var f float64
	if err := json.Unmarshal(value.Value, &f); err == nil {
		return f
	}
	return 0
}

func redactQueueBaselineDatabaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return "(invalid url)"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
