package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type StoreSummary struct {
	Store StoreInfo
	Slots []Slot
}

func buildReservationSuccessCard(reservation ReservationRecord, generatedAt time.Time) map[string]any {
	storeName := fallbackString(reservation.StoreName, "未知门店")
	storeAddress := fallbackString(reservation.StoreAddress, "未提供")
	slotLabel := fallbackString(reservation.SlotLabel, "未提供")
	reservationNumber := fallbackString(reservation.Number, "未提供")

	detailLines := []string{
		fmt.Sprintf("**店名**：%s", storeName),
		fmt.Sprintf("**地址**：%s", storeAddress),
		fmt.Sprintf("**预约时间段**：%s", slotLabel),
		fmt.Sprintf("**预约号码**：`%s`", reservationNumber),
	}
	if strings.TrimSpace(reservation.Status) != "" {
		detailLines = append(detailLines, fmt.Sprintf("**状态**：%s", reservation.Status))
	}

	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "寿司郎预约成功",
			},
			"template": "green",
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("### %s\n预约已成功创建，请及时到店。", storeName),
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": strings.Join(detailLines, "\n"),
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "note",
				"elements": []map[string]any{
					{
						"tag":     "plain_text",
						"content": fmt.Sprintf("通知时间：%s", generatedAt.Format("2006-01-02 15:04:05")),
					},
				},
			},
		},
	}
}

func buildMultiStoreWeekendSummaryCard(storeSummaries []StoreSummary, generatedAt time.Time) map[string]any {
	elements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag": "lark_md",
				"content": strings.Join([]string{
					fmt.Sprintf("**更新时间**：%s", generatedAt.Format("2006-01-02 15:04:05")),
					fmt.Sprintf("**监控门店数**：%d", len(storeSummaries)),
					"**范围**：未来30天周末 `AVAILABLE` 时段",
				}, "\n"),
			},
		},
	}

	if len(storeSummaries) == 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": "当前未配置任何监控门店。",
			},
		})
	} else {
		for _, summary := range storeSummaries {
			elements = append(elements, map[string]any{"tag": "hr"})
			storeName := fallbackString(summary.Store.Name, "未知门店")
			storeAddress := fallbackString(summary.Store.Address, "未提供")
			lines := []string{
				fmt.Sprintf("### %s", storeName),
				fmt.Sprintf("地址：%s", storeAddress),
			}
			grouped := groupSlotsByDate(summary.Slots, generatedAt.Location())
			if len(grouped) == 0 {
				lines = append(lines, "- 未来30天周末暂无可预约的 `AVAILABLE` 时段。")
			} else {
				dates := make([]string, 0, len(grouped))
				for key := range grouped {
					dates = append(dates, key)
				}
				sort.Strings(dates)
				for _, dateKey := range dates {
					lines = append(lines, fmt.Sprintf("- %s：%s", dateKey, strings.Join(grouped[dateKey], "、")))
				}
			}
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": strings.Join(lines, "\n"),
				},
			})
		}
	}

	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "未来30天周末可预约时段",
			},
			"template": "blue",
		},
		"elements": elements,
	}
}

func buildWeekdayEveningConsoleReport(storeSummaries []StoreSummary, now time.Time) string {
	lines := []string{
		"本周工作日晚7点后可预约情况",
		fmt.Sprintf("更新时间：%s", now.Format("2006-01-02 15:04:05")),
	}
	for _, summary := range storeSummaries {
		storeName := fallbackString(summary.Store.Name, "未知门店")
		grouped := groupSlotsByDate(summary.Slots, now.Location())
		if len(grouped) == 0 {
			lines = append(lines, fmt.Sprintf("%s：无", storeName))
			continue
		}
		dates := make([]string, 0, len(grouped))
		for key := range grouped {
			dates = append(dates, key)
		}
		sort.Strings(dates)
		chunks := make([]string, 0, len(dates))
		for _, dateKey := range dates {
			chunks = append(chunks, fmt.Sprintf("%s %s", dateKey, strings.Join(grouped[dateKey], "、")))
		}
		lines = append(lines, fmt.Sprintf("%s：%s", storeName, strings.Join(chunks, "；")))
	}
	return strings.Join(lines, "\n")
}

func groupSlotsByDate(slots []Slot, loc *time.Location) map[string][]string {
	grouped := map[string][]string{}
	for _, slot := range slots {
		day, err := parseCompactDate(slot.Date, loc)
		if err != nil {
			continue
		}
		key := formatDateWithWeekday(day)
		grouped[key] = append(grouped[key], fmt.Sprintf("%s-%s", formatCompactTime(slot.Start), formatCompactTime(defaultString(slot.End, slot.Start))))
	}
	for key := range grouped {
		sort.Strings(grouped[key])
	}
	return grouped
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
