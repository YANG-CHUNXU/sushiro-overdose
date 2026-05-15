package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dayPriorityDate         = "date"
	dayPriorityWeekendFirst = "weekend_first"
	dayPriorityWeekdayFirst = "weekday_first"
	dayPriorityCustom       = "custom"

	dayKindWeekday  = "weekday"
	dayKindSaturday = "saturday"
	dayKindSunday   = "sunday"

	slotStrategyEarliest = "earliest"
	slotStrategyLatest   = "latest"
	slotStrategyClosest  = "closest"

	defaultTargetTime = "1930"
)

// TimeRange represents a target time window for slot booking.
type TimeRange struct {
	Start string `json:"start"` // "1930" or "193000"
	End   string `json:"end"`   // "2030" or "203000"
}

// UserPreferences stores all user-configurable booking preferences.
// Persisted to ~/.sushiro/preferences.json and editable from the Web UI.
type UserPreferences struct {
	Adult           int         `json:"adult"`
	Child           int         `json:"child"`
	TableType       string      `json:"table_type"`
	SelectedStores  []string    `json:"selected_stores"`
	StorePriority   []string    `json:"store_priority"`
	DayPriorityMode string      `json:"day_priority_mode"`
	DayPriority     []string    `json:"day_priority"`
	SlotStrategy    string      `json:"slot_strategy"`
	TargetTime      string      `json:"target_time"`
	WeekdaySlots    []TimeRange `json:"weekday_slots"`
	SaturdaySlots   []TimeRange `json:"saturday_slots"`
	SundaySlots     []TimeRange `json:"sunday_slots"`
}

func preferencesPath() string {
	return filepath.Join(appDirPath(), "preferences.json")
}

func DefaultPreferences() UserPreferences {
	return UserPreferences{
		Adult:           2,
		Child:           0,
		TableType:       "T",
		DayPriorityMode: dayPriorityDate,
		DayPriority:     []string{dayKindSaturday, dayKindSunday, dayKindWeekday},
		SlotStrategy:    slotStrategyEarliest,
		TargetTime:      defaultTargetTime,
		WeekdaySlots:    []TimeRange{{Start: "1930", End: "2030"}},
		SaturdaySlots:   []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
		SundaySlots:     []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
	}
}

func LoadPreferences() UserPreferences {
	data, err := os.ReadFile(preferencesPath())
	if err != nil {
		return DefaultPreferences()
	}
	var prefs UserPreferences
	if json.Unmarshal(data, &prefs) != nil {
		return DefaultPreferences()
	}
	return NormalizePreferences(prefs)
}

func NormalizePreferences(prefs UserPreferences) UserPreferences {
	if prefs.Adult <= 0 && prefs.Child <= 0 {
		prefs.Adult = 2
	}
	if prefs.TableType == "" {
		prefs.TableType = "T"
	}
	if !validDayPriorityMode(prefs.DayPriorityMode) {
		prefs.DayPriorityMode = dayPriorityDate
	}
	prefs.DayPriority = normalizeDayPriority(prefs.DayPriority)
	if !validSlotStrategy(prefs.SlotStrategy) {
		prefs.SlotStrategy = slotStrategyEarliest
	}
	if parseTimeSeconds(prefs.TargetTime) < 0 {
		prefs.TargetTime = defaultTargetTime
	}
	prefs.SelectedStores = uniqueNonEmptyStrings(prefs.SelectedStores)
	prefs.StorePriority = normalizeStorePriority(prefs.StorePriority, prefs.SelectedStores)
	return prefs
}

func SavePreferences(prefs UserPreferences) error {
	os.MkdirAll(appDirPath(), 0o755)
	prefs = NormalizePreferences(prefs)
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(preferencesPath(), data, 0o600)
}

func (p UserPreferences) ShouldTarget(slot Slot, loc *time.Location) bool {
	day, err := parseCompactDate(slot.Date, loc)
	if err != nil {
		return false
	}

	var ranges []TimeRange
	switch day.Weekday() {
	case time.Saturday:
		ranges = p.SaturdaySlots
	case time.Sunday:
		ranges = p.SundaySlots
	default:
		ranges = p.WeekdaySlots
	}

	if len(ranges) == 0 {
		return false
	}

	start := normalizeTimeStr(slot.Start)
	end := normalizeTimeStr(slot.End)
	if end == "" {
		end = start
	}
	for _, r := range ranges {
		rangeStart := normalizeTimeStr(r.Start)
		rangeEnd := normalizeTimeStr(r.End)
		if rangeStart == "" || rangeEnd == "" {
			continue
		}
		if start >= rangeStart && start < rangeEnd && end <= rangeEnd {
			return true
		}
	}
	return false
}

