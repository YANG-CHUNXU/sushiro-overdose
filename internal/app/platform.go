package app

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

type AutoStartStatus struct {
	Supported bool   `json:"supported"`
	Enabled   bool   `json:"enabled"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

func SamplingAutoStartStatus() AutoStartStatus {
	return samplingAutoStartStatus()
}

func InstallSamplingAutoStart() error {
	return installSamplingAutoStart()
}

func RemoveSamplingAutoStart() error {
	return removeSamplingAutoStart()
}
