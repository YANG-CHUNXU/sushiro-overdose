package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	appDir      = ".sushiro"
	pidFile     = "sushiro.pid"
	logFilePath = "sushiro.log"
)

// Version is set via ldflags at build time.
var Version = "dev"

func appDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, appDir)
}

func pidFilePath() string {
	return filepath.Join(appDirPath(), pidFile)
}

func stateFilePath() string {
	return filepath.Join(appDirPath(), ".sushiro_state.json")
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
	fmt.Printf("寿司郎 Overdose v%s — https://github.com/Ryujoxys/sushiro-overdose\n", Version)
	fmt.Println()
}

func printUsage() {
	fmt.Println("Usage: sushiro-overdose [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (no args)    Launch Web UI (recommended)")
	fmt.Println("  web          Launch Web UI")
	fmt.Println("  cli          Run in foreground CLI mode (advanced)")
	fmt.Println("  start, -d    Start booking daemon in background")
	fmt.Println("  status       Show running status")
	fmt.Println("  calendar     View available time slots (nearest 7 days)")
	fmt.Println("  sniper       Sniper mode - pre-book unopened slots")
	fmt.Println("  list         Show current reservations")
	fmt.Println("  cancel <id>  Cancel a reservation by ticket ID")
	fmt.Println("  trends       Analyze slot availability trends")
	fmt.Println("  recommend    Smart time slot recommendations")
	fmt.Println("  doctor       Print readonly diagnostics")
	fmt.Println("  repair-proxy Restore system proxy settings")
	fmt.Println("  uninstall    Remove local sensitive data and certificate")
	fmt.Println("  exit         Stop background process")
	fmt.Println("  config       Configure settings (feishu, telegram, bark, etc.)")
	fmt.Println("  help         Show this help message")
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "doctor" || args[0] == "diagnostics") {
		cmdDoctor()
		return
	}

	os.MkdirAll(appDirPath(), 0o755)
	migrateOldConfig()

	if len(args) == 0 || (len(args) == 1 && args[0] == "web") {
		cmdWeb()
	} else if len(args) == 1 && (args[0] == "cli" || args[0] == "run" || args[0] == "-f" || args[0] == "--foreground") {
		cmdForeground()
	} else if len(args) == 1 && (args[0] == "start" || args[0] == "-d" || args[0] == "--daemon") {
		cmdStart()
	} else if len(args) == 1 && (args[0] == "exit" || args[0] == "stop") {
		cmdStop()
	} else if len(args) == 1 && args[0] == "status" {
		cmdStatus()
	} else if len(args) == 1 && args[0] == "calendar" {
		cmdCalendar()
	} else if len(args) >= 1 && args[0] == "sniper" {
		cmdSniper(args[1:])
	} else if len(args) == 1 && (args[0] == "list" || args[0] == "reservations") {
		cmdList()
	} else if len(args) >= 1 && args[0] == "cancel" {
		cmdCancel(args[1:])
	} else if len(args) == 1 && (args[0] == "trends" || args[0] == "history") {
		cmdTrends()
	} else if len(args) == 1 && (args[0] == "recommend" || args[0] == "rec") {
		cmdRecommend()
	} else if len(args) == 1 && args[0] == "--daemon-child" {
		cmdDaemon()
	} else if len(args) == 1 && (args[0] == "repair-proxy" || args[0] == "repair") {
		cmdRepairProxy()
	} else if len(args) >= 1 && (args[0] == "uninstall" || args[0] == "purge") {
		cmdUninstall(args[1:])
	} else if len(args) >= 1 && (args[0] == "config" || args[0] == "setting" || args[0] == "settings") {
		cmdConfig(args[1:])
	} else if len(args) >= 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
	} else {
		printUsage()
	}
}

// ---- CLI Commands ----

