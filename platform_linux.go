//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func desktopNotification(title, message string) {
	// Try notify-send (available on most Linux desktops)
	_ = exec.Command("notify-send", title, message).Run()
}

func setSystemProxy(port int) error {
	// Linux proxy settings vary by desktop environment.
	// Set environment variables as a baseline — most tools respect these.
	p := fmt.Sprintf("127.0.0.1:%d", port)
	_ = os.Setenv("http_proxy", "http://"+p)
	_ = os.Setenv("https_proxy", "http://"+p)
	_ = os.Setenv("HTTP_PROXY", "http://"+p)
	_ = os.Setenv("HTTPS_PROXY", "http://"+p)
	return nil
}

func clearSystemProxy() error {
	_ = os.Unsetenv("http_proxy")
	_ = os.Unsetenv("https_proxy")
	_ = os.Unsetenv("HTTP_PROXY")
	_ = os.Unsetenv("HTTPS_PROXY")
	return nil
}

func isCertTrusted() (bool, error) {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")
	if _, err := os.Stat(certPath); err != nil {
		return false, nil
	}
	// Check if cert is in system trust store
	_, err := os.Stat("/usr/local/share/ca-certificates/sushiro-proxy.crt")
	return err == nil, nil
}

func installCert() error {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")
	target := "/usr/local/share/ca-certificates/sushiro-proxy.crt"

	// Copy cert to system CA directory (may need sudo)
	src, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read cert: %w", err)
	}
	if err := os.WriteFile(target, src, 0o644); err != nil {
		return fmt.Errorf("write cert (may need sudo): %w", err)
	}
	return exec.Command("update-ca-certificates").Run()
}

func daemonProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Run()
}
