package app

import (
	"testing"
	"time"
)

func obsAt(storeID string, called, groups, wait int, at time.Time) QueueObservation {
	return QueueObservation{
		StoreID:          storeID,
		DisplayCalledNo:  called,
		GroupQueuesCount: groups,
		WaitMinutes:      wait,
		CollectedAt:      at.Format(time.RFC3339),
	}
}

func TestQueuePressureLevel(t *testing.T) {
	cases := []struct {
		groups, wait int
		want         string
	}{
		{0, 0, "unknown"},
		{10, 0, "low"},
		{0, 15, "low"},
		{50, 0, "medium"},
		{0, 55, "medium"},
		{100, 0, "high"},
		{0, 110, "high"},
		{200, 0, "extreme"},
		{0, 300, "extreme"},
	}
	for _, c := range cases {
		if got := queuePressureLevel(c.groups, c.wait); got != c.want {
			t.Errorf("queuePressureLevel(%d,%d)=%q want %q", c.groups, c.wait, got, c.want)
		}
	}
}

func TestQueuePressureScore(t *testing.T) {
	if s := queuePressureScore(0, 0); s != 0 {
		t.Errorf("empty score=%d want 0", s)
	}
	if s := queuePressureScore(500, 0); s != 100 {
		t.Errorf("huge score=%d want capped 100", s)
	}
	low := queuePressureScore(20, 0)
	high := queuePressureScore(120, 0)
	if !(low < high) {
		t.Errorf("score not monotonic: low=%d high=%d", low, high)
	}
}

func TestCalledRateOverWindow(t *testing.T) {
	now := time.Now()
	store := "3006"
	// 30 分钟推进 60 个号 → 2 桌/分钟。
	obs := []QueueObservation{
		obsAt(store, 100, 50, 60, now.Add(-30*time.Minute)),
		obsAt(store, 160, 40, 50, now),
	}
	rate := calledRateOverWindow(obs, store, now, queueAdvisorWindow60)
	if rate == nil {
		t.Fatal("expected rate, got nil")
	}
	if *rate < 1.9 || *rate > 2.1 {
		t.Errorf("rate=%.2f want ~2.0", *rate)
	}

	// 只有一个点 → nil。
	one := []QueueObservation{obsAt(store, 100, 50, 60, now)}
	if r := calledRateOverWindow(one, store, now, queueAdvisorWindow60); r != nil {
		t.Errorf("single point should be nil, got %v", *r)
	}

	// 叫号回退（队列重置）→ diff<=0 → nil。
	back := []QueueObservation{
		obsAt(store, 900, 50, 60, now.Add(-20*time.Minute)),
		obsAt(store, 50, 40, 50, now),
	}
	if r := calledRateOverWindow(back, store, now, queueAdvisorWindow60); r != nil {
		t.Errorf("rollback should be nil, got %v", *r)
	}
}

func TestQueuePressureTrend(t *testing.T) {
	now := time.Now()
	store := "3006"
	improving := []QueueObservation{
		obsAt(store, 100, 80, 90, now.Add(-15*time.Minute)),
		obsAt(store, 140, 60, 70, now),
	}
	if level, _ := queuePressureTrend(improving, 2.0, false); level != "improving" {
		t.Errorf("improving trend=%q", level)
	}
	worsening := []QueueObservation{
		obsAt(store, 100, 60, 70, now.Add(-15*time.Minute)),
		obsAt(store, 110, 90, 100, now),
	}
	if level, _ := queuePressureTrend(worsening, 1.0, false); level != "worsening" {
		t.Errorf("worsening trend=%q", level)
	}
	if level, _ := queuePressureTrend(improving, 2.0, true); level != "stalled" {
		t.Errorf("stalled override trend=%q", level)
	}
	if level, _ := queuePressureTrend(nil, 0, false); level != "unknown" {
		t.Errorf("no data trend=%q", level)
	}
}

func TestEstimateWaitRange(t *testing.T) {
	// 速度优先。
	if wr, src := estimateWaitRange(100, 0, 2.0, nil); wr == nil || src != "recent_speed" {
		t.Fatalf("speed range nil/src=%q", src)
	}
	// 无速度退化到官方。
	wr, src := estimateWaitRange(0, 60, 0, nil)
	if wr == nil || src != "official" {
		t.Fatalf("official range nil/src=%q", src)
	}
	// 无速度无官方退化到历史。
	hist := &QueueWaitRange{Low: 40, High: 70}
	wr2, src2 := estimateWaitRange(0, 0, 0, hist)
	if wr2 == nil || src2 != "history" || wr2.Low != 40 || wr2.High != 70 {
		t.Fatalf("history range %+v src=%q", wr2, src2)
	}
	// 全缺 → unknown。
	if wr3, src3 := estimateWaitRange(0, 0, 0, nil); wr3 != nil || src3 != "unknown" {
		t.Fatalf("unknown range %+v src=%q", wr3, src3)
	}
}

func TestComputeQueueEta(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 30, 0, 0, time.Local)

	// 已轮到。
	done := computeQueueEta(900, 950, 0, 90, 2.0, nil, now)
	if done.RemainingGroups != 0 || done.Risk != "low" {
		t.Errorf("done eta=%+v", done)
	}

	// 正常：还差 178，速度 2/分 → ~89 分钟，带区间与出发建议。
	eta := computeQueueEta(1078, 900, 25, 90, 2.0, nil, now)
	if eta.RemainingGroups != 178 {
		t.Errorf("remaining=%d want 178", eta.RemainingGroups)
	}
	if eta.WaitMinutesRange == nil || eta.EstimatedCalledAtRange == nil {
		t.Fatalf("missing range: %+v", eta)
	}
	if eta.ArrivalSuggestion == "" {
		t.Error("missing arrival suggestion")
	}
	if eta.Risk == "unknown" {
		t.Error("risk should be resolved with speed source")
	}

	// 数据不足：无速度、无官方、无历史 → 无法预估。
	none := computeQueueEta(1078, 900, 0, 0, 0, nil, now)
	if none.EstimatedCalledAt != "" || none.WaitMinutesRange != nil {
		t.Errorf("insufficient data should not estimate: %+v", none)
	}
}

func TestParseHHMM(t *testing.T) {
	now := time.Date(2026, 6, 8, 0, 0, 0, 0, time.Local)
	for _, raw := range []string{"1210", "12:10"} {
		ts, ok := parseHHMM(raw, now)
		if !ok || ts.Hour() != 12 || ts.Minute() != 10 {
			t.Errorf("parseHHMM(%q)=%v ok=%v", raw, ts, ok)
		}
	}
	for _, bad := range []string{"", "9", "2510", "1260", "abcd"} {
		if _, ok := parseHHMM(bad, now); ok {
			t.Errorf("parseHHMM(%q) should fail", bad)
		}
	}
}
