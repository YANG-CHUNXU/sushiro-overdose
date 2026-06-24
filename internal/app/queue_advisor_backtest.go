package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// queue_advisor_backtest.go —— 预测精度的「自证回测」闭环。
//
// 思路：每次给出「几点叫到」的预测就记一条开放预测（首次为准，不被后续更近的预测覆盖——
// 越接近叫到预测越准，用首次预测才能诚实衡量「我们当初说的有多靠谱」）。当后续观测显示
// 该号已被叫到，就把开放预测「结算」成一条带误差的样本，落 eta_backtest.jsonl。
//
// 用途：
//   1. 按店聚合实测误差（MAE / 系统性偏差）→ 设置页「预测准确度」面板，让用户看到可信度。
//   2. 反哺区间：偏差大的店自动放宽区间、修正系统性偏早/偏晚（buildQueueAdvisorEta 调用）。
//
// 只统计「号确实被叫到」的客观时刻与当初预测之差，不掺用户是否按建议出发——避免行为污染。

const (
	etaOpenFile     = "eta_open.json"
	etaBacktestFile = "eta_backtest.jsonl"

	// 开放预测的最大存活时间：超过则视为「号当天没被叫到 / 用户输错号」放弃，不结算。
	etaOpenMaxAge = 8 * time.Hour
	// 结算样本回测文件行数上限，超出滚动裁剪。
	etaBacktestMaxLines = 4000
	// 聚合时每店最多看最近这么多条样本（更早的节奏可能已不具代表性）。
	etaBacktestMaxPerStore = 40
	// 少于这么多样本不做校准（统计不可信）。
	etaCalibMinSamples = 4
)

// etaOpenRecord 一条尚未结算的预测。key = storeID|targetNo。
type etaOpenRecord struct {
	StoreID           string    `json:"store_id"`
	TargetNo          int       `json:"target_no"`
	PredictedAt       time.Time `json:"predicted_at"`        // 何时做的这条预测
	PredictedCalledAt time.Time `json:"predicted_called_at"` // 预测几点叫到（区间中点）
	CalledNoAtPred    int       `json:"called_no_at_pred"`   // 预测时已叫到的号
}

// etaBacktestSample 一条已结算样本。
type etaBacktestSample struct {
	TS       string  `json:"ts"` // 实际叫到时刻（近似为观测到 called>=target 的时刻）
	StoreID  string  `json:"store_id"`
	TargetNo int     `json:"target_no"`
	ErrorMin float64 `json:"error_min"` // 实际 - 预测（分钟）；正=偏晚(低估等待)，负=偏早
	LeadMin  float64 `json:"lead_min"`  // 预测时距叫到还有多久（预测的等待时长，衡量是否早期预测）
}

var etaBacktestMu sync.Mutex // 串行化 open/backtest 两个文件的读改写

func etaOpenPath() string     { return filepath.Join(AppDirPath(), etaOpenFile) }
func etaBacktestPath() string { return filepath.Join(AppDirPath(), etaBacktestFile) }

