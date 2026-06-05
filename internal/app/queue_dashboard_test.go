package app

import (
	. "github.com/Ryujoxys/sushiro-overdose/internal/core"

	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBuildQueueDashboardUsesLocalObservationShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	data := `{"ts":"2026-06-04T17:30:00+08:00","store_id":"3006","display_called_no":100,"wait_minutes":60,"group_queues_count":10,"store_status":"OPEN","net_ticket_status":"ONLINE","online_open":true}` + "\n" +
		`{"collected_at":"2026-06-04T18:10:00+08:00","store_id":"3006","display_called_no":130,"wait_minutes":80,"group_queues_count":24,"store_status":"OPEN","net_ticket_status":"ONLINE","reservation_status":"ON","online_open":true,"source_endpoint":"getStoreById","api_profile_version":"store-detail-profile-v1"}` + "\n"
	if err := os.WriteFile(queueObservationPath(), []byte(data), 0o600); err != nil {
		t.Fatalf("write observations: %v", err)
	}
	now := time.Date(2026, 6, 4, 18, 30, 0, 0, time.FixedZone("CST", 8*3600))
	got := BuildQueueDashboardWithContext(context.Background(), QueueDashboardQuery{
		StoreIDs:    []string{"3006"},
		Scope:       "local",
		WindowHours: 6,
	}, now)
	if got.Summary.StoreCount != 1 || got.Summary.TotalQueueGroups != 24 || got.Summary.OpenStores != 1 {
		t.Fatalf("unexpected summary: %+v", got.Summary)
	}
	if len(got.Trend) != 2 || got.Summary.TrendDelta != 14 {
		t.Fatalf("unexpected trend: summary=%+v trend=%+v", got.Summary, got.Trend)
	}
	if got.CalledSummary.StoreID != "3006" || got.CalledSummary.LatestCalledNo != 130 || got.CalledSummary.PointCount != 2 {
		t.Fatalf("unexpected called summary: %+v", got.CalledSummary)
	}
	if len(got.CalledCurve) != 2 || got.CalledCurve[0].Bucket != "17:30" || got.CalledCurve[1].CalledNoTypical != 130 {
		t.Fatalf("unexpected called curve: %+v", got.CalledCurve)
	}
}

func TestQueueDashboardRollupsExcludeHolidayFromAllWeekdayViews(t *testing.T) {
	query := normalizeQueueDashboardQuery(QueueDashboardQuery{DateType: "all"})
	rollups := []QueueBaselineRollup{
		{
			StoreID:            3006,
			DateType:           "weekday",
			Weekday:            1,
			TimeBucket:         "18:00",
			SampleCount:        10,
			QueueGroupsTypical: floatPtr(20),
			WaitTypicalMinutes: floatPtr(40),
			OnlineOpenRate:     1,
			BusyRate:           0.5,
		},
		{
			StoreID:            3006,
			DateType:           "holiday",
			Weekday:            1,
			TimeBucket:         "18:00",
			SampleCount:        10,
			QueueGroupsTypical: floatPtr(99),
			WaitTypicalMinutes: floatPtr(120),
			OnlineOpenRate:     1,
			BusyRate:           1,
		},
	}
	weekdays, heatmap, dateTypes := buildQueueDashboardRollupViews(query, rollups, nil)
	if len(weekdays) != 1 || weekdays[0].QueueGroupsAvg == nil || *weekdays[0].QueueGroupsAvg != 20 {
		t.Fatalf("holiday leaked into weekday profiles: %+v", weekdays)
	}
	if len(heatmap) != 1 || heatmap[0].QueueGroupsAvg == nil || *heatmap[0].QueueGroupsAvg != 20 {
		t.Fatalf("holiday leaked into heatmap: %+v", heatmap)
	}
	if len(dateTypes) != 2 {
		t.Fatalf("date type summaries should still expose separated holiday bucket: %+v", dateTypes)
	}
}

