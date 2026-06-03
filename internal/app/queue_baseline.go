package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	queueBaselineFile           = "queue_baseline.json"
	queueBaselineRecordsFile    = "queue_baseline.jsonl"
	queueBaselineDefaultMinutes = 5
	queueBaselineTickSeconds    = 30
)

// QueueBaselineRecord 是单家门店在某时刻的基准快照（全量记录，不做活跃过滤，
// 闲时/关店的 0 等位也是基准数据）。写入独立文件，不污染排队趋势观测。
type QueueBaselineRecord struct {
	Timestamp         string `json:"ts"`
	StoreID           int    `json:"store_id"`
	Name              string `json:"name"`
	City              string `json:"city"`
	Area              string `json:"area"`
	Wait              int    `json:"wait"`
	StoreStatus       string `json:"store_status"`
	NetTicketStatus   string `json:"net_ticket_status"`
	ReservationStatus string `json:"reservation_status,omitempty"`
	GroupQueuesCount  int    `json:"group_queues_count"`
	OnlineOpen        bool   `json:"online_open"`
}

// QueueBaselineConfig 控制「全国门店基准采集」：周期性快照全部门店的实时
// 等位/状态并落盘，作为基准数据。走公开接口，无需认证。
type QueueBaselineConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"interval_minutes"`
}

func queueBaselinePath() string { return filepath.Join(AppDirPath(), queueBaselineFile) }

func queueBaselineRecordsPath() string {
	return filepath.Join(AppDirPath(), queueBaselineRecordsFile)
}

func NormalizeQueueBaselineConfig(cfg QueueBaselineConfig) QueueBaselineConfig {
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = queueBaselineDefaultMinutes
	}
	if cfg.IntervalMinutes < 1 {
		cfg.IntervalMinutes = 1
	}
	if cfg.IntervalMinutes > 1440 {
		cfg.IntervalMinutes = 1440
	}
	return cfg
}

func LoadQueueBaselineConfig() QueueBaselineConfig {
	def := QueueBaselineConfig{Enabled: false, IntervalMinutes: queueBaselineDefaultMinutes}
	data, err := os.ReadFile(queueBaselinePath())
	if err != nil {
		return def
	}
	var cfg QueueBaselineConfig
	if json.Unmarshal(data, &cfg) != nil {
		return def
	}
	return NormalizeQueueBaselineConfig(cfg)
}

func SaveQueueBaselineConfig(cfg QueueBaselineConfig) error {
	cfg = NormalizeQueueBaselineConfig(cfg)
	os.MkdirAll(AppDirPath(), 0o755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(queueBaselinePath(), data, 0o600)
}

type QueueBaselineCollector struct {
	mu      sync.Mutex
	running bool
	lastAt  time.Time
}

var queueBaselineCollector = &QueueBaselineCollector{}

// Start 后台周期采集；按 base cadence 轮询，按配置间隔实际落盘，支持热改开关/间隔。
func (c *QueueBaselineCollector) Start(ctx context.Context) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go func() {
		t := time.NewTicker(queueBaselineTickSeconds * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.tick(ctx)
			}
		}
	}()
}

func (c *QueueBaselineCollector) tick(ctx context.Context) {
	cfg := LoadQueueBaselineConfig()
	if !cfg.Enabled {
		return
	}
	c.mu.Lock()
	due := c.lastAt.IsZero() || time.Since(c.lastAt) >= time.Duration(cfg.IntervalMinutes)*time.Minute
	c.mu.Unlock()
	if !due {
		return
	}
	if _, err := collectQueueBaseline(ctx); err != nil {
		return // 取数失败下个周期再试，不更新 lastAt
	}
	c.mu.Lock()
	c.lastAt = time.Now()
	c.mu.Unlock()
}

// collectQueueBaseline 拉全国门店快照并全量落盘为基准记录，返回写入条数。
func collectQueueBaseline(ctx context.Context) (int, error) {
	stores, err := NewQueueLiveClient().CachedAllStores(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().Format(time.RFC3339)
	records := make([]QueueBaselineRecord, 0, len(stores))
	for _, s := range stores {
		records = append(records, QueueBaselineRecord{
			Timestamp:         now,
			StoreID:           s.ID,
			Name:              s.Name,
			City:              s.NameKana,
			Area:              s.Area,
			Wait:              s.Wait,
			StoreStatus:       s.StoreStatus,
			NetTicketStatus:   s.NetTicketStatus,
			ReservationStatus: s.ReservationStatus,
			GroupQueuesCount:  s.GroupQueuesCount,
			OnlineOpen:        queueLiveStoreOnlineOpen(s),
		})
	}
	if err := appendQueueBaselineRecords(records); err != nil {
		return 0, err
	}
	return len(records), nil
}

// appendQueueBaselineRecords 一次性追加一批基准记录到 queue_baseline.jsonl。
func appendQueueBaselineRecords(records []QueueBaselineRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(queueBaselineRecordsPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = f.Chmod(0o600)
	enc := json.NewEncoder(f)
	for i := range records {
		if err := enc.Encode(records[i]); err != nil {
			return err
		}
	}
	return nil
}

func handleQueueBaseline(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, LoadQueueBaselineConfig())
	case http.MethodPost:
		var body QueueBaselineConfig
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := SaveQueueBaselineConfig(body); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, LoadQueueBaselineConfig())
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST only")
	}
}
