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
	// Use PowerShell toast notification on Windows — pass args as separate
	// parameters to avoid injection through string interpolation.
	script := `
[System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms')
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(5000, $args[0], $args[1], 'Info')
`
	_ = exec.Command("powershell", "-NoProfile", "-Command", script, title, message).Run()
}

func setSystemProxy(port int) error {
	p := fmt.Sprintf("127.0.0.1:%d", port)
	// Use $args instead of string interpolation to avoid injection
	script := `
Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -Value 1
Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyServer -Value $args[0]
`
	return exec.Command("powershell", "-NoProfile", "-Command", script, p).Run()
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
	return exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
}

func isProcessAlive(pid int) bool {
	// Use tasklist with exact PID filter; verify the output line contains
	// the PID as a standalone number (not a substring of a larger PID).
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").CombinedOutput()
	if err != nil {
		return false
	}
	line := strings.TrimSpace(string(out))
	// tasklist returns "INFO: No tasks are running..." when no match
	if strings.Contains(line, "No tasks") {
		return false
	}
	// Verify the PID appears as an exact number in the output
	for _, field := range strings.Fields(line) {
		if field == fmt.Sprintf("%d", pid) {
			return true
		}
	}
	return false
}

func openBrowser(url string) error {
	// Use rundll32 instead of cmd /c start to avoid URL injection
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
}