func TestQueueDashboardCalledCurveUsesDisplayHoursAndLatestBucketSample(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	data := `{"collected_at":"2026-06-04T09:50:00+08:00","store_id":"3006","display_called_no":90,"wait_minutes":10,"group_queues_count":1}` + "\n" +
		`{"collected_at":"2026-06-04T10:01:00+08:00","store_id":"3006","display_called_no":100,"wait_minutes":20,"group_queues_count":2}` + "\n" +
		`{"collected_at":"2026-06-04T10:08:00+08:00","store_id":"3006","display_called_no":110,"wait_minutes":30,"group_queues_count":3}` + "\n" +
		`{"collected_at":"2026-06-04T10:12:00+08:00","store_id":"3006","display_called_no":120,"wait_minutes":40,"group_queues_count":4}` + "\n" +
		`{"collected_at":"2026-06-04T22:00:00+08:00","store_id":"3006","display_called_no":900,"wait_minutes":50,"group_queues_count":5}` + "\n" +
		`{"collected_at":"2026-06-04T22:01:00+08:00","store_id":"3006","display_called_no":901,"wait_minutes":60,"group_queues_count":6}` + "\n"
	if err := os.WriteFile(queueObservationPath(), []byte(data), 0o600); err != nil {
		t.Fatalf("write observations: %v", err)
	}
	now := time.Date(2026, 6, 4, 23, 0, 0, 0, time.FixedZone("CST", 8*3600))
	got := BuildQueueDashboardWithContext(context.Background(), QueueDashboardQuery{
		StoreIDs:      []string{"3006"},
		Scope:         "local",
		BucketMinutes: 10,
	}, now)
	wantBuckets := []string{"10:00", "10:10", "22:00"}
	if len(got.CalledCurve) != len(wantBuckets) {
		t.Fatalf("called curve len = %d, want %d: %+v", len(got.CalledCurve), len(wantBuckets), got.CalledCurve)
	}
	for i, bucket := range wantBuckets {
		if got.CalledCurve[i].Bucket != bucket {
			t.Fatalf("bucket[%d] = %s, want %s: %+v", i, got.CalledCurve[i].Bucket, bucket, got.CalledCurve)
		}
	}
	if got.CalledCurve[0].CalledNoTypical != 110 || got.CalledCurve[0].LatestCalledNo != 110 {
		t.Fatalf("same-bucket latest sample not used: %+v", got.CalledCurve[0])
	}
	if got.CalledSummary.SampleCount != 3 || got.CalledSummary.LatestBucket != "22:00" || got.CalledSummary.LatestCalledNo != 900 {
		t.Fatalf("unexpected called summary: %+v", got.CalledSummary)
	}
}

