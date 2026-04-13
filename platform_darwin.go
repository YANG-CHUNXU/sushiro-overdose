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

	// Add cert to user login keychain
	cmd := exec.Command("security", "add-certificates", "-k",
		filepath.Join(os.Getenv("HOME"), "Library/Keychains/login.keychain-db"),
		certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add-certificates: %w", err)
	}

	// Set trust at user level
	cmd = exec.Command("security", "add-trusted-cert", "-r", "trustRoot", certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	return exec.Command("open", url).Start()
}
