package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DayPriorityDate         = "date"
	DayPriorityWeekendFirst = "weekend_first"
	DayPriorityWeekdayFirst = "weekday_first"
	DayPriorityCustom       = "custom"

	dayKindWeekday  = "weekday"
	dayKindSaturday = "saturday"
	dayKindSunday   = "sunday"

	SlotStrategyEarliest = "earliest"
	SlotStrategyLatest   = "latest"
	SlotStrategyClosest  = "closest"

	DefaultTargetTime = "1930"
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
	PhoneNumber     string      `json:"phone_number"`
	WechatID        string      `json:"wechat_id"`
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

func PreferencesPath() string {
	return filepath.Join(AppDirPath(), "preferences.json")
}

func DefaultPreferences() UserPreferences {
	return UserPreferences{
		Adult:           2,
		Child:           0,
		TableType:       "T",
		DayPriorityMode: DayPriorityDate,
		DayPriority:     []string{dayKindSaturday, dayKindSunday, dayKindWeekday},
		SlotStrategy:    SlotStrategyEarliest,
		TargetTime:      DefaultTargetTime,
		WeekdaySlots:    []TimeRange{{Start: "1930", End: "2030"}},
		SaturdaySlots:   []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
		SundaySlots:     []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
	}
}

func LoadPreferences() UserPreferences {
	data, err := os.ReadFile(PreferencesPath())
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
		prefs.DayPriorityMode = DayPriorityDate
	}
	prefs.DayPriority = normalizeDayPriority(prefs.DayPriority)
	if !validSlotStrategy(prefs.SlotStrategy) {
		prefs.SlotStrategy = SlotStrategyEarliest
	}
	if ParseTimeSeconds(prefs.TargetTime) < 0 {
		prefs.TargetTime = DefaultTargetTime
	}
	prefs.PhoneNumber = NormalizePreferencePhoneNumber(prefs.PhoneNumber)
	prefs.SelectedStores = UniqueNonEmptyStrings(prefs.SelectedStores)
	prefs.StorePriority = normalizeStorePriority(prefs.StorePriority, prefs.SelectedStores)
	return prefs
}

func NormalizePreferencePhoneNumber(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if len(digits) < 8 {
		return ""
	}
	return digits
}

func SavePreferences(prefs UserPreferences) error {
	os.MkdirAll(AppDirPath(), 0o755)
	prefs = NormalizePreferences(prefs)
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(PreferencesPath(), data, 0o600)
}

func (p UserPreferences) ShouldTarget(slot Slot, loc *time.Location) bool {
	day, err := ParseCompactDate(slot.Date, loc)
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

	start := NormalizeTimeStr(slot.Start)
	end := NormalizeTimeStr(slot.End)
	if end == "" {
		end = start
	}
	for _, r := range ranges {
		rangeStart := NormalizeTimeStr(r.Start)
		rangeEnd := NormalizeTimeStr(r.End)
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

	candidateDate, candidateDateErr := ParseCompactDate(candidate.Date, loc)
	currentDate, currentDateErr := ParseCompactDate(current.Date, loc)
	if candidateDateErr != nil || currentDateErr != nil {
		return candidate.Date+candidate.Start+candidate.StoreID < current.Date+current.Start+current.StoreID
	}

	if p.DayPriorityMode != DayPriorityDate {
		candidateRank := p.DayPriorityRank(candidateDate.Weekday())
		currentRank := p.DayPriorityRank(currentDate.Weekday())
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

func (p UserPreferences) DayPriorityRank(day time.Weekday) int {
	switch p.DayPriorityMode {
	case DayPriorityWeekendFirst:
		if day == time.Saturday || day == time.Sunday {
			return 0
		}
		return 1
	case DayPriorityWeekdayFirst:
		if day == time.Saturday || day == time.Sunday {
			return 1
		}
		return 0
	case DayPriorityCustom:
		kind := dayKind(day)
		for i, preferred := range p.DayPriority {
			if preferred == kind {
				return i
			}
		}
	}
	return 0
}

func NormalizeTimeStr(t string) string {
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
	case DayPriorityDate, DayPriorityWeekendFirst, DayPriorityWeekdayFirst, DayPriorityCustom:
		return true
	default:
		return false
	}
}

func validSlotStrategy(strategy string) bool {
	switch strategy {
	case SlotStrategyEarliest, SlotStrategyLatest, SlotStrategyClosest:
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
	base := UniqueNonEmptyStrings(priority)
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

func UniqueNonEmptyStrings(items []string) []string {
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
	candidateSeconds := ParseTimeSeconds(candidateStart)
	currentSeconds := ParseTimeSeconds(currentStart)
	if candidateSeconds < 0 || currentSeconds < 0 {
		return strings.Compare(candidateStart, currentStart)
	}

	switch strategy {
	case SlotStrategyLatest:
		return currentSeconds - candidateSeconds
	case SlotStrategyClosest:
		targetSeconds := ParseTimeSeconds(targetTime)
		if targetSeconds < 0 {
			targetSeconds = ParseTimeSeconds(DefaultTargetTime)
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

func ParseTimeSeconds(value string) int {
	normalized := NormalizeTimeStr(value)
	if len(normalized) != 6 {
		return -1
	}
	hour, minute, second, err := ParseCompactTime(normalized)
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