func TestQueueDashboardRemoteCalledCurveUsesTursoRollups(t *testing.T) {
	query := normalizeQueueDashboardQuery(QueueDashboardQuery{
		Scope:         "all",
		DateType:      "all",
		BucketMinutes: 10,
	})
	baseline := QueueBaselineExport{
		BucketMinutes: 10,
		Stats: QueueBaselineStats{
			SourceUpdatedAt: "2026-06-04T22:10:00+08:00",
		},
		Rollups: []QueueBaselineRollup{
			{
				StoreID:            3006,
				DateType:           "weekday",
				Weekday:            1,
				TimeBucket:         "09:50",
				SampleCount:        10,
				CalledSampleCount:  10,
				CalledNoTypical:    floatPtr(90),
				QueueGroupsTypical: floatPtr(1),
			},
			{
				StoreID:            3006,
				DateType:           "weekday",
				Weekday:            1,
				TimeBucket:         "10:00",
				SampleCount:        10,
				WaitTypicalMinutes: floatPtr(25),
				QueueGroupsTypical: floatPtr(8),
				CalledSampleCount:  10,
				CalledNoSlow:       floatPtr(100),
				CalledNoTypical:    floatPtr(120),
				CalledNoFast:       floatPtr(150),
				UpdatedAt:          "2026-06-04T22:00:00+08:00",
			},
			{
				StoreID:            3006,
				DateType:           "weekend",
				Weekday:            6,
				TimeBucket:         "10:00",
				SampleCount:        30,
				WaitTypicalMinutes: floatPtr(45),
				QueueGroupsTypical: floatPtr(16),
				CalledSampleCount:  30,
				CalledNoSlow:       floatPtr(140),
				CalledNoTypical:    floatPtr(180),
				CalledNoFast:       floatPtr(220),
				UpdatedAt:          "2026-06-04T22:05:00+08:00",
			},
			{
				StoreID:           4000,
				DateType:          "weekday",
				Weekday:           1,
				TimeBucket:        "10:00",
				CalledSampleCount: 1,
				CalledNoTypical:   floatPtr(500),
			},
		},
	}
	latest := []QueueDashboardStoreRow{{
		StoreID:     "3006",
		StoreName:   "太阳宫凯德店",
		CalledNo:    540,
		QueueGroups: 12,
		WaitMinutes: 30,
		LatestAt:    "2026-06-04T21:58:00+08:00",
	}}
	summary, curve := buildQueueDashboardRemoteCalledCurve(query, baseline, map[string]string{"3006": "太阳宫凯德店"}, nil, latest)
	if summary.Source != "remote_baseline" || summary.StoreID != "3006" || summary.LatestCalledNo != 540 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(curve) != 1 {
		t.Fatalf("curve len = %d, want 1: %+v", len(curve), curve)
	}
	point := curve[0]
	if point.Bucket != "10:00" || point.CalledNoSlow != 130 || point.CalledNoTypical != 165 || point.CalledNoFast != 203 {
		t.Fatalf("unexpected called point: %+v", point)
	}
	if point.QueueGroups != 14 || point.WaitMinutes != 40 || point.SampleCount != 40 || point.DayCount != 2 {
		t.Fatalf("unexpected weighted metrics: %+v", point)
	}
}

func TestQueueDashboardAdvisorMapsTargetNumberToBucket(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	curve := []QueueDashboardCalledPoint{
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "18:10", CalledNoTypical: 760, CalledNoSlow: 720, CalledNoFast: 810, Confidence: "medium", Source: "local"},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "18:30", CalledNoTypical: 840, CalledNoSlow: 800, CalledNoFast: 890, Confidence: "medium", Source: "local"},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "18:40", CalledNoTypical: 900, CalledNoSlow: 860, CalledNoFast: 940, Confidence: "high", Source: "local"},
	}
	summary := QueueDashboardCalledSummary{
		StoreID:        "3006",
		StoreName:      "太阳宫凯德店",
		LatestCalledNo: 760,
		Confidence:     "medium",
		Source:         "local",
	}
	got := buildQueueDashboardAdvisor(QueueDashboardQuery{BucketMinutes: 10, TargetNo: 893}, summary, curve, nil, time.Date(2026, 6, 5, 18, 0, 0, 0, loc))
	if got.State != "target" || got.TargetBucket != "18:40" || got.ArrivalBucket != "18:20" {
		t.Fatalf("unexpected advisor: %+v", got)
	}
	if got.Headline != "预计 18:40 左右叫到 893 号" {
		t.Fatalf("headline = %q", got.Headline)
	}
	if len(got.Milestones) != 1 || got.Milestones[0].CalledNoTypical != 900 {
		t.Fatalf("unexpected milestones: %+v", got.Milestones)
	}
}

func TestQueueDashboardAdvisorMarksAlreadyPassedTarget(t *testing.T) {
	curve := []QueueDashboardCalledPoint{
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "18:30", CalledNoTypical: 840, Confidence: "medium", Source: "local"},
	}
	summary := QueueDashboardCalledSummary{
		StoreID:        "3006",
		StoreName:      "太阳宫凯德店",
		LatestBucket:   "18:35",
		LatestCalledNo: 910,
		Confidence:     "medium",
		Source:         "local",
	}
	got := buildQueueDashboardAdvisor(QueueDashboardQuery{BucketMinutes: 10, TargetNo: 893}, summary, curve, nil, time.Now())
	if got.State != "passed" || got.TargetBucket != "18:35" {
		t.Fatalf("unexpected advisor: %+v", got)
	}
	if got.Copy != "893 号可能已经过号；请用手机小程序确认现场状态。" {
		t.Fatalf("copy = %q", got.Copy)
	}
}

