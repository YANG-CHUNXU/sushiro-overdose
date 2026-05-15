package main

import (
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

const defaultInsightTopN = 10

type SlotHistoryAnalysis struct {
	GeneratedAt      string                      `json:"generated_at"`
	TotalSnapshots   int                         `json:"total_snapshots"`
	ValidSnapshots   int                         `json:"valid_snapshots"`
	SkippedSnapshots int                         `json:"skipped_snapshots"`
	Stores           []StoreHistoryInsight       `json:"stores"`
	Recommendations  []SlotInsightRecommendation `json:"recommendations"`
}

type StoreHistoryInsight struct {
	StoreID  string                  `json:"store_id"`
	Weekdays []WeekdayHistoryInsight `json:"weekdays"`
}

type WeekdayHistoryInsight struct {
	Weekday     int               `json:"weekday"`
	WeekdayName string            `json:"weekday_name"`
	Slots       []SlotHistoryStat `json:"slots"`
}

type SlotHistoryStat struct {
	StoreID                 string   `json:"store_id"`
	Weekday                 int      `json:"weekday"`
	WeekdayName             string   `json:"weekday_name"`
	Start                   string   `json:"start"`
	End                     string   `json:"end"`
	Observations            int      `json:"observations"`
	AvailableObservations   int      `json:"available_observations"`
	UnavailableObservations int      `json:"unavailable_observations"`
	AvailabilityRate        float64  `json:"availability_rate"`
	SoldOutMinutes          *float64 `json:"sold_out_minutes"`
	SoldOutObservations     int      `json:"sold_out_observations"`
	LastObservedAt          string   `json:"last_observed_at,omitempty"`
}

type SlotInsightRecommendation struct {
	StoreID          string   `json:"store_id"`
	Weekday          int      `json:"weekday"`
	WeekdayName      string   `json:"weekday_name"`
	Start            string   `json:"start"`
	End              string   `json:"end"`
	AvailabilityRate float64  `json:"availability_rate"`
	SoldOutMinutes   *float64 `json:"sold_out_minutes"`
	Score            float64  `json:"score"`
	Observations     int      `json:"observations"`
}

type insightStatKey struct {
	storeID string
	weekday int
	start   string
	end     string
}

type insightTransitionKey struct {
	storeID string
	date    string
	start   string
}

type insightStatAccumulator struct {
	stat                SlotHistoryStat
	lastObservedAt      time.Time
	soldOutTotalMinutes float64
}

type insightObservation struct {
	at           time.Time
	hasTime      bool
	storeID      string
	date         string
	weekday      int
	start        string
	end          string
	availability string
}

// LoadSlotHistoryInsights reads ~/.sushiro/history.jsonl and returns topN recommendations.
func LoadSlotHistoryInsights(topN int, now time.Time) (SlotHistoryAnalysis, error) {
	snapshots, err := loadHistory()
	if err != nil {
		if os.IsNotExist(err) {
			return AnalyzeSlotHistoryTopN(nil, now, topN), nil
		}
		return SlotHistoryAnalysis{}, err
	}
	return AnalyzeSlotHistoryTopN(snapshots, now, topN), nil
}

// AnalyzeSlotHistory aggregates slot history and returns up to the default top recommendations.
func AnalyzeSlotHistory(snapshots []SlotSnapshot, now time.Time) SlotHistoryAnalysis {
	return AnalyzeSlotHistoryTopN(snapshots, now, defaultInsightTopN)
}

// AnalyzeSlotHistoryTopN aggregates slot history and returns at most topN recommendations.
func AnalyzeSlotHistoryTopN(snapshots []SlotSnapshot, now time.Time, topN int) SlotHistoryAnalysis {
	if now.IsZero() {
		now = time.Now()
	}

	stats := map[insightStatKey]*insightStatAccumulator{}
	transitions := map[insightTransitionKey][]insightObservation{}
	analysis := SlotHistoryAnalysis{
		GeneratedAt:    now.Format(time.RFC3339),
		TotalSnapshots: len(snapshots),
	}

	for _, snapshot := range snapshots {
		obs, ok := normalizeInsightObservation(snapshot, now.Location())
		if !ok {
			analysis.SkippedSnapshots++
			continue
		}
		analysis.ValidSnapshots++

		key := insightStatKey{
			storeID: obs.storeID,
			weekday: obs.weekday,
			start:   obs.start,
			end:     obs.end,
		}
		acc := stats[key]
		if acc == nil {
			acc = &insightStatAccumulator{
				stat: SlotHistoryStat{
					StoreID:     obs.storeID,
					Weekday:     obs.weekday,
					WeekdayName: insightWeekdayName(obs.weekday),
					Start:       obs.start,
					End:         obs.end,
				},
			}
			stats[key] = acc
		}
		acc.stat.Observations++
		if obs.availability == "AVAILABLE" {
			acc.stat.AvailableObservations++
		} else {
			acc.stat.UnavailableObservations++
		}
		if obs.hasTime && (acc.lastObservedAt.IsZero() || obs.at.After(acc.lastObservedAt)) {
			acc.lastObservedAt = obs.at
		}

		transitionKey := insightTransitionKey{storeID: obs.storeID, date: obs.date, start: obs.start}
		transitions[transitionKey] = append(transitions[transitionKey], obs)
	}

	applySoldOutEstimates(stats, transitions)
	finalStats := finalizeInsightStats(stats)
	analysis.Stores = buildStoreHistoryInsights(finalStats)
	analysis.Recommendations = TopSlotInsightRecommendations(analysis, topN)
	return analysis
}

// TopSlotInsightRecommendations returns at most topN recommendations from an analysis.
func TopSlotInsightRecommendations(analysis SlotHistoryAnalysis, topN int) []SlotInsightRecommendation {
	if topN <= 0 {
		return []SlotInsightRecommendation{}
	}
	stats := flattenSlotHistoryStats(analysis.Stores)
	recommendations := make([]SlotInsightRecommendation, 0, len(stats))
	for _, stat := range stats {
		if stat.Observations == 0 {
			continue
		}
		recommendations = append(recommendations, SlotInsightRecommendation{
			StoreID:          stat.StoreID,
			Weekday:          stat.Weekday,
			WeekdayName:      stat.WeekdayName,
			Start:            stat.Start,
			End:              stat.End,
			AvailabilityRate: stat.AvailabilityRate,
			SoldOutMinutes:   cloneFloat64Ptr(stat.SoldOutMinutes),
			Score:            scoreSlotInsight(stat),
			Observations:     stat.Observations,
		})
	}
	sort.Slice(recommendations, func(i, j int) bool {
		left, right := recommendations[i], recommendations[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.AvailabilityRate != right.AvailabilityRate {
			return left.AvailabilityRate > right.AvailabilityRate
		}
		if cmp := compareNullableFloatDesc(left.SoldOutMinutes, right.SoldOutMinutes); cmp != 0 {
			return cmp < 0
		}
		if left.Observations != right.Observations {
			return left.Observations > right.Observations
		}
		if left.StoreID != right.StoreID {
			return left.StoreID < right.StoreID
		}
		if left.Weekday != right.Weekday {
			return left.Weekday < right.Weekday
		}
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		return left.End < right.End
	})
	if len(recommendations) > topN {
		recommendations = recommendations[:topN]
	}
	return recommendations
}

func normalizeInsightObservation(snapshot SlotSnapshot, loc *time.Location) (insightObservation, bool) {
	storeID := strings.TrimSpace(snapshot.StoreID)
	if storeID == "" {
		return insightObservation{}, false
	}

	day, err := parseCompactDate(strings.TrimSpace(snapshot.Date), loc)
	if err != nil {
		return insightObservation{}, false
	}

	start := normalizeTimeStr(snapshot.Start)
	if parseTimeSeconds(start) < 0 {
		return insightObservation{}, false
	}
	end := normalizeTimeStr(snapshot.End)
	if end == "" {
		end = start
	}
	if parseTimeSeconds(end) < 0 {
		return insightObservation{}, false
	}

	availability := strings.ToUpper(strings.TrimSpace(snapshot.Availability))
	if availability == "" {
		return insightObservation{}, false
	}

	at, hasTime := parseInsightTimestamp(snapshot.Timestamp)
	return insightObservation{
		at:           at,
		hasTime:      hasTime,
		storeID:      storeID,
		date:         strings.TrimSpace(snapshot.Date),
		weekday:      isoWeekday(day.Weekday()),
		start:        start,
		end:          end,
		availability: availability,
	}, true
}

func applySoldOutEstimates(stats map[insightStatKey]*insightStatAccumulator, transitions map[insightTransitionKey][]insightObservation) {
	for _, observations := range transitions {
		sort.Slice(observations, func(i, j int) bool {
			if observations[i].hasTime != observations[j].hasTime {
				return observations[i].hasTime
			}
			return observations[i].at.Before(observations[j].at)
		})

		var firstAvailable *insightObservation
		for i := range observations {
			obs := observations[i]
			if !obs.hasTime {
				continue
			}
			if firstAvailable == nil {
				if obs.availability == "AVAILABLE" {
					firstAvailable = &obs
				}
				continue
			}
			if obs.availability == "AVAILABLE" {
				continue
			}
			if obs.at.Before(firstAvailable.at) {
				continue
			}
			minutes := obs.at.Sub(firstAvailable.at).Minutes()
			if minutes < 0 {
				continue
			}
			key := insightStatKey{
				storeID: firstAvailable.storeID,
				weekday: firstAvailable.weekday,
				start:   firstAvailable.start,
				end:     firstAvailable.end,
			}
			if acc := stats[key]; acc != nil {
				acc.soldOutTotalMinutes += minutes
				acc.stat.SoldOutObservations++
			}
			break
		}
	}
}

func finalizeInsightStats(stats map[insightStatKey]*insightStatAccumulator) []SlotHistoryStat {
	out := make([]SlotHistoryStat, 0, len(stats))
	for _, acc := range stats {
		stat := acc.stat
		if stat.Observations > 0 {
			stat.AvailabilityRate = roundFloat(float64(stat.AvailableObservations)/float64(stat.Observations), 4)
		}
		if stat.SoldOutObservations > 0 {
			value := roundFloat(acc.soldOutTotalMinutes/float64(stat.SoldOutObservations), 2)
			stat.SoldOutMinutes = &value
		}
		if !acc.lastObservedAt.IsZero() {
			stat.LastObservedAt = acc.lastObservedAt.Format(time.RFC3339)
		}
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StoreID != out[j].StoreID {
			return out[i].StoreID < out[j].StoreID
		}
		if out[i].Weekday != out[j].Weekday {
			return out[i].Weekday < out[j].Weekday
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].End < out[j].End
	})
	return out
}

func buildStoreHistoryInsights(stats []SlotHistoryStat) []StoreHistoryInsight {
	storeIndex := map[string]int{}
	stores := []StoreHistoryInsight{}
	for _, stat := range stats {
		storePos, ok := storeIndex[stat.StoreID]
		if !ok {
			storePos = len(stores)
			storeIndex[stat.StoreID] = storePos
			stores = append(stores, StoreHistoryInsight{StoreID: stat.StoreID, Weekdays: []WeekdayHistoryInsight{}})
		}

		weekdays := stores[storePos].Weekdays
		weekdayPos := -1
		for i := range weekdays {
			if weekdays[i].Weekday == stat.Weekday {
				weekdayPos = i
				break
			}
		}
		if weekdayPos < 0 {
			weekdayPos = len(weekdays)
			weekdays = append(weekdays, WeekdayHistoryInsight{
				Weekday:     stat.Weekday,
				WeekdayName: stat.WeekdayName,
				Slots:       []SlotHistoryStat{},
			})
		}
		weekdays[weekdayPos].Slots = append(weekdays[weekdayPos].Slots, stat)
		stores[storePos].Weekdays = weekdays
	}
	return stores
}

func flattenSlotHistoryStats(stores []StoreHistoryInsight) []SlotHistoryStat {
	var stats []SlotHistoryStat
	for _, store := range stores {
		for _, weekday := range store.Weekdays {
			stats = append(stats, weekday.Slots...)
		}
	}
	return stats
}

func scoreSlotInsight(stat SlotHistoryStat) float64 {
	soldOutFactor := 0.5
	if stat.SoldOutMinutes != nil {
		soldOutFactor = clampFloat(*stat.SoldOutMinutes/120, 0, 1)
	}
	confidence := clampFloat(float64(stat.Observations)/20, 0, 1)
	score := stat.AvailabilityRate*0.7 + soldOutFactor*0.2 + confidence*0.1
	return roundFloat(score, 4)
}

func compareNullableFloatDesc(left, right *float64) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	if *left > *right {
		return -1
	}
	if *left < *right {
		return 1
	}
	return 0
}

func parseInsightTimestamp(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts, true
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func isoWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func insightWeekdayName(weekday int) string {
	if weekday < 1 || weekday > 7 {
		return ""
	}
	return chineseWeekdayNames[weekday-1]
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func roundFloat(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
