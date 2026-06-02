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
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const samplingPidFile = "sampling.pid"

func samplingPidFilePath() string {
	return filepath.Join(AppDirPath(), samplingPidFile)
}

func cmdSample(args []string) {
	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch action {
	case "status", "":
		cmdSampleStatus()
	case "once":
		cmdSampleOnce()
	case "run":
		cmdSampleRun()
	case "start":
		cmdSampleStart()
	case "stop", "exit":
		cmdSampleStop()
	case "autostart", "login":
		cmdSampleAutoStart(args[1:])
	default:
		fmt.Println("Usage: sushiro-overdose sample [status|once|run|start|stop|autostart]")
	}
}

func cmdSampleStatus() {
	cfg := LoadSamplingConfig()
	fmt.Println("信息收集:", samplingSummary(cfg))
	pid := readSamplingPID()
	if pid != "" && IsProcessAlive(atoi(pid)) {
		fmt.Println("状态: 运行中 (PID " + pid + ")")
	} else if holder, ok := processLockHolder(samplingLockFileName); ok {
		fmt.Printf("状态: 运行中 (PID %d，应用内/前台信息收集)\n", holder)
	} else {
		fmt.Println("状态: 未运行")
	}
	fmt.Println("配置文件: " + samplingConfigPath())
}

func cmdSampleOnce() {
	cfg := LoadSamplingConfig()
	cfg.Enabled = true
	result := sampler.RunOnceNow(context.Background(), cfg)
	printSamplingResult(result)
}

func cmdSampleRun() {
	printBanner()
	cfg := LoadSamplingConfig()
	cfg.Enabled = true
	if err := SaveSamplingConfig(cfg); err != nil {
		fmt.Println("保存信息收集配置失败:", err)
		return
	}
	fmt.Println("信息收集前台运行:", samplingSummary(cfg))
	fmt.Println("按 Ctrl+C 退出")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := sampler.startWithConfig(ctx, cfg); err != nil {
		fmt.Println("启动信息收集失败:", err)
		return
	}
	<-ctx.Done()
	sampler.Stop()
}

func cmdSampleStart() {
	if isSamplingDaemonRunning() {
		fmt.Println("sampling is already running (PID " + readSamplingPID() + ")")
		return
	}
	if holder, ok := processLockHolder(samplingLockFileName); ok {
		fmt.Printf("sampling is already running (PID %d)\n", holder)
		return
	}
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	self, _ := os.Executable()
	cmd := exec.Command(self, "--sampler-daemon-child")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = DaemonProcessAttrs()
	if err := cmd.Start(); err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	childPID := cmd.Process.Pid
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if readSamplingPID() == fmt.Sprintf("%d", childPID) && IsProcessAlive(childPID) {
			fmt.Printf("sampling started (PID %d)\n", childPID)
			fmt.Println("日志: " + SamplingLogPath())
			return
		}
		if !IsProcessAlive(childPID) {
			fmt.Println("启动失败，请查看日志: " + SamplingLogPath())
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if IsProcessAlive(childPID) {
		fmt.Printf("sampling starting (PID %d)\n", childPID)
		fmt.Println("日志: " + SamplingLogPath())
		return
	}
	fmt.Println("启动失败，请查看日志: " + SamplingLogPath())
}

func cmdSampleStop() {
	stopped, err := stopSamplingDaemon()
	if err != nil {
		fmt.Println("停止失败:", err)
		return
	}
	if !stopped {
		fmt.Println("sampling is not running")
		return
	}
	fmt.Println("sampling stopped")
}

func cmdSampleAutoStart(args []string) {
	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch action {
	case "status", "":
		status := SamplingAutoStartStatus()
		fmt.Println("系统开机自启动:", autoStartSummary(status))
		if status.Path != "" {
			fmt.Println("位置:", status.Path)
		}
	case "on", "enable", "start":
		if err := InstallSamplingAutoStart(); err != nil {
			fmt.Println("启用失败:", err)
			return
		}
		fmt.Println("系统开机自启动已启用")
	case "off", "disable", "stop":
		if err := RemoveSamplingAutoStart(); err != nil {
			fmt.Println("取消失败:", err)
			return
		}
		fmt.Println("系统开机自启动已取消")
	default:
		fmt.Println("Usage: sushiro-overdose sample autostart [status|on|off]")
	}
}

func autoStartSummary(status AutoStartStatus) string {
	if !status.Supported {
		if status.Message != "" {
			return "不支持 (" + status.Message + ")"
		}
		return "不支持"
	}
	if status.Enabled {
		return "已启用"
	}
	if status.Error != "" {
		return "状态异常: " + status.Error
	}
	return "未启用"
}

func cmdSamplerDaemon() {
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return
	}
	logFile, err := os.OpenFile(SamplingLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer logFile.Close()
	os.Stdout = logFile
	os.Stderr = logFile
	LogMessage(time.Now(), "sampling daemon started")

	cfg := LoadSamplingConfig()
	cfg.Enabled = true
	cfg.AutoStart = true
	if err := SaveSamplingConfig(cfg); err != nil {
		LogMessage(time.Now(), "save sampling config failed: "+err.Error())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	netTicketSched.Start(ctx)
	if err := sampler.startWithConfig(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		LogMessage(time.Now(), "sampling daemon failed: "+err.Error())
		return
	}
	writeSamplingPID(os.Getpid())
	defer removeSamplingPID(os.Getpid())
	<-ctx.Done()
	sampler.Stop()
}

func readSamplingPID() string {
	data, err := os.ReadFile(samplingPidFilePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isSamplingDaemonRunning() bool {
	pid := readSamplingPID()
	if pid == "" {
		return false
	}
	n := atoi(pid)
	if n > 0 && IsProcessAlive(n) {
		return true
	}
	removeSamplingPID(n)
	return false
}

func writeSamplingPID(pid int) {
	_ = os.WriteFile(samplingPidFilePath(), []byte(fmt.Sprintf("%d", pid)), 0o644)
}

func removeSamplingPID(pid int) {
	current := atoi(readSamplingPID())
	if pid <= 0 || current == pid {
		_ = os.Remove(samplingPidFilePath())
	}
}

func stopSamplingDaemon() (bool, error) {
	pidStr := readSamplingPID()
	if pidStr == "" {
		return false, nil
	}
	pid := atoi(pidStr)
	if pid <= 0 || !IsProcessAlive(pid) {
		removeSamplingPID(pid)
		return false, nil
	}
	if pid == os.Getpid() {
		return false, nil
	}
	if err := KillProcess(pid); err != nil {
		return true, err
	}
	removeSamplingPID(pid)
	return true, nil
}

func printSamplingResult(result SamplingRunResult) {
	if result.Skipped {
		fmt.Println("本轮跳过:", result.SkipReason)
		return
	}
	fmt.Printf("信息收集完成: %d 家门店, %d 条时段, %d 个错误\n", len(result.Stores), result.Snapshots, result.StoreErrors)
	for _, store := range result.Stores {
		if store.Error != "" {
			fmt.Printf("  %s %s: %s\n", store.StoreID, store.StoreName, store.Error)
			continue
		}
		fmt.Printf("  %s %s: %d 条\n", store.StoreID, store.StoreName, store.Slots)
	}
}
