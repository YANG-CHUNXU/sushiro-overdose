package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	localConfig = ".sushiro_local.json"
	appDir      = ".sushiro"
	pidFile     = "sushiro.pid"
	logFilePath = "sushiro.log"
)

func appDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, appDir)
}

func pidFilePath() string {
	return filepath.Join(appDirPath(), pidFile)
}

func logPath() string {
	return filepath.Join(appDirPath(), logFilePath)
}

func printBanner() {
	fmt.Println(" ▄██████▄  ████████▄     ▄████████ ███    █▄     ▄████████    ▄█    █▄     ▄█")
	fmt.Println("███    ███ ███   ▀███   ███    ███ ███    ███   ███    ███   ███    ███   ███")
	fmt.Println("███    ███ ███    ███   ███    █▀  ███    ███   ███    █▀    ███    ███   ███▌")
	fmt.Println("███    ███ ███    ███   ███        ███    ███   ███         ▄███▄▄▄▄███▄▄ ███▌")
	fmt.Println("███    ███ ███    ███ ▀███████████ ███    ███ ▀███████████ ▀▀███▀▀▀▀███▀  ███▌")
	fmt.Println("███    ███ ███    ███          ███ ███    ███          ███   ███    ███   ███")
	fmt.Println("███    ███ ███   ▄███    ▄█    ███ ███    ███    ▄█    ███   ███    ███   ███")
	fmt.Println(" ▀██████▀  ████████▀   ▄████████▀  ████████▀   ▄████████▀    ███    █▀    █▀")
	fmt.Println()
	fmt.Println("寿司郎重度依赖 - https://github.com/Ryujoxys")
	fmt.Println()
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 || (len(args) == 1 && (args[0] == "run" || args[0] == "-f" || args[0] == "--foreground")) {
		cmdForeground()
	} else if len(args) == 1 && (args[0] == "start" || args[0] == "-d" || args[0] == "--daemon") {
		cmdStart()
	} else if len(args) == 1 && (args[0] == "exit" || args[0] == "stop") {
		cmdStop()
	} else if len(args) == 1 && args[0] == "status" {
		cmdStatus()
	} else if len(args) == 1 && args[0] == "--daemon-child" {
		cmdDaemon()
	} else if len(args) >= 1 && (args[0] == "config" || args[0] == "setting" || args[0] == "settings") {
		cmdConfig(args[1:])
	} else {
		fmt.Println("Usage: sushiro [command]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  (no args)    Run in foreground (interactive)")
		fmt.Println("  start, -d    Start in background (daemon)")
		fmt.Println("  status       Show running status")
		fmt.Println("  exit         Stop background process")
		fmt.Println("  setting      Configure settings (feishu, etc.)")
		fmt.Println("  config       Alias for setting")
	}
}

// ---- CLI Commands ----

func cmdStart() {
	if isRunning() {
		fmt.Println("sushiro is already running (PID " + readPID() + ")")
		return
	}

	// Ensure app dir
	os.MkdirAll(appDirPath(), 0o755)

	self, _ := os.Executable()
	cmd := exec.Command(self, "--daemon-child")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		fmt.Println("启动失败:", err)
		os.Exit(1)
	}

	// Write PID
	os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)
	fmt.Printf("sushiro started (PID %d)\n", cmd.Process.Pid)
	fmt.Println("日志: " + logPath())
}

func cmdStop() {
	pid := readPID()
	if pid == "" {
		fmt.Println("sushiro is not running")
		return
	}
	if err := syscall.Kill(atoi(pid), syscall.SIGTERM); err != nil {
		fmt.Println("停止失败:", err)
		os.Remove(pidFilePath())
		return
	}
	os.Remove(pidFilePath())
	fmt.Println("sushiro stopped")
}

