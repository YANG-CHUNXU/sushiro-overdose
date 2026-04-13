package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TimeRange represents a target time window for slot booking.
type TimeRange struct {
	Start string `json:"start"` // "1930" or "193000"
	End   string `json:"end"`   // "2030" or "203000"
}

// UserPreferences stores all user-configurable booking preferences.
// Persisted to ~/.sushiro/preferences.json and editable from the Web UI.
type UserPreferences struct {
	Adult          int         `json:"adult"`
	Child          int         `json:"child"`
	TableType      string      `json:"table_type"`
	SelectedStores []string    `json:"selected_stores"`
	WeekdaySlots   []TimeRange `json:"weekday_slots"`
	SaturdaySlots  []TimeRange `json:"saturday_slots"`
	SundaySlots    []TimeRange `json:"sunday_slots"`
}

func preferencesPath() string {
	return filepath.Join(appDirPath(), "preferences.json")
}

func DefaultPreferences() UserPreferences {
	return UserPreferences{
		Adult:         2,
		Child:         0,
		TableType:     "T",
		WeekdaySlots:  []TimeRange{{Start: "1930", End: "2030"}},
		SaturdaySlots: []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
		SundaySlots:   []TimeRange{{Start: "1030", End: "1300"}, {Start: "1930", End: "2030"}},
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
	if prefs.Adult <= 0 && prefs.Child <= 0 {
		prefs.Adult = 2
	}
	if prefs.TableType == "" {
		prefs.TableType = "T"
	}
	return prefs
}

func SavePreferences(prefs UserPreferences) error {
	os.MkdirAll(appDirPath(), 0o755)
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
	for _, r := range ranges {
		rangeStart := normalizeTimeStr(r.Start)
		rangeEnd := normalizeTimeStr(r.End)
		if start >= rangeStart && start < rangeEnd {
			return true
		}
	}
	return false
}

func normalizeTimeStr(t string) string {
	t = strings.TrimSpace(t)
	switch len(t) {
	case 4:
		return t + "00"
	case 6:
		return t
	default:
		return t
	}
}