func TestQueueDashboardAdvisorHandlesUncoveredTargetNumber(t *testing.T) {
	curve := []QueueDashboardCalledPoint{
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "21:30", CalledNoTypical: 920, Confidence: "low", Source: "remote_baseline"},
	}
	got := buildQueueDashboardAdvisor(QueueDashboardQuery{BucketMinutes: 10, TargetNo: 1100}, QueueDashboardCalledSummary{StoreID: "3006", Source: "remote_baseline"}, curve, nil, time.Now())
	if got.State != "uncovered" || got.TargetBucket != "21:30" {
		t.Fatalf("unexpected advisor: %+v", got)
	}
	if got.Headline != "样本还没覆盖到 1100 号" {
		t.Fatalf("headline = %q", got.Headline)
	}
}

func TestQueueDashboardAdvisorReturnsMilestonesWithoutTarget(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	curve := []QueueDashboardCalledPoint{
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "18:00", CalledNoTypical: 700, Confidence: "medium", Source: "local"},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "19:00", CalledNoTypical: 820, Confidence: "medium", Source: "local"},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "20:00", CalledNoTypical: 920, Confidence: "medium", Source: "local"},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Bucket: "21:30", CalledNoTypical: 1030, Confidence: "medium", Source: "local"},
	}
	got := buildQueueDashboardAdvisor(QueueDashboardQuery{BucketMinutes: 10}, QueueDashboardCalledSummary{StoreID: "3006", Source: "local"}, curve, nil, time.Date(2026, 6, 5, 18, 0, 0, 0, loc))
	if got.State != "milestones" || len(got.Milestones) < 3 {
		t.Fatalf("unexpected advisor: %+v", got)
	}
	if got.Milestones[0].Label != "现在附近" || got.Milestones[0].Bucket != "18:00" {
		t.Fatalf("unexpected first milestone: %+v", got.Milestones)
	}
}

func TestQueueDashboardHTTPContractIncludesAdvisor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	data := `{"collected_at":"2026-06-04T18:10:00+08:00","store_id":"3006","display_called_no":760,"wait_minutes":30,"group_queues_count":8,"store_status":"OPEN","net_ticket_status":"ONLINE","online_open":true}` + "\n" +
		`{"collected_at":"2026-06-04T18:30:00+08:00","store_id":"3006","display_called_no":840,"wait_minutes":40,"group_queues_count":12,"store_status":"OPEN","net_ticket_status":"ONLINE","online_open":true}` + "\n" +
		`{"collected_at":"2026-06-04T18:40:00+08:00","store_id":"3006","display_called_no":900,"wait_minutes":45,"group_queues_count":14,"store_status":"OPEN","net_ticket_status":"ONLINE","online_open":true}` + "\n" +
		`{"collected_at":"2026-06-05T10:10:00+08:00","store_id":"3006","display_called_no":100,"wait_minutes":20,"group_queues_count":4,"store_status":"OPEN","net_ticket_status":"ONLINE","online_open":true}` + "\n"
	if err := os.WriteFile(queueObservationPath(), []byte(data), 0o600); err != nil {
		t.Fatalf("write observations: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/queue/dashboard?scope=local&stores=3006&target_no=893&bucket=10", nil)
	rec := httptest.NewRecorder()
	handleQueueDashboard(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got QueueDashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if got.Advisor.TargetNo != 893 || got.Advisor.TargetBucket != "18:40" || got.Advisor.ArrivalLabel != "18:20 前到店" {
		t.Fatalf("unexpected advisor contract: %+v", got.Advisor)
	}
}
