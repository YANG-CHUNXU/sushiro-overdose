package main

import (
	"math"
	"testing"
	"time"
)

func insightTestNow(t *testing.T) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return time.Date(2026, 5, 15, 12, 0, 0, 0, loc)
}

func insightSnapshot(ts, storeID, date, start, end, availability string) SlotSnapshot {
	return SlotSnapshot{
		Timestamp:    ts,
		StoreID:      storeID,
		Date:         date,
		Start:        start,
		End:          end,
		Availability: availability,
	}
}

func findInsightStat(t *testing.T, analysis SlotHistoryAnalysis, storeID string, weekday int, start string) SlotHistoryStat {
	t.Helper()
	for _, store := range analysis.Stores {
		if store.StoreID != storeID {
			continue
		}
		for _, day := range store.Weekdays {
			if day.Weekday != weekday {
				continue
			}
			for _, stat := range day.Slots {
				if stat.Start == start {
					return stat
				}
			}
		}
	}
	t.Fatalf("stat not found: store=%s weekday=%d start=%s", storeID, weekday, start)
	return SlotHistoryStat{}
}

func TestAnalyzeSlotHistoryAggregatesAvailabilityAndSoldOutSpeed(t *testing.T) {
	now := insightTestNow(t)
	snapshots := []SlotSnapshot{
		insightSnapshot("2026-05-01T10:00:00+08:00", "001", "20260515", "193000", "200000", "AVAILABLE"),
		insightSnapshot("2026-05-01T10:30:00+08:00", "001", "20260515", "193000", "200000", "AVAILABLE"),
		insightSnapshot("2026-05-01T11:00:00+08:00", "001", "20260515", "193000", "200000", "FULL"),
		insightSnapshot("2026-05-08T10:00:00+08:00", "001", "20260522", "193000", "200000", "FULL"),
		insightSnapshot("2026-05-01T10:00:00+08:00", "001", "bad-date", "193000", "200000", "AVAILABLE"),
	}

	analysis := AnalyzeSlotHistoryTopN(snapshots, now, 5)

	if analysis.TotalSnapshots != 5 || analysis.ValidSnapshots != 4 || analysis.SkippedSnapshots != 1 {
		t.Fatalf("snapshot counts = total %d valid %d skipped %d, want 5/4/1",
			analysis.TotalSnapshots, analysis.ValidSnapshots, analysis.SkippedSnapshots)
	}

	stat := findInsightStat(t, analysis, "001", 5, "193000")
	if stat.Observations != 4 || stat.AvailableObservations != 2 || stat.UnavailableObservations != 2 {
		t.Fatalf("observations = %d/%d/%d, want 4/2/2",
			stat.Observations, stat.AvailableObservations, stat.UnavailableObservations)
	}
	if math.Abs(stat.AvailabilityRate-0.5) > 0.0001 {
		t.Fatalf("availability rate = %.4f, want 0.5", stat.AvailabilityRate)
	}
	if stat.SoldOutMinutes == nil || math.Abs(*stat.SoldOutMinutes-60) > 0.0001 {
		t.Fatalf("sold out minutes = %v, want 60", stat.SoldOutMinutes)
	}
	if stat.SoldOutObservations != 1 {
		t.Fatalf("sold out observations = %d, want 1", stat.SoldOutObservations)
	}
}

func TestAnalyzeSlotHistorySoldOutSpeedIsConservative(t *testing.T) {
	now := insightTestNow(t)
	snapshots := []SlotSnapshot{
		insightSnapshot("2026-05-01T09:00:00+08:00", "001", "20260516", "103000", "110000", "FULL"),
		insightSnapshot("2026-05-01T10:00:00+08:00", "001", "20260516", "103000", "110000", "AVAILABLE"),
		insightSnapshot("2026-05-01T10:30:00+08:00", "001", "20260516", "103000", "110000", "AVAILABLE"),
		insightSnapshot("bad-ts", "001", "20260517", "103000", "110000", "AVAILABLE"),
		insightSnapshot("2026-05-01T11:00:00+08:00", "001", "20260517", "103000", "110000", "FULL"),
	}

	analysis := AnalyzeSlotHistoryTopN(snapshots, now, 5)

	saturday := findInsightStat(t, analysis, "001", 6, "103000")
	if saturday.SoldOutMinutes != nil {
		t.Fatalf("saturday sold out minutes = %v, want nil without later non-AVAILABLE", *saturday.SoldOutMinutes)
	}

	sunday := findInsightStat(t, analysis, "001", 7, "103000")
	if sunday.SoldOutMinutes != nil {
		t.Fatalf("sunday sold out minutes = %v, want nil when AVAILABLE timestamp is invalid", *sunday.SoldOutMinutes)
	}
}

func TestAnalyzeSlotHistoryTopNRecommendations(t *testing.T) {
	now := insightTestNow(t)
	snapshots := []SlotSnapshot{
		insightSnapshot("2026-05-01T10:00:00+08:00", "001", "20260515", "193000", "200000", "AVAILABLE"),
		insightSnapshot("2026-05-01T11:00:00+08:00", "001", "20260515", "193000", "200000", "FULL"),
		insightSnapshot("2026-05-08T10:00:00+08:00", "001", "20260522", "193000", "200000", "FULL"),
		insightSnapshot("2026-05-08T10:30:00+08:00", "001", "20260522", "193000", "200000", "AVAILABLE"),
		insightSnapshot("2026-05-02T10:00:00+08:00", "002", "20260516", "180000", "183000", "AVAILABLE"),
		insightSnapshot("2026-05-02T10:30:00+08:00", "002", "20260516", "180000", "183000", "AVAILABLE"),
		insightSnapshot("2026-05-09T10:00:00+08:00", "002", "20260523", "180000", "183000", "AVAILABLE"),
	}

	analysis := AnalyzeSlotHistoryTopN(snapshots, now, 1)

	if len(analysis.Recommendations) != 1 {
		t.Fatalf("recommendations len = %d, want 1", len(analysis.Recommendations))
	}
	got := analysis.Recommendations[0]
	if got.StoreID != "002" || got.Weekday != 6 || got.Start != "180000" || got.End != "183000" {
		t.Fatalf("top recommendation = %+v, want store 002 saturday 18:00", got)
	}
	if got.AvailabilityRate != 1 {
		t.Fatalf("top availability rate = %.4f, want 1", got.AvailabilityRate)
	}
	if got.Score <= 0 {
		t.Fatalf("top score = %.4f, want positive", got.Score)
	}

	if recs := TopSlotInsightRecommendations(analysis, 0); len(recs) != 0 {
		t.Fatalf("TopSlotInsightRecommendations top 0 len = %d, want 0", len(recs))
	}
}
