package app

import (
	"math"
	"testing"
	"time"
)

func TestComputeEtaAccuracyAggregation(t *testing.T) {
	samples := []etaBacktestSample{
		{StoreID: "A", ErrorMin: 10},
		{StoreID: "A", ErrorMin: 20},
		{StoreID: "A", ErrorMin: -6},
		{StoreID: "B", ErrorMin: -30},
	}
	rep := computeEtaAccuracy(samples)
	if rep.TotalSamples != 4 {
		t.Fatalf("total=%d want 4", rep.TotalSamples)
	}
	// 按样本数降序：A(3) 在前。
	if rep.Stores[0].StoreID != "A" {
		t.Fatalf("expected A first, got %s", rep.Stores[0].StoreID)
	}
	a := rep.Stores[0]
	// MAE = (10+20+6)/3 = 12
	if a.MAEMin != 12 {
		t.Errorf("A MAE=%v want 12", a.MAEMin)
	}
	// 有符号中位 = 中位{-6,10,20} = 10
	if a.BiasMin != 10 {
		t.Errorf("A bias=%v want 10", a.BiasMin)
	}
	if a.WorstMin != 20 {
		t.Errorf("A worst=%v want 20", a.WorstMin)
	}
}

func TestApplyEtaCalibrationShiftsAndWidens(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	eta := &QueueAdvisorEta{
		TargetNo:               1078,
		WaitMinutesRange:       &QueueWaitRange{Low: 30, High: 50},
		EstimatedCalledAtRange: &QueueTimeRange{},
		EstimatedCalledAt:      now.Add(40 * time.Minute).Format(time.RFC3339),
	}
	// 偏差 +10（通常偏晚），额外加宽 5。
	applyEtaCalibration(eta, etaCalibration{BiasMin: 10, ExtraSpread: 5, Samples: 8}, 0, now)
	if eta.WaitMinutesRange.Low != 35 { // 30+10-5
		t.Errorf("low=%d want 35", eta.WaitMinutesRange.Low)
	}
	if eta.WaitMinutesRange.High != 65 { // 50+10+5
		t.Errorf("high=%d want 65", eta.WaitMinutesRange.High)
	}
	if eta.AccuracyNote == "" {
		t.Errorf("应生成可信度话术")
	}
}

func TestApplyEtaCalibrationClampsNegative(t *testing.T) {
	now := time.Now()
	eta := &QueueAdvisorEta{
		WaitMinutesRange:       &QueueWaitRange{Low: 2, High: 8},
		EstimatedCalledAtRange: &QueueTimeRange{},
	}
	applyEtaCalibration(eta, etaCalibration{BiasMin: -20, ExtraSpread: 3, Samples: 5}, 0, now)
	if eta.WaitMinutesRange.Low < 0 {
		t.Errorf("low 不能为负: %d", eta.WaitMinutesRange.Low)
	}
	if eta.WaitMinutesRange.High < eta.WaitMinutesRange.Low {
		t.Errorf("high 不应小于 low")
	}
}

// TestEtaBacktestRoundTrip 端到端验证记录→结算：用临时 HOME 隔离磁盘。
func TestEtaBacktestRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	// 预测 1078 号 12:40 叫到，当前叫到 1051。
	predicted := now.Add(40 * time.Minute)
	recordEtaPrediction("STORE1", 1078, 1051, predicted, now)

	// 实际 12:52 观测到已叫到 1080（晚了 12 分钟）。
	actual := now.Add(52 * time.Minute)
	backfillEtaOnObservation("STORE1", 1080, actual)

	rep := getEtaAccuracyReport()
	if rep.TotalSamples != 1 {
		t.Fatalf("应结算出 1 条样本, got %d", rep.TotalSamples)
	}
	if got := rep.Stores[0].BiasMin; math.Abs(got-12) > 0.01 {
		t.Fatalf("误差应约 +12 分钟（偏晚）, got %v", got)
	}
	// 二次回填不应重复结算（开放预测已删除）。
	backfillEtaOnObservation("STORE1", 1090, actual.Add(time.Minute))
	if getEtaAccuracyReport().TotalSamples != 1 {
		t.Fatalf("不应重复结算")
	}
}

func TestRecordEtaPredictionKeepsFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	now := time.Now()
	first := now.Add(40 * time.Minute)
	second := now.Add(10 * time.Minute)
	recordEtaPrediction("S", 100, 50, first, now)
	recordEtaPrediction("S", 100, 90, second, now.Add(5*time.Minute)) // 更近的预测不应覆盖

	etaBacktestMu.Lock()
	m := loadEtaOpenLocked()
	etaBacktestMu.Unlock()
	rec, ok := m[etaOpenKey("S", 100)]
	if !ok {
		t.Fatal("应存在开放预测")
	}
	if !rec.PredictedCalledAt.Equal(first) {
		t.Fatalf("应保留首次预测时间")
	}
}
