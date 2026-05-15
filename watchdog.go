package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const proxyStateFile = "proxy_active.json"

func proxyStatePath() string {
	return fmt.Sprintf("%s/%s", appDirPath(), proxyStateFile)
}

type proxyState struct {
	Active bool      `json:"active"`
	Port   int       `json:"port"`
	SetAt  time.Time `json:"set_at"`
	PID    int       `json:"pid"`
}

// markProxyActive records that the system proxy is currently set.
func markProxyActive(port, pid int) {
	state := proxyState{
		Active: true,
		Port:   port,
		SetAt:  time.Now(),
		PID:    pid,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.MkdirAll(appDirPath(), 0o755)
	_ = os.WriteFile(proxyStatePath(), data, 0o644)
}

// markProxyInactive clears the proxy state marker.
func markProxyInactive() {
	os.Remove(proxyStatePath())
}

// checkStaleProxy checks for leftover proxy state and cleans it up.
// Returns true if a stale proxy was found and cleaned.
func checkStaleProxy() bool {
	data, err := os.ReadFile(proxyStatePath())
	if err != nil {
		return false
	}

	var state proxyState
	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}

	if !state.Active {
		return false
	}

	// Check if the process that set the proxy is still alive
	if state.PID > 0 && IsProcessAlive(state.PID) {
		// The process is still running, don't interfere
		return false
	}

	// Stale proxy detected — clean up
	logMessage(time.Now(), fmt.Sprintf("检测到残留代理设置 (PID %d 已退出)，正在清除...", state.PID))
	ClearSystemProxy()
	markProxyInactive()
	return true
}
