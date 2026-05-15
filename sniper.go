package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const sniperConfigFile = ".sushiro_sniper.json"

func sniperConfigPath() string {
	return filepath.Join(appDirPath(), sniperConfigFile)
}

// SniperTarget represents a pre-configured reservation target.
type SniperTarget struct {
	Date        string `json:"date"`         // "20260503"
	StartAfter  string `json:"start_after"`  // "193000" earliest acceptable start time
	StartBefore string `json:"start_before"` // "203000" latest acceptable start time
	StoreID     string `json:"store_id"`     // target store
}

func cmdSniper(args []string) {
	printBanner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Parse CLI args
	var (
		dateStr  string
		timeStr  string
		storeStr string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case (args[i] == "--date" || args[i] == "-d") && i+1 < len(args):
			i++
			dateStr = args[i]
		case (args[i] == "--time" || args[i] == "-t") && i+1 < len(args):
			i++
			timeStr = args[i]
		case (args[i] == "--store" || args[i] == "-s") && i+1 < len(args):
			i++
			storeStr = args[i]
		}
	}

	// Load tokens
	tokens, ok := tryLoadConfig()

	// Initialize notifier
	setNotifier(BuildNotifierFromConfig())
	if !ok {
		fmt.Println("暂无配置，请先运行 sushiro-overdose 完成参数捕获")
		return
	}
	if err := tokens.validateForReservation(); err != nil {
		fmt.Println(err)
		fmt.Println("请重新运行 sushiro-overdose 完成参数捕获")
		return
	}
	settings := tokens.toSettings()
	client := NewClient(settings)

	logMessage(time.Now(), "验证认证参数...")
	if _, err := client.GetTimeslots(ctx, settings.StoreIDs[0]); err != nil {
		logMessage(time.Now(), "验证失败: "+err.Error())
		fmt.Println("认证参数已过期，请重新运行 sushiro-overdose 重新捕获")
		return
	}

	var targets []SniperTarget

	if dateStr != "" && timeStr != "" {
		// Quick mode: parse args
		targets = parseSniperArgs(dateStr, timeStr, storeStr, settings.StoreIDs)
	} else {
		// Interactive mode
		var err error
		targets, err = interactiveSniperConfig(ctx, client, tokens)
		if err != nil {
			fmt.Println("配置失败:", err)
			return
		}
	}

	if len(targets) == 0 {
		fmt.Println("无狙击目标")
		return
	}

	// Save config
	saveSniperConfig(targets)

	// Display targets and open times
	now := time.Now().In(settings.Location)
	fmt.Println()
	fmt.Println("=== 狙击目标 ===")
	for _, t := range targets {
		openAt := sniperOpenTime(t, settings.Location)
		status := ""
		if openAt.Before(now) {
			status = " [已开放]"
		} else {
			status = fmt.Sprintf(" [开放于 %s]", openAt.Format("2006-01-02 15:04:05"))
		}
		storeInfo, _ := client.GetStoreInfo(ctx, t.StoreID)
		storeName := storeInfo.Name
		if storeName == "" {
			storeName = t.StoreID
		}
		fmt.Printf("  %s %s-%s @ %s%s\n",
			t.Date, formatCompactTime(t.StartAfter), formatCompactTime(t.StartBefore),
			storeName, status)
	}
	fmt.Println()

	runSniperLoop(ctx, client, settings, targets)
}

func parseSniperArgs(dateStr, timeStr, storeStr string, defaultStores []string) []SniperTarget {
	// Parse dates: "20260503" or "20260503,20260510"
	dates := strings.Split(dateStr, ",")
	for i := range dates {
		dates[i] = strings.TrimSpace(dates[i])
	}

	// Parse time range: "1930-2030"
	parts := strings.SplitN(timeStr, "-", 2)
	startAfter := "000000"
	startBefore := "235959"
	if len(parts) >= 1 {
		if normalized := normalizeTimeStr(parts[0]); parseTimeSeconds(normalized) >= 0 {
			startAfter = normalized
		}
	}
	if len(parts) >= 2 {
		if normalized := normalizeTimeStr(parts[1]); parseTimeSeconds(normalized) >= 0 {
			startBefore = normalized
		}
	}

	// Determine store
	storeID := storeStr
	if storeID == "" && len(defaultStores) > 0 {
		storeID = defaultStores[0]
	}

	targets := make([]SniperTarget, 0, len(dates))
	for _, d := range dates {
		if len(d) != 8 {
			fmt.Printf("忽略无效日期: %s (需要 YYYYMMDD 格式)\n", d)
			continue
		}
		targets = append(targets, SniperTarget{
			Date:        d,
			StartAfter:  startAfter,
			StartBefore: startBefore,
			StoreID:     storeID,
		})
	}
	return targets
}