func loadEtaOpenLocked() map[string]etaOpenRecord {
	out := map[string]etaOpenRecord{}
	raw, err := os.ReadFile(etaOpenPath())
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func saveEtaOpenLocked(m map[string]etaOpenRecord) {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(AppDirPath(), 0o755)
	_ = AtomicWriteFile(etaOpenPath(), raw, 0o600)
}

func etaOpenKey(storeID string, targetNo int) string {
	return fmt.Sprintf("%s|%d", storeID, targetNo)
}

// recordEtaPrediction：登记一条开放预测。同 key 已存在则保留首次（不覆盖），并顺手清理过期项。
// predictedCalledAt 为零值（无可靠预测）时直接忽略。
func recordEtaPrediction(storeID string, targetNo, calledNoNow int, predictedCalledAt, now time.Time) {
	if storeID == "" || targetNo <= 0 || predictedCalledAt.IsZero() {
		return
	}
	// 目标号不大于当前叫号 = 已轮到/过号，没有等待可衡量，不记。
	if calledNoNow > 0 && targetNo <= calledNoNow {
		return
	}
	etaBacktestMu.Lock()
	defer etaBacktestMu.Unlock()
	m := loadEtaOpenLocked()
	pruneEtaOpenLocked(m, now)
	key := etaOpenKey(storeID, targetNo)
	if _, exists := m[key]; exists {
		return // 保留首次预测
	}
	m[key] = etaOpenRecord{
		StoreID:           storeID,
		TargetNo:          targetNo,
		PredictedAt:       now,
		PredictedCalledAt: predictedCalledAt,
		CalledNoAtPred:    calledNoNow,
	}
	saveEtaOpenLocked(m)
}

// pruneEtaOpenLocked 删除过期开放预测（caller 持锁）。返回是否有删除。
func pruneEtaOpenLocked(m map[string]etaOpenRecord, now time.Time) bool {
	changed := false
	for k, rec := range m {
		if now.Sub(rec.PredictedAt) > etaOpenMaxAge {
			delete(m, k)
			changed = true
		}
	}
	return changed
}

// backfillEtaOnObservation：观测显示叫号推进到 calledNo 时，结算该店所有 targetNo<=calledNo
// 的开放预测，写入回测样本。由 appendQueueObservation 在新观测落盘后调用。
func backfillEtaOnObservation(storeID string, calledNo int, now time.Time) {
	if storeID == "" || calledNo <= 0 {
		return
	}
	etaBacktestMu.Lock()
	defer etaBacktestMu.Unlock()
	m := loadEtaOpenLocked()
	changed := pruneEtaOpenLocked(m, now)
	var settled []etaBacktestSample
	for k, rec := range m {
		if rec.StoreID != storeID || rec.TargetNo > calledNo {
			continue
		}
		errMin := now.Sub(rec.PredictedCalledAt).Minutes()
		leadMin := rec.PredictedCalledAt.Sub(rec.PredictedAt).Minutes()
		settled = append(settled, etaBacktestSample{
			TS:       now.Format(time.RFC3339),
			StoreID:  storeID,
			TargetNo: rec.TargetNo,
			ErrorMin: round1(errMin),
			LeadMin:  round1(leadMin),
		})
		delete(m, k)
		changed = true
	}
	if changed {
		saveEtaOpenLocked(m)
	}
	for _, s := range settled {
		appendEtaBacktestSampleLocked(s)
	}
}

func appendEtaBacktestSampleLocked(s etaBacktestSample) {
	os.MkdirAll(AppDirPath(), 0o755)
	f, err := os.OpenFile(etaBacktestPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		f.Close()
		return
	}
	_, _ = f.Write(append(data, '\n'))
	f.Close()
	trimEtaBacktestLocked()
}

// trimEtaBacktestLocked 行数超限时保留最近 etaBacktestMaxLines 行。
func trimEtaBacktestLocked() {
	lines, err := readEtaBacktestLines()
	if err != nil || len(lines) <= etaBacktestMaxLines {
		return
	}
	keep := lines[len(lines)-etaBacktestMaxLines:]
	buf := make([]byte, 0, len(keep)*64)
	for _, ln := range keep {
		buf = append(buf, ln...)
		buf = append(buf, '\n')
	}
	_ = AtomicWriteFile(etaBacktestPath(), buf, 0o600)
}

func readEtaBacktestLines() ([]string, error) {
	f, err := os.Open(etaBacktestPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		t := sc.Text()
		if t != "" {
			lines = append(lines, t)
		}
	}
	return lines, sc.Err()
}

func loadEtaBacktestSamples() []etaBacktestSample {
	lines, err := readEtaBacktestLines()
	if err != nil {
		return nil
	}
	out := make([]etaBacktestSample, 0, len(lines))
	for _, ln := range lines {
		var s etaBacktestSample
		if json.Unmarshal([]byte(ln), &s) == nil && s.StoreID != "" {
			out = append(out, s)
		}
	}
	return out
}

// EtaAccuracyStore 单店实测精度。
type EtaAccuracyStore struct {
	StoreID  string  `json:"store_id"`
	Samples  int     `json:"samples"`
	MAEMin   float64 `json:"mae_min"`   // 平均绝对误差
	BiasMin  float64 `json:"bias_min"`  // 中位有符号误差：>0 系统性偏晚(低估等待)，<0 偏早
	WorstMin float64 `json:"worst_min"` // 最大绝对误差
}

// EtaAccuracyReport 暴露给前端的整体精度。
type EtaAccuracyReport struct {
	GeneratedAt  string             `json:"generated_at"`
	TotalSamples int                `json:"total_samples"`
	Stores       []EtaAccuracyStore `json:"stores"`
}

// computeEtaAccuracy 按店聚合（纯函数，便于测试）。每店只取最近 etaBacktestMaxPerStore 条。
func computeEtaAccuracy(samples []etaBacktestSample) EtaAccuracyReport {
	byStore := map[string][]etaBacktestSample{}
	for _, s := range samples {
		byStore[s.StoreID] = append(byStore[s.StoreID], s)
	}
	report := EtaAccuracyReport{GeneratedAt: time.Now().Format(time.RFC3339), TotalSamples: len(samples)}
	for store, list := range byStore {
		if len(list) > etaBacktestMaxPerStore {
			list = list[len(list)-etaBacktestMaxPerStore:]
		}
		var absSum, worst float64
		signed := make([]float64, 0, len(list))
		for _, s := range list {
			a := math.Abs(s.ErrorMin)
			absSum += a
			if a > worst {
				worst = a
			}
			signed = append(signed, s.ErrorMin)
		}
		report.Stores = append(report.Stores, EtaAccuracyStore{
			StoreID:  store,
			Samples:  len(list),
			MAEMin:   round1(absSum / float64(len(list))),
			BiasMin:  round1(medianFloat(signed)),
			WorstMin: round1(worst),
		})
	}
	sort.Slice(report.Stores, func(i, j int) bool {
		if report.Stores[i].Samples != report.Stores[j].Samples {
			return report.Stores[i].Samples > report.Stores[j].Samples
		}
		return report.Stores[i].StoreID < report.Stores[j].StoreID
	})
	return report
}

func getEtaAccuracyReport() EtaAccuracyReport {
	etaBacktestMu.Lock()
	samples := loadEtaBacktestSamples()
	etaBacktestMu.Unlock()
	return computeEtaAccuracy(samples)
}

// etaCalibration 区间校准量：偏差 + 额外加宽。
type etaCalibration struct {
	BiasMin     float64 // 实际相对预测的系统性偏差（正=该往后挪）
	ExtraSpread float64 // 半宽外扩（分钟），按历史 MAE 给
	Samples     int
}

// storeEtaCalibration 取某店的校准量；样本不足返回 ok=false。
func storeEtaCalibration(storeID string) (etaCalibration, bool) {
	if storeID == "" {
		return etaCalibration{}, false
	}
	report := getEtaAccuracyReport()
	for _, s := range report.Stores {
		if s.StoreID != storeID {
			continue
		}
		if s.Samples < etaCalibMinSamples {
			return etaCalibration{}, false
		}
		return etaCalibration{
			BiasMin:     s.BiasMin,
			ExtraSpread: s.MAEMin * 0.5, // 用半个 MAE 适度加宽，避免区间过度膨胀
			Samples:     s.Samples,
		}, true
	}
	return etaCalibration{}, false
}

func medianFloat(v []float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	cp := append([]float64(nil), v...)
	sort.Float64s(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
