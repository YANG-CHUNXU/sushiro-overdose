package core

import (
	"path/filepath"
	"sync"
)

// ---- 活动 Web 端口（供守护/平台层读取，构造本地 PAC 等）----

var (
	webPortMu sync.RWMutex
	webPort   int
)

func SetActiveWebPort(port int) {
	webPortMu.Lock()
	webPort = port
	webPortMu.Unlock()
}

func GetActiveWebPort() int {
	webPortMu.RLock()
	defer webPortMu.RUnlock()
	return webPort
}

// ---- 采样日志路径 ----

const samplingLogFile = "sampling.log"

func SamplingLogPath() string { return filepath.Join(AppDirPath(), samplingLogFile) }

// ---- 维护/清理结果（platform 与 maintenance 共用）----

const (
	MaintenanceStatusOK          = "ok"
	MaintenanceStatusMissing     = "missing"
	MaintenanceStatusError       = "error"
	MaintenanceStatusSkipped     = "skipped"
	MaintenanceStatusWouldRemove = "would_remove"
)

type MaintenanceResult struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