func interactiveSniperConfig(ctx context.Context, client *Client, tokens *CapturedTokens) ([]SniperTarget, error) {
	settings := tokens.toSettings()

	// Select store
	selectedStores, err := selectStores(ctx, client, tokens)
	if err != nil {
		return nil, fmt.Errorf("选择门店失败: %w", err)
	}
	if len(selectedStores) == 0 {
		return nil, fmt.Errorf("未选择门店")
	}
	storeID := selectedStores[0]

	now := time.Now().In(settings.Location)

	// Show dates from day 8 onwards (not yet fully opened for booking)
	fmt.Println("\n--- 目标日期选择 (第8天起) ---")
	fmt.Println("提示：前7天可直接用 sushiro calendar 查看，这里只展示更远日期")
	days := make([]time.Time, 0)
	for i := 7; i < 31; i++ {
		days = append(days, now.AddDate(0, 0, i))
	}
	printCalendarGrid(days, now)

	fmt.Println()
	fmt.Print("选择日期（输入编号，多个用逗号分隔，如 1,3,7）: ")
	dateInput := readInput()

	validDates := make([]string, 0)
	for _, part := range strings.Split(dateInput, ",") {
		part = strings.TrimSpace(part)
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 1 && idx <= len(days) {
			validDates = append(validDates, days[idx-1].Format("20060102"))
		} else if len(part) == 8 {
			if _, err := parseCompactDate(part, settings.Location); err == nil {
				validDates = append(validDates, part)
			}
		}
	}
	if len(validDates) == 0 {
		return nil, fmt.Errorf("未输入有效日期")
	}

	// Time range
	fmt.Println("\n--- 时间范围 ---")
	fmt.Print("最早可接受时间 (如 1930): ")
	startInput := readInput()
	startAfter := "000000"
	if len(startInput) == 4 {
		startAfter = startInput + "00"
	} else if len(startInput) == 6 {
		startAfter = startInput
	}

	fmt.Print("最晚可接受时间 (如 2030): ")
	endInput := readInput()
	startBefore := "235959"
	if len(endInput) == 4 {
		startBefore = endInput + "00"
	} else if len(endInput) == 6 {
		startBefore = endInput
	}

	targets := make([]SniperTarget, 0, len(validDates))
	for _, d := range validDates {
		targets = append(targets, SniperTarget{
			Date:        d,
			StartAfter:  startAfter,
			StartBefore: startBefore,
			StoreID:     storeID,
		})
	}
	return targets, nil
}

// sniperOpenTime calculates when a target slot opens for booking.
// Rule: slot opens 30 days before the target date, at the same time.
func sniperOpenTime(target SniperTarget, loc *time.Location) time.Time {
	day, err := parseCompactDate(target.Date, loc)
	if err != nil {
		return time.Time{}
	}
	hour, minute, _, _ := parseCompactTime(target.StartAfter)

	// Open time = target date - 30 days, at the target time
	openDay := day.AddDate(0, 0, -30)
	return time.Date(openDay.Year(), openDay.Month(), openDay.Day(), hour, minute, 0, 0, loc)
}