func cmdForeground() {
	printBanner()

	// Check for stale proxy from a previous crashed run
	if checkStaleProxy() {
		fmt.Println("已清除上次异常退出的系统代理设置")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdConfig(args []string) {
	if len(args) == 0 {
		fmt.Println("当前通知设置:")
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		fmt.Printf("  飞书: %s\n", notifyStatus(cfg.Feishu.Webhook))
		fmt.Printf("  Telegram: %s\n", notifyStatus(cfg.Telegram.Token))
		fmt.Printf("  Bark: %s\n", notifyStatus(cfg.Bark.Key))
		fmt.Printf("  Server酱: %s\n", notifyStatus(cfg.ServerChan.Key))
		fmt.Println()
		fmt.Print("配置飞书通知？输入 Webhook 地址（留空跳过，输入 clear 清除）: ")
		input := readInput()
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
	case "telegram":
		if len(args) < 3 {
			fmt.Println("Usage: sushiro config telegram <bot_token> <chat_id>")
			return
		}
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		cfg.Telegram.Token = args[1]
		cfg.Telegram.ChatID = args[2]
		saveNotifyConfig(cfg)
		fmt.Println("Telegram 通知已配置!")
	case "bark":
		if len(args) < 3 {
			fmt.Println("Usage: sushiro config bark <server_url> <device_key>")
			return
		}
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		cfg.Bark.URL = args[1]
		cfg.Bark.Key = args[2]
		saveNotifyConfig(cfg)
		fmt.Println("Bark 通知已配置!")
	case "serverchan", "server-chan", "sct":
		if len(args) < 2 {
			fmt.Println("Usage: sushiro config serverchan <send_key>")
			return
		}
		cfg, _ := loadNotifyConfig()
		if cfg == nil {
			cfg = &notifyConfig{}
		}
		cfg.ServerChan.Key = args[1]
		saveNotifyConfig(cfg)
		fmt.Println("Server酱 通知已配置!")
	case "store":
		if len(args) < 2 {
			reg := GetStoreRegistry()
			stores := reg.List()
			if len(stores) == 0 {
				fmt.Println("无已配置的门店昵称")
			} else {
				for _, s := range stores {
					fmt.Printf("  %s -> %s\n", s.ID, s.Nickname)
				}
			}
			return
		}
		reg := GetStoreRegistry()
		switch args[1] {
		case "add":
			if len(args) < 4 {
				fmt.Println("Usage: sushiro config store add <store_id> <nickname>")
				return
			}
			reg.Add(args[2], args[3])
			fmt.Printf("门店 %s 昵称已设置为 %s\n", args[2], args[3])
		case "remove", "rm", "delete":
			if len(args) < 3 {
				fmt.Println("Usage: sushiro config store remove <store_id>")
				return
			}
			reg.Remove(args[2])
			fmt.Printf("门店 %s 昵称已移除\n", args[2])
		default:
			fmt.Println("Usage: sushiro config store [add|remove]")
		}
	default:
		fmt.Println("未知配置项:", args[0])
		fmt.Println("可用: feishu, telegram, bark, serverchan, store")
	}
}

func notifyStatus(v string) string {
	if v != "" {
		return "已配置 (" + v[:min(30, len(v))] + "...)"
	}
	return "未配置"
}

// ---- Update feishu in local config ----

func updateLocalConfigFeishu(webhook string) {
	saveFeishuConfig(webhook)
	// Also update notify config
	cfg, _ := loadNotifyConfig()
	if cfg == nil {
		cfg = &notifyConfig{}
	}
	cfg.Feishu.Webhook = webhook
	saveNotifyConfig(cfg)
	// Also update in-memory tokens if loaded
	tokens, err := loadLocalConfig()
	if err == nil {
		tokens.FeishuWebhook = webhook
	}
}

// ---- Notification helper ----

var globalNotifierMu sync.RWMutex
var globalNotifier *MultiNotifier

func setNotifier(n *MultiNotifier) {
	globalNotifierMu.Lock()
	globalNotifier = n
	globalNotifierMu.Unlock()
}

func sendNotification(title, content string) {
	globalNotifierMu.RLock()
	n := globalNotifier
	globalNotifierMu.RUnlock()
	if n != nil {
		n.Send(context.Background(), title, content)
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
			logMessage(time.Now(), "配置已保存到 "+localConfigPath())
		}
	}

	settings := tokens.toSettings()

	// Initialize notifier
	setNotifier(BuildNotifierFromConfig())

	// Interactive Feishu config (legacy support)
	tokens.mu.Lock()
	feishu := tokens.FeishuWebhook
	tokens.mu.Unlock()
	if feishu == "" {
		fmt.Println()
		fmt.Print("是否配置飞书通知机器人？(y/N): ")
		if answer := readInput(); strings.ToLower(answer) == "y" {
			fmt.Println("飞书群 → 群设置 → 群机器人 → 添加自定义机器人 → 复制 Webhook 地址")
			fmt.Print("请输入 Webhook 地址: ")
			if webhook := readInput(); webhook != "" {
				tokens.mu.Lock()
				tokens.FeishuWebhook = webhook
				tokens.mu.Unlock()
				saveFeishuConfig(webhook)
				settings.FeishuWebhook = webhook
				// Add feishu to notifier
				globalNotifier.Add(&feishuNotifier{webhook: webhook})
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
		sendNotification("寿司郎 - 认证过期", "需要重新运行捕获认证参数")
		deleteLocalConfig()
		tokens, err = runCapturePhase(ctx)
		if err != nil {
			return err
		}
		saveLocalConfig(tokens)
		settings = tokens.toSettings()
		client = NewClient(settings)
	}

	if err := tokens.validateForReservation(); err != nil {
		logMessage(time.Now(), "预约参数不完整，需要重新捕获: "+err.Error())
		deleteLocalConfig()
		tokens, err = runCapturePhase(ctx)
		if err != nil {
			return err
		}
		if err := tokens.validateForReservation(); err != nil {
			return err
		}
		if err := saveLocalConfig(tokens); err != nil {
			logMessage(time.Now(), "保存配置失败: "+err.Error())
		}
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

	prefs := LoadPreferences()
	prefs.SelectedStores = selectedStores

	// CLI mode: offer to configure slots interactively or use saved
	if len(prefs.WeekdaySlots) == 0 && len(prefs.SaturdaySlots) == 0 && len(prefs.SundaySlots) == 0 {
		slotConfig := configureSlots()
		prefs.WeekdaySlots = slotPrefToRanges(slotConfig.Weekday)
		prefs.SaturdaySlots = slotPrefToRanges(slotConfig.Saturday)
		prefs.SundaySlots = slotPrefToRanges(slotConfig.Sunday)
	} else {
		fmt.Println("\n使用已保存的时段偏好（可通过 Web UI 修改）")
	}
	SavePreferences(prefs)

	healthStop := startHealthCheck(ctx, client, selectedStores)
	defer close(healthStop)

	logMessage(time.Now(), "开始抢号...")
	runBookingLoop(ctx, client, settings, selectedStores, prefs)
	return nil
}

func runCapturePhase(ctx context.Context) (*CapturedTokens, error) {
	// Load or generate CA certificate
	caCert, caKey, err := loadOrGenerateCA()
	if err != nil {
		return nil, fmt.Errorf("CA证书加载失败: %w", err)
	}

	// Check if cert is trusted, offer to install
	trusted, _ := IsCertTrusted()
	if !trusted {
		fmt.Println("\n首次运行需要安装CA证书（用于拦截HTTPS流量）")
		fmt.Println("安装后需要输入登录密码确认")
		if err := InstallCert(); err != nil {
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
	if err := SetSystemProxy(proxyPort); err != nil {
		return nil, fmt.Errorf("设置系统代理失败: %w", err)
	}
	fmt.Println("系统代理已设置 (127.0.0.1:8080)")
	markProxyActive(proxyPort, os.Getpid())

	// Ensure proxy is cleared on exit
	defer func() {
		ClearSystemProxy()
		markProxyInactive()
		fmt.Println("系统代理已清除")
	}()

	// Wait for capture with skip channel
	skipCapture := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			close(skipCapture)
		default:
			var buf [1]byte
			os.Stdin.Read(buf[:])
			close(skipCapture)
		}
	}()

	if err := waitForCapture(ctx, tokens, skipCapture); err != nil {
		return nil, err
	}

	// Prompt phone number if not captured
	tokens.mu.Lock()
	phone := tokens.PhoneNumber
	tokens.mu.Unlock()
	if phone == "" {
		fmt.Print("\n请输入手机号: ")
		input := readInput()
		tokens.mu.Lock()
		tokens.PhoneNumber = input
		tokens.mu.Unlock()
	}

	return tokens, nil
}

func tryLoadConfig() (*CapturedTokens, bool) {
	tokens, err := loadLocalConfig()
	if err != nil {
		return nil, false
	}
	if err := tokens.validateForQuery(); err != nil {
		logMessage(time.Now(), "已保存配置不可用: "+err.Error())
		return nil, false
	}
	logMessage(time.Now(), "使用已保存的配置")
	logMessage(time.Now(), fmt.Sprintf("  手机号: %s", maskPhone(tokens.PhoneNumber)))
	logMessage(time.Now(), fmt.Sprintf("  门店: %v", tokens.StoreIDs))
	return tokens, true
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	if isHTTPStatus(err, http.StatusUnauthorized) || isHTTPStatus(err, http.StatusForbidden) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 401") ||
		strings.Contains(msg, "HTTP 403")
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func runBookingLoop(ctx context.Context, client *Client, settings Settings, storeIDs []string, prefs UserPreferences) {
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
		for _, storeID := range storeIDs {
			slots, err := client.GetTimeslots(ctx, storeID)
			if err != nil {
				if isAuthError(err) {
					authErrors++
					if authErrors >= 3 {
						logMessage(now, "认证失败，请重新运行获取新参数")
						sendNotification("寿司郎 - 认证失败", "认证参数已失效，请重新打开 sushiro-overdose 重新捕获")
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

			// Record slot history
			appendHistory(slots, storeID)

			for i := range slots {
				if !prefs.ShouldTarget(slots[i], settings.Location) {
					continue
				}
				// Skip non-available slots
				if strings.ToUpper(slots[i].Availability) != "AVAILABLE" {
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
				if best == nil || prefs.PreferTargetSlot(*t, *best, settings.Location, storeIDs) {
					best = t
				}
			}
		}

		if best == nil {
			fmt.Printf("\r[%s] 查询中...无目标时段", now.Format("15:04:05"))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		slotLabel := formatSlotWindow(best.Date, best.Start, best.End, settings.Location)
		fmt.Printf("\r[%s] %s - 尝试预约...", now.Format("15:04:05"), slotLabel)

		reservation, err := client.CreateReservation(ctx, best.StoreID, best.Date, best.Start)
		if err != nil {
			if isAuthError(err) {
				authErrors++
				if authErrors >= 3 {
					logMessage(now, "认证失败，请重新运行")
					sendNotification("寿司郎 - 认证失败", "请重新打开 sushiro-overdose 重新捕获")
					deleteLocalConfig()
					return
				}
			}

			if isHTTPStatus(err, http.StatusInternalServerError) {
				logMessage(now, "预约接口 HTTP 500，参数可能已失效，请重新进入小程序获取")
				sendNotification("寿司郎 - HTTP 500", "参数可能已失效，请重新进入小程序刷新并运行 `sushiro run`")
				deleteLocalConfig()
				return
			} else if errors.Is(err, errNoReservationAvailable) {
				key := best.StoreID + best.Date + best.Start
				if booked == nil {
					booked = make(map[string]bool)
				}
				booked[key] = true
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
		onBookingSuccess(reservation, storeName, storeInfo.Address, slotLabel, "预约")

		return
	}
}
