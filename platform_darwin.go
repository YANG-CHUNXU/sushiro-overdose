//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func desktopNotification(title, message string) {
	// Escape double quotes to prevent AppleScript injection
	t := strings.ReplaceAll(title, `"`, `\"`)
	m := strings.ReplaceAll(message, `"`, `\"`)
	_ = exec.Command("osascript", "-e",
		fmt.Sprintf(`display notification "%s" with title "%s"`, m, t),
	).Run()
}

func setSystemProxy(port int) error {
	services, err := getNetworkServices()
	if err != nil {
		return err
	}
	p := fmt.Sprintf("%d", port)
	for _, svc := range services {
		runCmd("networksetup", "-setwebproxy", svc, "127.0.0.1", p)
		runCmd("networksetup", "-setsecurewebproxy", svc, "127.0.0.1", p)
		runCmd("networksetup", "-setwebproxystate", svc, "on")
		runCmd("networksetup", "-setsecurewebproxystate", svc, "on")
	}
	return nil
}

func clearSystemProxy() error {
	services, err := getNetworkServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		runCmd("networksetup", "-setwebproxystate", svc, "off")
		runCmd("networksetup", "-setsecurewebproxystate", svc, "off")
	}
	return nil
}

func getNetworkServices() ([]string, error) {
	out, err := runCmd("networksetup", "-listallnetworkservices")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var services []string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "*") {
			services = append(services, line)
		}
	}
	return services, nil
}

func isCertTrusted() (bool, error) {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	if _, err := os.Stat(certPath); err != nil {
		return false, nil
	}

	cmd := exec.Command("security", "verify-cert", "-c", certPath, "-p", "basic")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func installCert() error {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	keychain, err := defaultUserKeychain()
	if err != nil {
		return err
	}

	out, err := exec.Command("security", "add-certificates", "-k", keychain, certPath).CombinedOutput()
	if err != nil && !isAlreadyExistsOutput(out) {
		return fmt.Errorf("add-certificates: %w: %s", err, strings.TrimSpace(string(out)))
	}

	out, err = exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-k", keychain, certPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("add-trusted-cert: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func uninstallCert() error {
	keychain, err := defaultUserKeychain()
	if err != nil {
		return err
	}
	out, err := exec.Command("security", "delete-certificate", "-c", "Sushiro Proxy CA", keychain).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(msg), "could not be found") || strings.Contains(msg, "未找到") {
			return nil
		}
		return fmt.Errorf("delete-certificate: %w: %s", err, msg)
	}
	return nil
}

func defaultUserKeychain() (string, error) {
	out, err := exec.Command("security", "default-keychain", "-d", "user").CombinedOutput()
	if err == nil {
		keychain := strings.Trim(strings.TrimSpace(string(out)), `"`)
		if keychain != "" {
			return keychain, nil
		}
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", fmt.Errorf("resolve user keychain: %w", homeErr)
	}
	return filepath.Join(home, "Library/Keychains/login.keychain-db"), nil
}

func isAlreadyExistsOutput(out []byte) bool {
	msg := strings.ToLower(string(out))
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "已存在")
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
	for _, exe := range darwinChromiumExecutables() {
		if _, err := os.Stat(exe); err != nil {
			continue
		}
		if err := exec.Command(exe, "--app="+url, "--new-window").Start(); err == nil {
			return nil
		}
	}
	return exec.Command("open", url).Start()
}

func darwinChromiumExecutables() []string {
	exes := []string{
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	if home, err := os.UserHomeDir(); err == nil {
		exes = append(exes,
			filepath.Join(home, "Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"),
			filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			filepath.Join(home, "Applications/Brave Browser.app/Contents/MacOS/Brave Browser"),
			filepath.Join(home, "Applications/Chromium.app/Contents/MacOS/Chromium"),
		)
	}
	return exes
}

func samplingAutoStartStatus() AutoStartStatus {
	path := darwinSamplingLaunchAgentPath()
	if path == "" {
		return AutoStartStatus{Supported: false, Message: "无法定位 LaunchAgents 目录"}
	}
	status := AutoStartStatus{Supported: true, Path: path}
	if _, err := os.Stat(path); err == nil {
		status.Enabled = true
		status.Message = "已配置 LaunchAgent，登录后会静默启动采样"
	} else if os.IsNotExist(err) {
		status.Message = "未配置系统开机自启动"
	} else {
		status.Error = err.Error()
	}
	return status
}

func installSamplingAutoStart() error {
	path := darwinSamplingLaunchAgentPath()
	if path == "" {
		return fmt.Errorf("无法定位 LaunchAgents 目录")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(appDirPath(), 0o755); err != nil {
		return err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.sushiro-overdose.sampler</string>
  <key>ProgramArguments</key>
  <array>
    <string>` + xmlEscape(exe) + `</string>
    <string>--sampler-daemon-child</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>` + xmlEscape(samplingLogPath()) + `</string>
  <key>StandardErrorPath</key><string>` + xmlEscape(samplingLogPath()) + `</string>
</dict>
</plist>
`
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", path).Run()
	return exec.Command("launchctl", "load", path).Run()
}

func removeSamplingAutoStart() error {
	path := darwinSamplingLaunchAgentPath()
	if path == "" {
		return nil
	}
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func darwinSamplingLaunchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com.sushiro-overdose.sampler.plist")
}

func xmlEscape(value string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	).Replace(value)
}