func runSniperLoop(ctx context.Context, client *Client, settings Settings, targets []SniperTarget) {
	doneActivity := markMainFlowActive("sniping")
	defer doneActivity()

	// Sort targets by open time
	sortSniperTargets(targets, settings.Location)

	now := time.Now().In(settings.Location)

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return
		default:
		}

		openAt := sniperOpenTime(target, settings.Location)
		storeInfo, _ := client.GetStoreInfo(ctx, target.StoreID)
		storeName := storeInfo.Name
		if storeName == "" {
			storeName = target.StoreID
		}

		targetLabel := fmt.Sprintf("%s %s-%s @ %s",
			target.Date, formatCompactTime(target.StartAfter),
			formatCompactTime(target.StartBefore), storeName)

		// Skip if too far past
		if openAt.Before(now.Add(-3 * time.Minute)) {
			// Already open, try immediately
			logMessage(now, fmt.Sprintf("目标已开放，立即尝试: %s", targetLabel))
		} else if openAt.After(now) {
			wait := time.Until(openAt)
			logMessage(now, fmt.Sprintf("等待 %v 后开始狙击: %s", wait.Round(time.Second), targetLabel))

			// Sleep until 3 seconds before open time
			if wait > 5*time.Second {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait - 3*time.Second):
				}
			}
		}

		// High-speed polling phase
		deadline := now
		if openAt.After(now) {
			deadline = openAt
		}
		deadline = deadline.Add(3 * time.Minute)

		success := false
		attemptCount := 0

		for time.Now().In(settings.Location).Before(deadline) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			slots, err := client.GetTimeslots(ctx, target.StoreID)
			if err != nil {
				if isAuthError(err) {
					logMessage(time.Now().In(settings.Location), "认证失败，终止狙击")
					sendNotification("寿司郎狙击 - 认证失败", "认证参数已失效")
					return
				}
				if strings.Contains(err.Error(), "500") {
					logMessage(time.Now().In(settings.Location), "HTTP 500，参数已失效，请重新进入小程序获取")
					sendNotification("寿司郎狙击 - HTTP 500", "参数已失效，请重新进入小程序刷新并运行 `sushiro sniper`")
					deleteLocalConfig()
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Find matching slot
			for _, s := range slots {
				if s.Date != target.Date {
					continue
				}
				if s.Start < target.StartAfter || s.Start >= target.StartBefore {
					continue
				}

				attemptCount++
				slotLabel := formatSlotWindow(s.Date, s.Start, defaultString(s.End, s.Start), settings.Location)
				fmt.Printf("\r[%s] 狙击中... %s (第%d次)", time.Now().Format("15:04:05"), slotLabel, attemptCount)

				reservation, err := client.CreateReservation(ctx, target.StoreID, s.Date, s.Start)
				if err != nil {
					if errors.Is(err, errNoReservationAvailable) {
						fmt.Printf("\r[%s] %s - 名额已满，继续尝试...", time.Now().Format("15:04:05"), slotLabel)
					} else if isAuthError(err) {
						logMessage(time.Now().In(settings.Location), "预约认证失败，终止狙击")
						sendNotification("寿司郎狙击 - 认证失败", "预约认证参数已失效")
						deleteLocalConfig()
						return
					} else if isHTTPStatus(err, http.StatusInternalServerError) {
						logMessage(time.Now().In(settings.Location), "预约接口 HTTP 500，参数可能已失效")
						sendNotification("寿司郎狙击 - HTTP 500", "参数可能已失效，请重新捕获")
						deleteLocalConfig()
						return
					}
					continue
				}

				// Success!
				success = true
				now = time.Now().In(settings.Location)
				reservation.MonitoredStoreID = target.StoreID
				onBookingSuccess(reservation, storeName, storeInfo.Address, slotLabel, "狙击")

				return

			}
			time.Sleep(50 * time.Millisecond)
		}

		if !success {
			logMessage(time.Now().In(settings.Location), fmt.Sprintf("目标超时: %s", targetLabel))
		}
	}
}

func sortSniperTargets(targets []SniperTarget, loc *time.Location) {
	type indexed struct {
		target SniperTarget
		openAt time.Time
	}
	items := make([]indexed, len(targets))
	for i, t := range targets {
		items[i] = indexed{target: t, openAt: sniperOpenTime(t, loc)}
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].openAt.Before(items[j-1].openAt); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	for i, item := range items {
		targets[i] = item.target
	}
}

// printCalendarGrid displays dates in a 7-column calendar grid with numbering.
func printCalendarGrid(days []time.Time, now time.Time) {
	if len(days) == 0 {
		return
	}

	// Print weekday header
	fmt.Println()
	fmt.Println("  周一    周二    周三    周四    周五    周六    周日")
	fmt.Println("  ────    ────    ────    ────    ────    ────    ────")

	// Group days by week row
	type dayEntry struct {
		num  int    // 1-based selection number
		date string // formatted display
		day  time.Time
	}
	entries := make([]dayEntry, len(days))
	for i, d := range days {
		entries[i] = dayEntry{num: i + 1, date: d.Format("01/02"), day: d}
	}

	// Determine starting column (Monday=0 ... Sunday=6)
	firstWeekday := weekdayIndexMon0(days[0].Weekday())

	// Print rows
	col := 0
	// Pad first row
	for col < firstWeekday {
		fmt.Print("         ")
		col++
	}

	for _, e := range entries {
		wIdx := weekdayIndexMon0(e.day.Weekday())

		// Fill gaps if we jumped to a new week
		for col%7 != wIdx {
			fmt.Print("         ")
			col++
		}

		tag := " "
		if e.num < 10 {
			tag = "  "
		}
		fmt.Printf("%s%d.%s", tag, e.num, e.date)
		col++
		if col%7 == 0 {
			fmt.Println()
		}
	}
	if col%7 != 0 {
		fmt.Println()
	}
}

func saveSniperConfig(targets []SniperTarget) {
	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(sniperConfigPath(), data, 0o644)
}
