package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestBuildRemoteQueuePressureCurvePoints(t *testing.T) {
	storeID := "3006"
	baseline := QueueBaselineExport{
		Rollups: []QueueBaselineRollup{
			{
				StoreID:            3006,
				DateType:           "weekday",
				TimeBucket:         "10:00",
				SampleCount:        12,
				WaitTypicalMinutes: floatPtr(55),
				QueueGroupsTypical: floatPtr(32),
				CalledSampleCount:  10,
				CalledNoTypical:    floatPtr(120),
			},
			{
				StoreID:            3006,
				DateType:           "weekend",
				TimeBucket:         "10:00",
				SampleCount:        12,
				WaitTypicalMinutes: floatPtr(120),
				QueueGroupsTypical: floatPtr(90),
				CalledSampleCount:  10,
				CalledNoTypical:    floatPtr(900),
			},
			{
				StoreID:            3006,
				DateType:           "weekday",
				TimeBucket:         "10:30",
				SampleCount:        20,
				WaitTypicalMinutes: floatPtr(70),
				QueueGroupsTypical: floatPtr(64),
				CalledSampleCount:  18,
				CalledNoTypical:    floatPtr(180),
			},
		},
		Latest: []QueueBaselineLatest{
			{
				StoreID:          3006,
				CollectedAt:      "2026-06-08T11:03:00+08:00",
				WaitMinutes:      80,
				GroupQueuesCount: 70,
				DisplayCalledNo:  220,
			},
			{
				StoreID:          3006,
				CollectedAt:      "2026-06-09T11:03:00+08:00",
				WaitMinutes:      10,
				GroupQueuesCount: 5,
				DisplayCalledNo:  20,
			},
		},
	}

	points := buildRemoteQueuePressureCurvePoints(storeID, "2026-06-08", "weekday", baseline)
	if len(points) != 3 {
		t.Fatalf("points len=%d want 3: %+v", len(points), points)
	}
	if points[0].Time != "10:00" || points[0].CalledNo != 120 || points[0].WaitingGroups != 32 || points[0].OfficialWaitMinutes != 55 {
		t.Fatalf("first point mismatch: %+v", points[0])
	}
	if points[0].Source != "remote_baseline" || points[0].Confidence == "" {
		t.Fatalf("first point missing source/confidence: %+v", points[0])
	}
	if points[2].Time != "11:03" || points[2].Source != "remote_latest" || points[2].CalledNo != 220 {
		t.Fatalf("latest point mismatch: %+v", points[2])
	}
}

func TestBuildQueuePressureCurveExplainsConnectedCloudWithNoStoreData(t *testing.T) {
	resetQueueBaselineRemoteCacheForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(queueBaselineTursoURLEnv, "")
	t.Setenv(queueBaselineTursoTokenEnv, "")
	t.Setenv(queueBaselineTursoFallbackURL, "")
	t.Setenv(queueBaselineTursoFallbackAuth, "")
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/baseline/store" {
			t.Fatalf("path = %s, want /api/queue/baseline/store", r.URL.Path)
		}
		writeJSON(w, QueueBaselineExport{
			Version:       1,
			GeneratedAt:   "2026-06-08T10:00:00+08:00",
			Source:        "turso-cloudflare",
			BucketMinutes: 10,
			DateTypes:     []string{"weekday"},
			Stats:         QueueBaselineStats{},
		})
	}))
	defer server.Close()
	if err := SaveCloudAuthConfig(CloudAuthConfig{BaseURL: server.URL, SessionToken: "cloud-session", UserLogin: "octocat"}); err != nil {
		t.Fatal(err)
	}

	curve := buildQueuePressureCurve(context.Background(), "3006", "2026-06-08", time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))

	if curve.Baseline.Provider != "cloudflare" || !curve.Baseline.Used {
		t.Fatalf("baseline = %+v, want used cloudflare", curve.Baseline)
	}
	if !strings.Contains(curve.Message, "线上 Turso 基准已连接") {
		t.Fatalf("message = %q, want connected cloud no-data explanation", curve.Message)
	}
}

func TestMergeQueuePressureCurvePointsPrefersLocal(t *testing.T) {
	remote := []QueuePressureCurvePoint{{
		Time:          "10:00",
		CalledNo:      100,
		Source:        "remote_baseline",
		PressureLevel: "medium",
	}}
	local := []QueuePressureCurvePoint{{
		Time:          "10:00",
		CalledNo:      140,
		Source:        "local",
		PressureLevel: "low",
	}}

	points := mergeQueuePressureCurvePoints(remote, local)
	if len(points) != 1 {
		t.Fatalf("merged len=%d want 1: %+v", len(points), points)
	}
	if points[0].Source != "local" || points[0].CalledNo != 140 {
		t.Fatalf("local point should win: %+v", points[0])
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
	// 速度 + 官方等待：用户不可能比门店整体更快叫到，官方等待抬低下界而非压缩。
	// remaining=178/rate=2 → base≈89，low=floor(89*0.85)=75，high=ceil(89*1.2)=107。
	// officialWait=120 高于本机估算区间 → low/high 都被抬到 120。
	if wr, src := estimateWaitRange(178, 120, 2.0, nil); wr == nil || src != "recent_speed" {
		t.Fatalf("official+speed range nil/src=%q", src)
	} else if wr.Low < 120 || wr.High < 120 {
		t.Errorf("officialWait should raise floor (max), got Low=%d High=%d want >=120", wr.Low, wr.High)
	}
	// officialWait 远小于本机估算（号码靠后）时不应压缩 low。
	if wr, _ := estimateWaitRange(178, 30, 2.0, nil); wr == nil || wr.Low < 75 {
		t.Errorf("small officialWait must not shrink low, got Low=%d want >=75", wr.Low)
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

	// 已轮到：targetNo==calledNo 是真的即将轮到。
	done := computeQueueEta(950, 950, 0, 90, 2.0, nil, now)
	if done.RemainingGroups != 0 || done.Risk != "low" {
		t.Errorf("done eta=%+v", done)
	}

	// 过号：targetNo<calledNo（哪怕只差一个号）也不能再说“已轮到、低风险”，
	// 寿司郎过号不会自动叫到，需补号或重新取号，与 dashboard 路径口径一致。
	passed := computeQueueEta(900, 950, 0, 90, 2.0, nil, now)
	if passed.Risk != "high" || !strings.Contains(passed.ArrivalSuggestion, "过号") {
		t.Errorf("passed eta should be high risk + 过号 hint, got %+v", passed)
	}
	passedBy1 := computeQueueEta(949, 950, 0, 90, 2.0, nil, now)
	if passedBy1.Risk != "high" || !strings.Contains(passedBy1.ArrivalSuggestion, "过号") {
		t.Errorf("passed-by-1 eta should be high risk + 过号 hint, got %+v", passedBy1)
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

	// 只有官方等待：只能作为门店压力参考，不能包装成到号码的 ETA。
	officialOnly := computeQueueEta(1078, 900, 0, 90, 0, nil, now)
	if officialOnly.WaitMinutesRange != nil || officialOnly.Source != "official" || officialOnly.Risk != "high" {
		t.Fatalf("official-only ETA should not estimate target number: %+v", officialOnly)
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
