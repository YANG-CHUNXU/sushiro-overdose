// Package platform 是平台适配层（facade）：把 OS 相关能力（系统代理、证书信任、通知、
// 进程管理、开机自启、浏览器拉起）封装成统一接口，内部按 build tag 分发到各平台实现
// （如 platform_desktop_darwin.go / platform_windows.go）。
// 跨平台业务代码只依赖这里导出的函数，不直接接触 syscall 或平台 API。
// 注意：本文件用 dot import 引入 core 包，是为了直接复用 core.MaintenanceResult 等类型
// 而不必每次写 core. 前缀。
package platform

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"syscall"
)

// DesktopNotification sends an OS-level notification to the user.
func DesktopNotification(title, message string) {
	desktopNotification(title, message)
}

// SetSystemProxy configures the system HTTP/HTTPS proxy to localhost:port.
func SetSystemProxy(port int) error {
	return setSystemProxy(port)
}

// ClearSystemProxy removes the system proxy configuration.
func ClearSystemProxy() error {
	return clearSystemProxy()
}

// IsCertTrusted checks if the CA certificate at certPath is trusted by the OS.
func IsCertTrusted() (bool, error) {
	return isCertTrusted()
}

// InstallCert installs the CA certificate into the OS trust store.
func InstallCert() error {
	return installCert()
}

// UninstallCert removes the CA certificate from the OS trust store when supported.
func UninstallCert() error {
	return uninstallCert()
}

// DaemonProcessAttrs returns syscall.SysProcAttr appropriate for spawning a daemon.
func DaemonProcessAttrs() *syscall.SysProcAttr {
	return daemonProcessAttrs()
}

// KillProcess sends a termination signal to the process with the given PID.
func KillProcess(pid int) error {
	return killProcess(pid)
}

// IsProcessAlive checks if a process with the given PID exists.
func IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}

// KillRelatedAppProcesses terminates other running sushiro-overdose processes
// discovered by platform process listing. excludePID is never terminated.
func KillRelatedAppProcesses(excludePID int) []MaintenanceResult {
	return killRelatedAppProcesses(excludePID)
}

// OpenBrowser opens the local Web UI. Desktop platforms prefer a standalone
// app-style window when a Chromium-based browser is available, then fall back
// to the default browser.
func OpenBrowser(url string) error {
	return openBrowser(url)
}

// AutoStartStatus 描述「开机/登录自启取号」的能力与当前状态。
// Supported=false 表示该平台不支持自启（如某些 Linux 桌面）；Enabled 表示当前是否已注册自启；
// Path 是自启条目指向的可执行路径；Message/Error 供 UI 展示诊断信息。
type AutoStartStatus struct {
	Supported bool   `json:"supported"`
	Enabled   bool   `json:"enabled"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SamplingAutoStartStatus 查询当前平台自启状态。不同平台语义不同
// （macOS 用 LaunchAgent，Windows 用注册表/启动文件夹），由各平台实现填充。
func SamplingAutoStartStatus() AutoStartStatus {
	return samplingAutoStartStatus()
}

// InstallSamplingAutoStart 注册开机自启取号。平台不支持时返回错误。
func InstallSamplingAutoStart() error {
	return installSamplingAutoStart()
}

// RemoveSamplingAutoStart 取消开机自启取号。幂等：未注册时通常返回 nil。
func RemoveSamplingAutoStart() error {
	return removeSamplingAutoStart()
}
