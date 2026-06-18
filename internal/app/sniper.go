package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/proxy"

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const sniperConfigFile = ".sushiro_sniper.json"

func sniperConfigPath() string {
	return filepath.Join(AppDirPath(), sniperConfigFile)
}

// SniperTarget represents a pre-configured reservation target.
type SniperTarget struct {
	Date        string `json:"date"`         // "20260503"
	StartAfter  string `json:"start_after"`  // "193000" earliest acceptable start time
	StartBefore string `json:"start_before"` // "203000" latest acceptable start time
	StoreID     string `json:"store_id"`     // target store
}

// cmdSniper 是 CLI 子命令 sushiro sniper 的入口：解析参数 → 加载凭证 → 验活 →
// 组目标（快速参数模式或交互式配置）→ 展示开放时间 → 跑 runSniperLoop。
// 与 Web 版 runSniper 的区别：这是 CLI 一次性运行，输出走 stdout/LogMessage，
// 不写 SniperPlan 文件、不广播 SSE。
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
		fmt.Println("暂无配置，请先运行 sushiro 完成参数捕获")
		return
	}
	if err := tokens.ValidateForReservation(); err != nil {
		fmt.Println(err)
		fmt.Println("请重新运行 sushiro 完成参数捕获")
		return
	}
	settings := tokens.ToSettings()
	client := NewClient(settings)

	LogMessage(time.Now(), "验证凭证参数...")
	if _, err := client.GetTimeslots(ctx, settings.StoreIDs[0]); err != nil {
		LogMessage(time.Now(), "验证失败: "+err.Error())
		fmt.Println("凭证参数已过期，请重新运行 sushiro 重新捕获")
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
			t.Date, FormatCompactTime(t.StartAfter), FormatCompactTime(t.StartBefore),
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
		if normalized := NormalizeTimeStr(parts[0]); ParseTimeSeconds(normalized) >= 0 {
			startAfter = normalized
		}
	}
	if len(parts) >= 2 {
		if normalized := NormalizeTimeStr(parts[1]); ParseTimeSeconds(normalized) >= 0 {
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
	settings := tokens.ToSettings()

	// Select store
	selectedStores, err := SelectStores(ctx, client, tokens)
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
	dateInput := ReadInput()

	validDates := make([]string, 0)
	for _, part := range strings.Split(dateInput, ",") {
		part = strings.TrimSpace(part)
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 1 && idx <= len(days) {
			validDates = append(validDates, days[idx-1].Format("20060102"))
		} else if len(part) == 8 {
			if _, err := ParseCompactDate(part, settings.Location); err == nil {
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
	startInput := ReadInput()
	startAfter := "000000"
	if len(startInput) == 4 {
		startAfter = startInput + "00"
	} else if len(startInput) == 6 {
		startAfter = startInput
	}

	fmt.Print("最晚可接受时间 (如 2030): ")
	endInput := ReadInput()
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

// sniperOpenTime 计算某目标时段的「放号时刻」。
// 业务规则：寿司郎预约系统在「目标日期前 30 天的同一时刻」放出该日同时段的可约名额，
// 例如 5/3 的 19:00 时段在 4/3 的 19:00 放号。用目标起始时间（StartAfter）的时分作为放号时分。
// 跨月/跨年用 AddDate(-30天) 自动正确处理；解析失败返回零值，调用方据此判定目标无效。
// 注意：30 天是经验值/官方口径，若官方调整放号周期需同步改这里和 sniperWindow。
func sniperOpenTime(target SniperTarget, loc *time.Location) time.Time {
	day, err := ParseCompactDate(target.Date, loc)
	if err != nil {
		return time.Time{}
	}
	hour, minute, _, err := ParseCompactTime(target.StartAfter)
	if err != nil {
		// 非法 StartAfter（畸形时间串）必须返回零值，否则下游会用 hour=0/minute=0
		// 构造出「目标日期前 30 天 00:00」的非零时间，让无效目标伪装成有效、永久挂 pending/open。
		return time.Time{}
	}

	// Open time = target date - 30 days, at the target time
	openDay := day.AddDate(0, 0, -30)
	return time.Date(openDay.Year(), openDay.Month(), openDay.Day(), hour, minute, 0, 0, loc)
}

// runSniperLoop 是 CLI 版狙击主循环（对应 Web 版 runSniper，但无 SniperPlan 落盘）。
// 按放号时间升序逐目标处理：等到放号前 3s → 开放窗口内（openAt+3min）高频 50ms 轮询抢约。
// 已开放超 3min 的目标视为过期，跳过等待直接试一次；凭证失败立即终止整个循环。
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
			target.Date, FormatCompactTime(target.StartAfter),
			FormatCompactTime(target.StartBefore), storeName)

		// 已开放超 3min（放号窗口已过）的视为过期：不再等，直接试一次碰运气。
		if openAt.Before(now.Add(-3 * time.Minute)) {
			// Already open, try immediately
			LogMessage(now, fmt.Sprintf("目标已开放，立即尝试: %s", targetLabel))
		} else if openAt.After(now) {
			wait := time.Until(openAt)
			LogMessage(now, fmt.Sprintf("等待 %v 后开始狙击: %s", wait.Round(time.Second), targetLabel))

			// 睡到放号前 3s：留余量应对本地与服务端时钟偏差，避免因慢几秒而错过放号瞬间。
			if wait > 5*time.Second {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait - 3*time.Second):
				}
			}
		}

		// High-speed polling phase：开放窗口 [基准点, 基准点+3min]，基准点取 max(openAt, now)。
		deadline := now
		if openAt.After(now) {
			deadline = openAt
		}
		deadline = deadline.Add(3 * time.Minute)

		success := false
		attemptCount := 0
		temporarySkips := map[string]time.Time{}

		for time.Now().In(settings.Location).Before(deadline) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			slots, err := client.GetTimeslots(ctx, target.StoreID)
			if err != nil {
				if isAuthError(err) {
					LogMessage(time.Now().In(settings.Location), "凭证失败，终止狙击")
					sendNotification("寿司郎狙击 - 凭证失败", "凭证参数已失效")
					return
				}
				if isOfficialServerHTTPError(err) {
					LogMessage(time.Now().In(settings.Location), friendlyOfficialAPIError(err))
					// 5xx 退避 200ms，给服务端恢复时间。
					time.Sleep(200 * time.Millisecond)
					continue
				}
				// 普通 GetTimeslots 失败退避 100ms（CLI 版比 Web 版 50ms 略保守）。
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Find matching slot：匹配目标日期 + 起始时间落在 [StartAfter, StartBefore) 半开区间。
			for _, s := range slots {
				if s.Date != target.Date {
					continue
				}
				if s.Start < target.StartAfter || s.Start >= target.StartBefore {
					continue
				}
				key := bookingSlotKey(target.StoreID, s.Date, s.Start)
				if isTemporaryBookingSkipped(temporarySkips, key, time.Now().In(settings.Location)) {
					continue
				}

				attemptCount++
				slotLabel := FormatSlotWindow(s.Date, s.Start, DefaultString(s.End, s.Start), settings.Location)
				fmt.Printf("\r[%s] 狙击中... %s (第%d次)", time.Now().Format("15:04:05"), slotLabel, attemptCount)

				reservation, err := client.CreateReservation(ctx, target.StoreID, s.Date, s.Start)
				if err != nil {
					// 名额已满：不冷却，下一轮继续刷（别人可能取消释放名额）。
					if errors.Is(err, ErrNoReservationAvailable) {
						fmt.Printf("\r[%s] %s - 名额已满，继续尝试...", time.Now().Format("15:04:05"), slotLabel)
					} else if isAuthError(err) {
						// 预约接口凭证失败：判定真失效，删配置并终止整个循环（与 Web 版一致，不重试）。
						LogMessage(time.Now().In(settings.Location), "预约凭证失败，终止狙击")
						sendNotification("寿司郎狙击 - 凭证失败", "预约凭证参数已失效")
						DeleteLocalConfig()
						return
					} else if isOfficialServerHTTPError(err) {
						// 5xx：该时段冷却 30s 再试，避免对抽风时段无脑重试。
						markTemporaryBookingSkip(temporarySkips, key, time.Now().In(settings.Location))
						LogMessage(time.Now().In(settings.Location), bookingServerErrorLog(slotLabel, err))
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
			// 每轮 GetTimeslots 之间 50ms：放号瞬间名额秒空，必须高频刷新才能抢到。
			time.Sleep(50 * time.Millisecond)
		}

		if !success {
			LogMessage(time.Now().In(settings.Location), fmt.Sprintf("目标超时: %s", targetLabel))
		}
	}
}

// sortSniperTargets 按放号时间升序原地排序目标。放号早的先抢，避免错过近的时段。
// 用插入排序：目标数量通常个位数（交互式一般选几条），插入排序在小 n 下比快排开销更低且稳定。
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
	firstWeekday := WeekdayIndexMon0(days[0].Weekday())

	// Print rows
	col := 0
	// Pad first row
	for col < firstWeekday {
		fmt.Print("         ")
		col++
	}

	for _, e := range entries {
		wIdx := WeekdayIndexMon0(e.day.Weekday())

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
