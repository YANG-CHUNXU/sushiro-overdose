//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func desktopNotification(title, message string) {
	// Use PowerShell toast notification on Windows
	script := fmt.Sprintf(
		`[System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms'); $n = New-Object System.Windows.Forms.NotifyIcon; $n.Icon = [System.Drawing.SystemIcons]::Information; $n.Visible = $true; $n.ShowBalloonTip(5000, '%s', '%s', 'Info')`,
		title, message,
	)
	_ = exec.Command("powershell", "-NoProfile", "-Command", script).Run()
}

func setSystemProxy(port int) error {
	// Windows uses registry-based proxy settings
	// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
	p := fmt.Sprintf("127.0.0.1:%d", port)
	enableScript := fmt.Sprintf(
		`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -Value 1; `+
			`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyServer -Value '%s'`,
		p,
	)
	return exec.Command("powershell", "-NoProfile", "-Command", enableScript).Run()
}

func clearSystemProxy() error {
	disableScript := `Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -Value 0`
	return exec.Command("powershell", "-NoProfile", "-Command", disableScript).Run()
}

func isCertTrusted() (bool, error) {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	if _, err := os.Stat(certPath); err != nil {
		return false, nil
	}

	// Check if cert is in the Root store
	out, err := exec.Command("certutil", "-store", "-user", "Root", "Sushiro Proxy CA").CombinedOutput()
	if err != nil {
		return false, nil
	}
	return len(out) > 0, nil
}

func installCert() error {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	// Add to user's trusted root store (no admin needed)
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func daemonProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcess(pid int) error {
	// On Windows, use taskkill to terminate a process
	return exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
}

func isProcessAlive(pid int) bool {
	// Check if process exists using tasklist
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d", pid))
}

func openBrowser(url string) error {
	return exec.Command("cmd", "/c", "start", url).Run()
}