func cmdStatus() {
	pid := readPID()
	if pid == "" || !isRunning() {
		fmt.Println("sushiro is not running")
		return
	}
	fmt.Printf("sushiro is running (PID %s)\n", pid)

	// Show last 10 lines of log
	log, err := os.ReadFile(logPath())
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

func cmdForeground() {
	printBanner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdDaemon() {
	// Silent mode — redirect all output to log file
	os.MkdirAll(appDirPath(), 0o755)

	logFile, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer logFile.Close()

	// Redirect stdout and stderr
	os.Stdout = logFile
	os.Stderr = logFile

	// Also set log output for standard log
	logMessage(time.Now(), "sushiro daemon started")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logMessage(time.Now(), "exit with error: "+err.Error())
	}

	os.Remove(pidFilePath())
}

func cmdConfig(args []string) {
	if len(args) == 0 {
		// Interactive mode
		tokens, err := loadLocalConfig()
		if err != nil {
			fmt.Println("暂无配置，请先运行 sushiro")
			return
		}
		fmt.Println("当前设置:")
		fmt.Printf("  飞书通知: ")
		tokens.mu.Lock()
		if tokens.FeishuWebhook != "" {
			fmt.Printf("已配置 (%s...)\n", tokens.FeishuWebhook[:min(50, len(tokens.FeishuWebhook))])
		} else {
			fmt.Println("未配置")
		}
		tokens.mu.Unlock()
		fmt.Println()
		fmt.Print("配置飞书通知？输入 Webhook 地址（留空跳过，输入 clear 清除）: ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)
		if strings.ToLower(input) == "clear" {
			updateLocalConfigFeishu("")
			fmt.Println("飞书通知已清除")
		} else if input != "" {
			updateLocalConfigFeishu(input)
			fmt.Println("飞书通知已配置!")
		}
		return
	}

	switch args[0] {
	case "feishu":
		if len(args) < 2 {
			fmt.Println("Usage: sushiro config feishu <webhook_url>")
			return
		}
		if args[1] == "--clear" {
			updateLocalConfigFeishu("")
			fmt.Println("飞书通知已清除")
			return
		}
		updateLocalConfigFeishu(args[1])
		fmt.Println("飞书通知已配置!")
	default:
		fmt.Println("Unknown config key:", args[0])
	}
}

// ---- PID management ----

func readPID() string {
	data, err := os.ReadFile(pidFilePath())
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
	err := syscall.Kill(atoi(pid), 0)
	return err == nil
}

func atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ---- Update feishu in local config ----

func updateLocalConfigFeishu(webhook string) {
	tokens, err := loadLocalConfig()
	if err != nil {
		fmt.Println("无法加载配置:", err)
		return
	}
	tokens.FeishuWebhook = webhook
	if err := saveLocalConfig(tokens); err != nil {
		fmt.Println("保存失败:", err)
	}
}

// ---- Feishu notification helper ----

func sendFeishuNotification(settings Settings, title, content string) {
	if settings.FeishuWebhook == "" {
		return
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": title},
			"template": "green",
		},
		"elements": []map[string]any{
			{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}},
			{"tag": "note", "elements": []map[string]any{
				{"tag": "plain_text", "content": time.Now().Format("2006-01-02 15:04:05")},
			}},
		},
	}
	client := &http.Client{Timeout: 5 * time.Second}
	payload, _ := json.Marshal(map[string]any{"msg_type": "interactive", "card": card})
	req, _ := http.NewRequest(http.MethodPost, settings.FeishuWebhook, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// ---- Main run logic (shared by foreground and daemon) ----

func run(ctx context.Context) error {
	tokens, configExists := tryLoadConfig()

	if !configExists {
		var err error
		tokens, err = runCapturePhase(ctx)
		if err != nil {
			return err
		}
		if err := saveLocalConfig(tokens); err != nil {
			logMessage(time.Now(), "保存配置失败: "+err.Error())
		} else {
			logMessage(time.Now(), "配置已保存到 "+localConfig)
		}
	}

	settings := tokens.toSettings()

	// Interactive Feishu config
	tokens.mu.Lock()
	feishu := tokens.FeishuWebhook
	tokens.mu.Unlock()
	if feishu == "" {
		fmt.Println()
		fmt.Print("是否配置飞书通知机器人？(y/N): ")
		var answer string
		fmt.Scanln(&answer)
		if strings.TrimSpace(strings.ToLower(answer)) == "y" {
			fmt.Println("飞书群 → 群设置 → 群机器人 → 添加自定义机器人 → 复制 Webhook 地址")
			fmt.Print("请输入 Webhook 地址: ")
			var webhook string
			fmt.Scanln(&webhook)
			webhook = strings.TrimSpace(webhook)
			if webhook != "" {
				tokens.mu.Lock()
				tokens.FeishuWebhook = webhook
				tokens.mu.Unlock()
				saveLocalConfig(tokens)
				settings.FeishuWebhook = webhook
				fmt.Println("飞书通知已配置!")
			}
		}
	}

	client := NewClient(settings)

	// Verify config still works
	logMessage(time.Now(), "验证认证参数...")
	if _, err := client.GetTimeslots(ctx, settings.StoreIDs[0]); err != nil {
		logMessage(time.Now(), "验证失败: "+err.Error())
		logMessage(time.Now(), "认证参数可能已过期，需要重新获取...")
		sendFeishuNotification(settings, "寿司郎 - 认证过期", "需要重新运行捕获认证参数")
		deleteLocalConfig()
		tokens, err = runCapturePhase(ctx)
		if err != nil {
			return err
		}
		saveLocalConfig(tokens)
		settings = tokens.toSettings()
		client = NewClient(settings)
	}

	selectedStores, err := selectStores(ctx, client, tokens)
	if err != nil {
		return fmt.Errorf("选择门店失败: %w", err)
	}
	settings.StoreIDs = selectedStores

	tokens.mu.Lock()
	tokens.StoreIDs = selectedStores
	tokens.mu.Unlock()
	saveLocalConfig(tokens)

	slotConfig := configureSlots()

	logMessage(time.Now(), "开始抢号...")
	runBookingLoop(ctx, client, settings, selectedStores, slotConfig)
	return nil
}

func runCapturePhase(ctx context.Context) (*CapturedTokens, error) {
	// Load or generate CA certificate
	caCert, caKey, err := loadOrGenerateCA()
	if err != nil {
		return nil, fmt.Errorf("CA证书加载失败: %w", err)
	}

	// Check if cert is trusted, offer to install
	trusted, _ := isCertTrusted()
	if !trusted {
		fmt.Println("\n首次运行需要安装CA证书（用于拦截HTTPS流量）")
		fmt.Println("安装后需要输入登录密码确认")
		if err := installCert(); err != nil {
			return nil, fmt.Errorf("证书安装失败: %w", err)
		}
		fmt.Println("证书安装成功!")
	}

	tokens := newCapturedTokens()

	// Start MITM proxy
	proxy, err := startProxy(caCert, caKey, tokens)
	if err != nil {
		return nil, fmt.Errorf("启动代理失败: %w", err)
	}
	defer proxy.close()

	// Set system proxy
	if err := setSystemProxy(); err != nil {
		return nil, fmt.Errorf("设置系统代理失败: %w", err)
	}
	fmt.Println("系统代理已设置 (127.0.0.1:8080)")

	// Ensure proxy is cleared on exit
	defer func() {
		clearSystemProxy()
		fmt.Println("系统代理已清除")
	}()

	// Wait for capture with skip channel
	skipCapture := make(chan struct{})
	go func() {
		var buf [1]byte
		os.Stdin.Read(buf[:])
		close(skipCapture)
	}()

	if err := waitForCapture(ctx, tokens, skipCapture); err != nil {
		return nil, err
	}

	// Clean up proxy
	proxy.close()
	clearSystemProxy()

	// Prompt phone number if not captured
	tokens.mu.Lock()
	phone := tokens.PhoneNumber
	tokens.mu.Unlock()
	if phone == "" {
		fmt.Print("\n请输入手机号: ")
		var input string
		fmt.Scanln(&input)
		tokens.mu.Lock()
		tokens.PhoneNumber = strings.TrimSpace(input)
		tokens.mu.Unlock()
	}

	return tokens, nil
}

func tryLoadConfig() (*CapturedTokens, bool) {
	tokens, err := loadLocalConfig()
	if err != nil {
		return nil, false
	}
	logMessage(time.Now(), "使用已保存的配置")
	logMessage(time.Now(), fmt.Sprintf("  手机号: %s", tokens.PhoneNumber))
	logMessage(time.Now(), fmt.Sprintf("  门店: %v", tokens.StoreIDs))
	return tokens, true
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "token") ||
		strings.Contains(msg, "auth")
}

func runBookingLoop(ctx context.Context, client *Client, settings Settings, storeIDs []string, cfg SlotConfig) {
	var booked map[string]bool
	errStreak := 0
	authErrors := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		now := time.Now().In(settings.Location)

		var best *TargetSlot
		var bestStatus string
		for _, storeID := range storeIDs {
			slots, err := client.GetTimeslots(ctx, storeID)
			if err != nil {
				if isAuthError(err) {
					authErrors++
					if authErrors >= 3 {
						logMessage(now, "认证失败，请重新运行获取新参数")
						sendFeishuNotification(settings, "寿司郎 - 认证失败", "认证参数已失效，请重新运行 `sushiro run` 获取新参数")
						deleteLocalConfig()
						return
					}
				}
				errStreak++
				if errStreak >= 5 {
					logMessage(now, "连续失败过多，等待5秒...")
					time.Sleep(5 * time.Second)
					errStreak = 0
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			errStreak = 0
			authErrors = 0

			for i := range slots {
				if !cfg.shouldTarget(slots[i], settings.Location) {
					continue
				}
				key := storeID + slots[i].Date + slots[i].Start
				if booked != nil && booked[key] {
					continue
				}
				t := &TargetSlot{
					StoreID: storeID,
					Date:    slots[i].Date,
					Start:   slots[i].Start,
					End:     slots[i].End,
				}
				if best == nil || t.Date+t.Start > best.Date+best.Start {
					best = t
					bestStatus = slots[i].Availability
				}
			}
		}

		if best == nil {
			fmt.Printf("\r[%s] 查询中...无目标时段", now.Format("15:04:05"))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		slotLabel := formatSlotWindow(best.Date, best.Start, best.End, settings.Location)
		statusTag := ""
		if bestStatus != "AVAILABLE" {
			statusTag = " [" + bestStatus + "]"
		}
		fmt.Printf("\r[%s] %s%s - 尝试预约...", now.Format("15:04:05"), slotLabel, statusTag)

		reservation, err := client.CreateReservation(ctx, best.StoreID, best.Date, best.Start)
		if err != nil {
			key := best.StoreID + best.Date + best.Start
			if booked == nil {
				booked = make(map[string]bool)
			}
			booked[key] = true

			if isAuthError(err) {
				authErrors++
				if authErrors >= 3 {
					logMessage(now, "认证失败，请重新运行")
					sendFeishuNotification(settings, "寿司郎 - 认证失败", "请重新运行 `sushiro run`")
					deleteLocalConfig()
					return
				}
			}

			if strings.Contains(err.Error(), "500") {
				fmt.Printf("\r[%s] HTTP 500，暂停1秒...", now.Format("15:04:05"))
				time.Sleep(1 * time.Second)
			} else if errors.Is(err, errNoReservationAvailable) {
				fmt.Printf("\r[%s] %s - 名额已满", now.Format("15:04:05"), slotLabel)
			} else {
				fmt.Printf("\r[%s] %s - 失败: %s", now.Format("15:04:05"), slotLabel, err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Success!
		now = time.Now().In(settings.Location)
		storeInfo, _ := client.GetStoreInfo(ctx, best.StoreID)
		storeName := storeInfo.Name
		if storeName == "" {
			storeName = best.StoreID
		}

		reservation.MonitoredStoreID = best.StoreID
		reservation.StoreName = storeName
		reservation.StoreAddress = storeInfo.Address
		reservation.SlotLabel = slotLabel

		state := State{
			ActiveReservation: &reservation,
			SavedAt:           now.Format(time.RFC3339),
		}
		_ = saveState(settings.StateFile, state)

		fmt.Println()
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════╗")
		fmt.Println("  ║         🎉 预约成功！                ║")
		fmt.Println("  ╠══════════════════════════════════════╣")
		fmt.Printf("  ║  门店：%s\n", storeName)
		fmt.Printf("  ║  时段：%s\n", slotLabel)
		fmt.Printf("  ║  号码：%s\n", reservation.Number)
		if storeInfo.Address != "" {
			fmt.Printf("  ║  地址：%s\n", storeInfo.Address)
		}
		fmt.Println("  ╚══════════════════════════════════════╝")
		fmt.Println()

		logMessage(now, "=== 预约成功 ===")
		logMessage(now, fmt.Sprintf("  门店：%s", storeName))
		logMessage(now, fmt.Sprintf("  时段：%s", slotLabel))
		logMessage(now, fmt.Sprintf("  号码：%s", reservation.Number))
		if storeInfo.Address != "" {
			logMessage(now, fmt.Sprintf("  地址：%s", storeInfo.Address))
		}

		// macOS notification
		title := fmt.Sprintf("寿司郎预约成功 - %s", storeName)
		message := fmt.Sprintf("号码: %s | 时段: %s", reservation.Number, slotLabel)
		_ = exec.Command("osascript", "-e",
			fmt.Sprintf(`display notification "%s" with title "%s"`, message, title),
		).Run()

		// Feishu notification
		content := fmt.Sprintf("### %s\n**号码**：`%s`\n**时段**：%s\n**地址**：%s",
			storeName, reservation.Number, slotLabel, storeInfo.Address)
		sendFeishuNotification(settings, title, content)

		return
	}
}
