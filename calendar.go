package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

func cmdCalendar() {
	printBanner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokens, ok := tryLoadConfig()
	if !ok {
		fmt.Println("暂无配置，请先运行 sushiro")
		return
	}
	settings := tokens.toSettings()
	client := NewClient(settings)

	logMessage(time.Now(), "验证认证参数...")
	if _, err := client.GetTimeslots(ctx, settings.StoreIDs[0]); err != nil {
		logMessage(time.Now(), "验证失败: "+err.Error())
		fmt.Println("认证参数已过期，请重新运行 sushiro")
		return
	}

	// Fetch timeslots concurrently
	type result struct {
		storeID string
		slots   []Slot
		err     error
	}
	results := make(chan result, len(settings.StoreIDs))
	for _, storeID := range settings.StoreIDs {
		storeID := storeID
		go func() {
			slots, err := client.GetTimeslots(ctx, storeID)
			results <- result{storeID: storeID, slots: slots, err: err}
		}()
	}

	allSlots := make([]Slot, 0)
	storeNames := map[string]string{}
	for range settings.StoreIDs {
		r := <-results
		if r.err != nil {
			fmt.Printf("获取门店 %s 时段失败: %s\n", r.storeID, r.err)
			continue
		}
		info, _ := client.GetStoreInfo(ctx, r.storeID)
		storeNames[r.storeID] = info.Name
		allSlots = append(allSlots, r.slots...)
	}

	if len(allSlots) == 0 {
		fmt.Println("未获取到任何时段数据")
		return
	}

	// Only show nearest 7 days
	now := time.Now().In(settings.Location)
	limit := now.AddDate(0, 0, 7)
	recentSlots := make([]Slot, 0)
	for _, s := range allSlots {
		slotTime, err := slotDateTime(s, settings.Location)
		if err != nil || slotTime.After(limit) {
			continue
		}
		recentSlots = append(recentSlots, s)
	}

	if len(recentSlots) == 0 {
		fmt.Println("近 7 天无可用时段")
		return
	}

	displayCalendar(recentSlots, settings.Location, storeNames)
}

func displayCalendar(slots []Slot, loc *time.Location, storeNames map[string]string) {
	// Group by date
	grouped := map[string][]Slot{}
	for _, s := range slots {
		grouped[s.Date] = append(grouped[s.Date], s)
	}

	dates := make([]string, 0, len(grouped))
	for k := range grouped {
		dates = append(dates, k)
	}
	sort.Strings(dates)

	now := time.Now().In(loc)

	// Print grid overview
	fmt.Println()
	fmt.Println("=== 寿司郎可预约时段日历 ===")
	fmt.Printf("查询时间: %s\n", now.Format("2006-01-02 15:04:05"))

	printSlotCalendarGrid(dates, grouped, loc)

	// Print detailed slot list
	fmt.Println("\n--- 详细时段 ---")
	for _, dateKey := range dates {
		day, err := parseCompactDate(dateKey, loc)
		if err != nil {
			continue
		}
		fmt.Printf("\n%s\n", formatDateWithWeekday(day))

		entries := grouped[dateKey]
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Start < entries[j].Start
		})

		for _, e := range entries {
			icon, label := slotStatusIcon(e.Availability)

			storeTag := ""
			if len(storeNames) > 1 {
				if name := storeNames[e.StoreID]; name != "" {
					storeTag = fmt.Sprintf(" [%s]", name)
				}
			}

			fmt.Printf("  %s-%s %s %s%s\n",
				formatCompactTime(e.Start),
				formatCompactTime(defaultString(e.End, e.Start)),
				icon, label, storeTag)
		}
	}
}

func slotStatusIcon(availability string) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(availability)) {
	case "AVAILABLE":
		return "✓", "可预约"
	case "FULL", "SOLDOUT", "SOLD_OUT":
		return "✗", "已满"
	default:
		if availability != "" && availability != "AVAILABLE" {
			return "●", availability
		}
	}
	return " ", "未知"
}

// printSlotCalendarGrid prints a 7-column calendar overview showing
// date + available slot count per day.
func printSlotCalendarGrid(dates []string, grouped map[string][]Slot, loc *time.Location) {
	if len(dates) == 0 {
		return
	}

	firstDay, _ := parseCompactDate(dates[0], loc)
	lastDay, _ := parseCompactDate(dates[len(dates)-1], loc)

	// Align grid to week boundaries (Monday-based)
	gridStart := beginningOfWeekMon(firstDay)
	gridEnd := lastDay
	for gridEnd.Weekday() != time.Sunday {
		gridEnd = gridEnd.AddDate(0, 0, 1)
	}

	fmt.Println()
	fmt.Println("  周一       周二       周三       周四       周五       周六       周日")
	fmt.Println("  ─────      ─────      ─────      ─────      ─────      ─────      ─────")

	col := 0
	for d := gridStart; !d.After(gridEnd); d = d.AddDate(0, 0, 1) {
		key := d.Format("20060102")
		entries, hasSlots := grouped[key]

		cell := d.Format("1/2")
		if hasSlots {
			avail := 0
			for _, e := range entries {
				if strings.ToUpper(e.Availability) == "AVAILABLE" {
					avail++
				}
			}
			if avail > 0 {
				cell = fmt.Sprintf("%s ✓%d", d.Format("1/2"), avail)
			} else {
				cell = fmt.Sprintf("%s ✗", d.Format("1/2"))
			}
		}

		fmt.Printf("  %-10s", cell)
		col++
		if col%7 == 0 {
			fmt.Println()
		}
	}
	if col%7 != 0 {
		fmt.Println()
	}
}

func beginningOfWeekMon(t time.Time) time.Time {
	d := beginningOfDay(t)
	idx := weekdayIndexMon0(d.Weekday())
	return d.AddDate(0, 0, -idx)
}
