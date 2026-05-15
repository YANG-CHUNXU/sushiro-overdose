package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	ActiveReservation *ReservationRecord `json:"active_reservation,omitempty"`
	SavedAt           string             `json:"saved_at,omitempty"`
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

func logMessage(now time.Time, message string) {
	fmt.Printf("[%s] %s\n", now.Format(time.RFC3339), message)
}

// stdinReader is a shared buffered reader for stdin to avoid losing data.
var stdinReader = bufio.NewReader(os.Stdin)

// readInput reads a trimmed line from stdin.
func readInput() string {
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(line)
}
