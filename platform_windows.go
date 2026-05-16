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
	script := `
Add-Type -AssemblyName System.Windows.Forms
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(5000, $args[0], $args[1], 'Info')
Start-Sleep -Seconds 6
$n.Dispose()
`
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script, title, message)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func setSystemProxy(port int) error {
	p := fmt.Sprintf("127.0.0.1:%d", port)
	script := `
Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -Value 1
Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyServer -Value $args[0]
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script, p)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置系统代理失败: %w", err)
	}

	refreshProxySettings()
	return nil
}

func clearSystemProxy() error {
	script := `Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -Value 0`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Run()
	refreshProxySettings()
	return err
}

// refreshProxySettings notifies the system that proxy settings have changed
// so that applications using WinINet pick up the new settings immediately.
func refreshProxySettings() {
	script := `
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class WinINet {
    [DllImport("wininet.dll", SetLastError=true)]
    public static extern bool InternetSetOption(IntPtr hInternet, int dwOption, IntPtr lpBuffer, int lpdwBufferLength);
    public const int INTERNET_OPTION_SETTINGS_CHANGED = 39;
    public const int INTERNET_OPTION_REFRESH = 37;
    public static void Refresh() {
        InternetSetOption(IntPtr.Zero, INTERNET_OPTION_SETTINGS_CHANGED, IntPtr.Zero, 0);
        InternetSetOption(IntPtr.Zero, INTERNET_OPTION_REFRESH, IntPtr.Zero, 0);
    }
}
"@
[WinINet]::Refresh()
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}

func isCertTrusted() (bool, error) {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")
	if _, err := os.Stat(certPath); err != nil {
		return false, nil
	}

	cmd := exec.Command("certutil", "-store", "-user", "Root", "Sushiro Proxy CA")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	return len(out) > 0 && !strings.Contains(string(out), "ERROR"), nil
}

func installCert() error {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func uninstallCert() error {
	cmd := exec.Command("certutil", "-delstore", "-user", "Root", "Sushiro Proxy CA")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(msg), "cannot find") || strings.Contains(strings.ToLower(msg), "not found") {
			return nil
		}
		return fmt.Errorf("删除证书失败: %w: %s", err, msg)
	}
	return nil
}

func daemonProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcess(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func isProcessAlive(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	line := strings.TrimSpace(string(out))
	if strings.Contains(line, "No tasks") {
		return false
	}
	for _, field := range strings.Fields(line) {
		if field == fmt.Sprintf("%d", pid) {
			return true
		}
	}
	return false
}

func openBrowser(url string) error {
	for _, exe := range windowsChromiumExecutables() {
		if _, err := os.Stat(exe); err != nil {
			continue
		}
		cmd := exec.Command(exe, "--app="+url, "--new-window")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func windowsChromiumExecutables() []string {
	names := []string{"msedge.exe", "chrome.exe", "brave.exe"}
	out := []string{}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			out = append(out, path)
		}
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("PROGRAMFILES")
	programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
	appendCandidate := func(base string, parts ...string) {
		if base == "" {
			return
		}
		out = append(out, filepath.Join(append([]string{base}, parts...)...))
	}
	appendCandidate(localAppData, "Microsoft", "Edge", "Application", "msedge.exe")
	appendCandidate(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe")
	appendCandidate(programFiles, "Microsoft", "Edge", "Application", "msedge.exe")
	appendCandidate(localAppData, "Google", "Chrome", "Application", "chrome.exe")
	appendCandidate(programFiles, "Google", "Chrome", "Application", "chrome.exe")
	appendCandidate(programFilesX86, "Google", "Chrome", "Application", "chrome.exe")
	appendCandidate(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe")
	appendCandidate(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe")
	appendCandidate(programFilesX86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe")
	return out
}
