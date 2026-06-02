package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	mainActivityFile     = "main_active.json"
	samplingLockFileName = "sampling.lock"
)

type activityMarker struct {
	PID    int    `json:"pid"`
	Status string `json:"status"`
	SetAt  string `json:"set_at"`
	Token  string `json:"token"`
}

type processLock struct {
	path string
	pid  int
}

func mainActivityPath() string {
	return filepath.Join(appDirPath(), mainActivityFile)
}

func markMainFlowActive(status string) func() {
	_ = os.MkdirAll(appDirPath(), 0o755)
	if current, err := readActivityMarker(mainActivityPath()); err == nil && current.PID != os.Getpid() {
		if IsProcessAlive(current.PID) {
			return func() {}
		}
		_ = os.Remove(mainActivityPath())
	}
	marker := activityMarker{
		PID:    os.Getpid(),
		Status: status,
		SetAt:  time.Now().Format(time.RFC3339),
		Token:  newActivityToken(),
	}
	data, _ := json.MarshalIndent(marker, "", "  ")
	_ = os.WriteFile(mainActivityPath(), data, 0o600)
	pauseSamplingForMainFlow()
	return func() {
		current, err := readActivityMarker(mainActivityPath())
		if err == nil && current.PID == os.Getpid() && current.Token == marker.Token {
			_ = os.Remove(mainActivityPath())
		}
	}
}

func newActivityToken() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func externalMainFlowActive() (bool, string) {
	marker, err := readActivityMarker(mainActivityPath())
	if err != nil {
		return false, ""
	}
	if marker.PID == os.Getpid() {
		return false, ""
	}
	if IsProcessAlive(marker.PID) {
		return true, marker.Status
	}
	_ = os.Remove(mainActivityPath())
	return false, ""
}

func readActivityMarker(path string) (activityMarker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return activityMarker{}, err
	}
	var marker activityMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return activityMarker{}, err
	}
	return marker, nil
}

func acquireProcessLock(name string) (*processLock, error) {
	path := filepath.Join(appDirPath(), name)
	_ = os.MkdirAll(appDirPath(), 0o755)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			pid := os.Getpid()
			_, _ = fmt.Fprintf(f, "%d\n", pid)
			_ = f.Close()
			return &processLock{path: path, pid: pid}, nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, err
		}
		pid := atoi(string(data))
		if pid <= 0 || !IsProcessAlive(pid) {
			_ = os.Remove(path)
			continue
		}
		return nil, fmt.Errorf("已有采样进程正在运行 (PID %d)", pid)
	}
	return nil, fmt.Errorf("采样锁被占用")
}

func processLockHolder(name string) (int, bool) {
	path := filepath.Join(appDirPath(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid := atoi(string(data))
	if pid <= 0 || !IsProcessAlive(pid) {
		_ = os.Remove(path)
		return 0, false
	}
	return pid, true
}

func (l *processLock) Release() {
	if l == nil {
		return
	}
	data, err := os.ReadFile(l.path)
	if err == nil && atoi(string(data)) == l.pid {
		_ = os.Remove(l.path)
	}
}