func (p UserPreferences) PreferTargetSlot(candidate, current TargetSlot, loc *time.Location, storeOrder []string) bool {
	p = NormalizePreferences(p)

	candidateDate, candidateDateErr := parseCompactDate(candidate.Date, loc)
	currentDate, currentDateErr := parseCompactDate(current.Date, loc)
	if candidateDateErr != nil || currentDateErr != nil {
		return candidate.Date+candidate.Start+candidate.StoreID < current.Date+current.Start+current.StoreID
	}

	if p.DayPriorityMode != dayPriorityDate {
		candidateRank := p.dayPriorityRank(candidateDate.Weekday())
		currentRank := p.dayPriorityRank(currentDate.Weekday())
		if candidateRank != currentRank {
			return candidateRank < currentRank
		}
	}

	if !sameDate(candidateDate, currentDate) {
		return candidateDate.Before(currentDate)
	}

	if cmp := compareSlotStart(candidate.Start, current.Start, p.SlotStrategy, p.TargetTime); cmp != 0 {
		return cmp < 0
	}

	candidateStoreRank := storePriorityRank(candidate.StoreID, p, storeOrder)
	currentStoreRank := storePriorityRank(current.StoreID, p, storeOrder)
	if candidateStoreRank != currentStoreRank {
		return candidateStoreRank < currentStoreRank
	}

	if candidate.End != current.End {
		return candidate.End < current.End
	}
	return candidate.StoreID < current.StoreID
}

func (p UserPreferences) dayPriorityRank(day time.Weekday) int {
	switch p.DayPriorityMode {
	case dayPriorityWeekendFirst:
		if day == time.Saturday || day == time.Sunday {
			return 0
		}
		return 1
	case dayPriorityWeekdayFirst:
		if day == time.Saturday || day == time.Sunday {
			return 1
		}
		return 0
	case dayPriorityCustom:
		kind := dayKind(day)
		for i, preferred := range p.DayPriority {
			if preferred == kind {
				return i
			}
		}
	}
	return 0
}

func normalizeTimeStr(t string) string {
	t = strings.TrimSpace(t)
	t = strings.ReplaceAll(t, ":", "")
	switch len(t) {
	case 4:
		return t + "00"
	case 6:
		return t
	default:
		return t
	}
}

func validDayPriorityMode(mode string) bool {
	switch mode {
	case dayPriorityDate, dayPriorityWeekendFirst, dayPriorityWeekdayFirst, dayPriorityCustom:
		return true
	default:
		return false
	}
}

func validSlotStrategy(strategy string) bool {
	switch strategy {
	case slotStrategyEarliest, slotStrategyLatest, slotStrategyClosest:
		return true
	default:
		return false
	}
}

func normalizeDayPriority(priority []string) []string {
	defaults := []string{dayKindSaturday, dayKindSunday, dayKindWeekday}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(defaults))
	for _, item := range priority {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] || !validDayKind(item) {
			continue
		}
		seen[item] = true
		normalized = append(normalized, item)
	}
	for _, item := range defaults {
		if !seen[item] {
			normalized = append(normalized, item)
		}
	}
	return normalized
}

func normalizeStorePriority(priority, selected []string) []string {
	base := uniqueNonEmptyStrings(priority)
	if len(selected) == 0 {
		return base
	}

	selectedSet := make(map[string]bool, len(selected))
	for _, storeID := range selected {
		selectedSet[storeID] = true
	}

	normalized := make([]string, 0, len(selected))
	seen := map[string]bool{}
	for _, storeID := range base {
		if selectedSet[storeID] {
			normalized = append(normalized, storeID)
			seen[storeID] = true
		}
	}
	for _, storeID := range selected {
		if !seen[storeID] {
			normalized = append(normalized, storeID)
		}
	}
	return normalized
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		normalized = append(normalized, item)
	}
	return normalized
}

func validDayKind(kind string) bool {
	return kind == dayKindWeekday || kind == dayKindSaturday || kind == dayKindSunday
}

func dayKind(day time.Weekday) string {
	switch day {
	case time.Saturday:
		return dayKindSaturday
	case time.Sunday:
		return dayKindSunday
	default:
		return dayKindWeekday
	}
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func storePriorityRank(storeID string, prefs UserPreferences, fallbackOrder []string) int {
	order := prefs.StorePriority
	if len(order) == 0 {
		order = prefs.SelectedStores
	}
	rank := rankInOrder(storeID, order)
	if rank >= 0 {
		return rank
	}
	rank = rankInOrder(storeID, fallbackOrder)
	if rank >= 0 {
		return len(order) + rank
	}
	return len(order) + len(fallbackOrder) + 1
}

func rankInOrder(value string, order []string) int {
	for i, item := range order {
		if item == value {
			return i
		}
	}
	return -1
}

func compareSlotStart(candidateStart, currentStart, strategy, targetTime string) int {
	candidateSeconds := parseTimeSeconds(candidateStart)
	currentSeconds := parseTimeSeconds(currentStart)
	if candidateSeconds < 0 || currentSeconds < 0 {
		return strings.Compare(candidateStart, currentStart)
	}

	switch strategy {
	case slotStrategyLatest:
		return currentSeconds - candidateSeconds
	case slotStrategyClosest:
		targetSeconds := parseTimeSeconds(targetTime)
		if targetSeconds < 0 {
			targetSeconds = parseTimeSeconds(defaultTargetTime)
		}
		candidateDiff := absInt(candidateSeconds - targetSeconds)
		currentDiff := absInt(currentSeconds - targetSeconds)
		if candidateDiff != currentDiff {
			return candidateDiff - currentDiff
		}
		return candidateSeconds - currentSeconds
	default:
		return candidateSeconds - currentSeconds
	}
}

func parseTimeSeconds(value string) int {
	normalized := normalizeTimeStr(value)
	if len(normalized) != 6 {
		return -1
	}
	hour, minute, second, err := parseCompactTime(normalized)
	if err != nil {
		return -1
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return -1
	}
	return hour*3600 + minute*60 + second
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
