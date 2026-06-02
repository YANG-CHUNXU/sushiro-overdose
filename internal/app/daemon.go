package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/platform"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func cmdStart() {
	if isRunning() {
		fmt.Println("sushiro is already running (PID " + readPID() + ")")
		return
	}

	os.MkdirAll(AppDirPath(), 0o755)

	self, _ := os.Executable()
	cmd := exec.Command(self, "--daemon-child")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = DaemonProcessAttrs()

	if err := cmd.Start(); err != nil {
		fmt.Println("启动失败:", err)
		os.Exit(1)
	}

	os.WriteFile(PidFilePath(), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)
	fmt.Printf("sushiro started (PID %d)\n", cmd.Process.Pid)
	fmt.Println("日志: " + LogPath())
}

func cmdStop() {
	pid := readPID()
	if pid == "" {
		fmt.Println("sushiro is not running")
		return
	}
	if err := KillProcess(atoi(pid)); err != nil {
		fmt.Println("停止失败:", err)
		os.Remove(PidFilePath())
		return
	}
	os.Remove(PidFilePath())
	fmt.Println("sushiro stopped")
}

func cmdStatus() {
	pid := readPID()
	if pid == "" || !isRunning() {
		fmt.Println("sushiro is not running")
		return
	}
	fmt.Printf("sushiro is running (PID %s)\n", pid)

	log, err := os.ReadFile(LogPath())
	if err == nil && len(log) > 0 {
		lines := strings.Split(strings.TrimSpace(string(log)), "\n")
		start := len(lines) - 10
		if start < 0 {
			start = 0
		}
		fmt.Println("\n最近日志:")
		for _, line := range lines[start:] {
			fmt.Println("  " + line)
		}
	}
}

func cmdDaemon() {
	os.MkdirAll(AppDirPath(), 0o755)

	logFile, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer logFile.Close()

	os.Stdout = logFile
	os.Stderr = logFile

	LogMessage(time.Now(), "sushiro daemon started")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		LogMessage(time.Now(), "exit with error: "+err.Error())
	}

	os.Remove(PidFilePath())
}

func readPID() string {
	data, err := os.ReadFile(PidFilePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isRunning() bool {
	pid := readPID()
	if pid == "" {
		return false
	}
	return IsProcessAlive(atoi(pid))
}

func atoi(s string) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return -1
	}
	return n
}
