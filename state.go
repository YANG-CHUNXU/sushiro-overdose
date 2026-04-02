package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	ActiveReservation      *ReservationRecord `json:"active_reservation,omitempty"`
	NotificationSent       bool               `json:"notification_sent,omitempty"`
	SavedAt                string             `json:"saved_at,omitempty"`
	NotifiedAt             string             `json:"notified_at,omitempty"`
	LastWeekendSummaryHour string             `json:"last_weekend_summary_hour,omitempty"`
	LastWeekendSummaryAt   string             `json:"last_weekend_summary_at,omitempty"`
}

func (s State) IsZero() bool {
	return s.ActiveReservation == nil &&
		!s.NotificationSent &&
		s.SavedAt == "" &&
		s.NotifiedAt == "" &&
		s.LastWeekendSummaryHour == "" &&
		s.LastWeekendSummaryAt == ""
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("invalid JSON in state file %s: %w", path, err)
	}
	return state, nil
}

func saveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func clearState(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}
	return nil
}

func pruneExpiredReservation(state *State, today time.Time) bool {
	if state.ActiveReservation == nil {
		return false
	}
	if activeReservation(*state, today) != nil {
		return false
	}
	state.ActiveReservation = nil
	state.NotificationSent = false
	state.SavedAt = ""
	state.NotifiedAt = ""
	return true
}

func activeReservation(state State, today time.Time) *ReservationRecord {
	if state.ActiveReservation == nil || strings.TrimSpace(state.ActiveReservation.QueueDate) == "" {
		return nil
	}
	reservationDay, err := parseCompactDate(state.ActiveReservation.QueueDate, today.Location())
	if err != nil {
		return nil
	}
	if reservationDay.Before(beginningOfDay(today)) {
		return nil
	}
	return state.ActiveReservation
}

func currentSummaryHourKey(now time.Time) string {
	return now.Format("2006-01-02T15")
}

func currentMinuteKey(now time.Time) string {
	return now.Format("2006-01-02T15:04")
}

func shouldSendWeekendSummary(state State, now time.Time) bool {
	return state.LastWeekendSummaryHour != currentSummaryHourKey(now)
}

func logMessage(now time.Time, message string) {
	fmt.Printf("[%s] %s\n", now.Format(time.RFC3339), message)
}
