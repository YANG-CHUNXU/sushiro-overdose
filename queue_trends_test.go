package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildQueueLocalStatsAggregatesPrivateSessions(t *testing.T) {
	sessions := []QueueSession{
		{StoreID: "001", TakenAt: "2026-05-16T18:00:00+08:00", TicketNo: 101, DisplayCalledNoAtTake: 80, CalledForUserAt: "2026-05-16T18:40:00+08:00", CalledNoWhenUserCalled: 101, ActualWaitMinutes: 40, CheckedInAt: "2026-05-16T18:20:00+08:00", PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-23T18:10:00+08:00", TicketNo: 102, DisplayCalledNoAtTake: 84, CalledForUserAt: "2026-05-23T19:00:00+08:00", CalledNoWhenUserCalled: 102, ActualWaitMinutes: 50, CheckedInAt: "2026-05-23T18:35:00+08:00", PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-30T18:25:00+08:00", TicketNo: 103, DisplayCalledNoAtTake: 88, CalledForUserAt: "2026-05-30T19:25:00+08:00", CalledNoWhenUserCalled: 103, ActualWaitMinutes: 60, CheckedInAt: "2026-05-30T18:55:00+08:00", PartySize: 2, TableType: "T", ExpiredOrMissed: true},
		{StoreID: "002", TakenAt: "2026-05-16T18:00:00+08:00", ActualWaitMinutes: 20, PartySize: 5, TableType: "C"},
	}

	stats, usable := BuildQueueLocalStats(sessions, 3)
	if usable != 4 {
		t.Fatalf("usable sessions = %d, want 4", usable)
	}
	if len(stats) != 1 {
		t.Fatalf("stats len = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.StoreID != "001" || got.Weekday != 6 || got.TimeBucket != "18:00" || got.PartySizeBucket != "1-2" || got.TableType != "T" {
		t.Fatalf("unexpected bucket: %+v", got)
	}
	if got.Samples != 3 {
		t.Fatalf("samples = %d, want 3", got.Samples)
	}
	if got.WaitP50Minutes == nil || *got.WaitP50Minutes != 50 {
		t.Fatalf("wait p50 = %v, want 50", got.WaitP50Minutes)
	}
	if got.WaitP80Minutes == nil || *got.WaitP80Minutes != 56 {
		t.Fatalf("wait p80 = %v, want 56", got.WaitP80Minutes)
	}
	if got.MissedRate < 0.33 || got.MissedRate > 0.34 {
		t.Fatalf("missed rate = %.4f, want about 0.3333", got.MissedRate)
	}
}

func TestQueueTrendActualPassedRequiresConfirmedCalledNumber(t *testing.T) {
	if _, ok := queueActualPassed(QueueSession{TicketNo: 130, DisplayCalledNoAtTake: 100}); ok {
		t.Fatal("actual passed should not be inferred without confirmed called number")
	}
	passed, ok := queueActualPassed(QueueSession{DisplayCalledNoAtTake: 100, CalledNoWhenUserCalled: 128})
	if !ok || passed != 28 {
		t.Fatalf("actual passed = %d/%v, want 28/true", passed, ok)
	}
}

func TestQueueTrendResponseDoesNotExposeRawTicketFields(t *testing.T) {
	sessions := []QueueSession{
		{StoreID: "001", TakenAt: "2026-05-16T18:00:00+08:00", TicketNo: 123, DisplayCalledNoAtTake: 100, CalledNoWhenUserCalled: 123, ActualWaitMinutes: 30, PartySize: 2, TableType: "T"},
	}
	stats, _ := BuildQueueLocalStats(sessions, 1)
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ticket_no", "display_called_no", "wechat", "authorization", "phone", "called_no_when_user_called"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("local aggregate contains raw field %q: %s", forbidden, data)
		}
	}
}

func TestQueueTrendDateTypeUsesLocalHolidayOverrides(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	holiday := time.Date(2026, 5, 1, 18, 0, 0, 0, loc)
	workday := time.Date(2026, 5, 2, 18, 0, 0, 0, loc)
	if got := queueTrendDateType(holiday, map[string]bool{"2026-05-01": true}, nil); got != "holiday" {
		t.Fatalf("holiday type = %s, want holiday", got)
	}
	if got := queueTrendDateType(workday, nil, map[string]bool{"2026-05-02": true}); got != "weekday" {
		t.Fatalf("adjusted workday type = %s, want weekday", got)
	}
}

func TestQueueTrendGlobalPassedUsesForwardObservationDelta(t *testing.T) {
	query := normalizeQueueTrendQuery(QueueTrendQuery{
		StoreIDs:      []string{"001"},
		DateType:      "weekend",
		From:          "2026-05-16",
		To:            "2026-05-16",
		Start:         "18:00",
		End:           "20:00",
		BucketMinutes: 30,
	}, time.Date(2026, 5, 16, 20, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	holidays, workdays := map[string]bool{}, map[string]bool{}
	storeFilter := stringSet(query.StoreIDs)
	series := map[string]*queueTrendAccumulator{}
	summary := QueueTrendSummary{}
	observations := []QueueObservation{
		{StoreID: "001", Timestamp: "2026-05-16T18:00:00+08:00", DisplayCalledNo: 80},
		{StoreID: "001", Timestamp: "2026-05-16T18:20:00+08:00", DisplayCalledNo: 91},
		{StoreID: "001", Timestamp: "2026-05-16T18:40:00+08:00", DisplayCalledNo: 90},
	}
	for i := 1; i < len(observations); i++ {
		prev, curr := observations[i-1], observations[i]
		currAt, _ := parseRFC3339Local(curr.Timestamp)
		if !queueTrendMatches(query, currAt, storeFilter, curr.StoreID, holidays, workdays) {
			continue
		}
		diff := curr.DisplayCalledNo - prev.DisplayCalledNo
		if diff <= 0 {
			continue
		}
		acc := queueTrendAcc(series, curr.StoreID, curr.StoreID, queueTrendDateType(currAt, holidays, workdays), queueTrendBucket(currAt, query.BucketMinutes))
		acc.point.GlobalPassed += diff
		summary.GlobalPassedTotal += diff
	}
	points := finalizeQueueTrendPoints(series)
	if len(points) != 1 {
		t.Fatalf("points len = %d, want 1", len(points))
	}
	if points[0].GlobalPassed != 11 || summary.GlobalPassedTotal != 11 {
		t.Fatalf("global passed = point %d summary %d, want 11", points[0].GlobalPassed, summary.GlobalPassedTotal)
	}
}

func TestBuildQueueTrendRecommendationsPrioritizesUsableStoreTime(t *testing.T) {
	waitFast := 25.0
	waitSlow := 70.0
	points := []QueueTrendPoint{
		{
			StoreID:        "slow",
			StoreName:      "慢店",
			DateType:       "weekend",
			DateTypeName:   "周末",
			Bucket:         "18:30",
			ActualSamples:  8,
			GlobalSamples:  6,
			WaitP50Minutes: &waitSlow,
			Confidence:     "high",
			MissedRate:     0.1,
		},
		{
			StoreID:        "fast",
			StoreName:      "快店",
			DateType:       "weekend",
			DateTypeName:   "周末",
			Bucket:         "17:30",
			ActualSamples:  4,
			GlobalSamples:  5,
			WaitP50Minutes: &waitFast,
			Confidence:     "medium",
			MissedRate:     0.0,
		},
	}

	recommendations := BuildQueueTrendRecommendations(points, 2)
	if len(recommendations) != 2 {
		t.Fatalf("recommendations len = %d, want 2", len(recommendations))
	}
	if recommendations[0].StoreID != "fast" {
		t.Fatalf("top store = %s, want fast: %+v", recommendations[0].StoreID, recommendations)
	}
	if recommendations[0].ActionLabel != "优先考虑" {
		t.Fatalf("action = %s, want 优先考虑", recommendations[0].ActionLabel)
	}
	if !strings.Contains(recommendations[0].Reason, "P50") {
		t.Fatalf("reason should mention P50 wait: %s", recommendations[0].Reason)
	}
}

func TestBuildQueueTrendRecommendationsSkipsEmptyPoints(t *testing.T) {
	recommendations := BuildQueueTrendRecommendations([]QueueTrendPoint{
		{StoreID: "empty", StoreName: "空", DateType: "weekday", Bucket: "12:00"},
	}, 5)
	if len(recommendations) != 0 {
		t.Fatalf("recommendations len = %d, want 0", len(recommendations))
	}
}
