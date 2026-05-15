package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"
)

type slotRecommendation struct {
	Date         string
	Start        string
	End          string
	StoreID      string
	AvailRate    float64
	Observations int
}

func cmdRecommend() {
	printBanner()

	snapshots, err := loadHistory()
	if err != nil || len(snapshots) == 0 {
		fmt.Println("暂无历史数据，无法推荐。")
		fmt.Println("运行 sushiro 一段时间后会自动收集数据，之后可使用 sushiro recommend")
		return
	}

	fmt.Println("\n=== 智能推荐 ===")
	fmt.Printf("基于 %d 条历史记录分析\n\n", len(snapshots))

	// Group by (date, start) → availability rate
	type key struct {
		Date    string
		Start   string
		End     string
		StoreID string
	}
	stats := map[key]*slotRecommendation{}

	for _, s := range snapshots {
		k := key{Date: s.Date, Start: s.Start, End: s.End, StoreID: s.StoreID}
		if _, ok := stats[k]; !ok {
			stats[k] = &slotRecommendation{
				Date:    s.Date,
				Start:   s.Start,
				End:     s.End,
				StoreID: s.StoreID,
			}
		}
		stats[k].Observations++
		if s.Availability == "AVAILABLE" {
			stats[k].AvailRate++
		}
	}

	// Calculate rates
	var recs []slotRecommendation
	for _, r := range stats {
		r.AvailRate = r.AvailRate / float64(r.Observations) * 100
		recs = append(recs, *r)
	}

	// Filter: only future dates, at least 3 observations
	loc := getWebSettings().Location
	if loc == nil {
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	now := time.Now().In(loc)

	validRecs := make([]slotRecommendation, 0)
	for _, r := range recs {
		if r.Observations < 3 {
			continue
		}
		slotTime, err := slotDateTime(Slot{Date: r.Date, Start: r.Start}, now.Location())
		if err != nil || slotTime.Before(now) {
			continue
		}
		validRecs = append(validRecs, r)
	}

	if len(validRecs) == 0 {
		fmt.Println("当前无足够的历史数据来推荐未来时段")
		fmt.Println("建议多运行几天后再次查看")
		return
	}

	// Sort by availability rate desc, then observations desc
	sort.Slice(validRecs, func(i, j int) bool {
		if validRecs[i].AvailRate != validRecs[j].AvailRate {
			return validRecs[i].AvailRate > validRecs[j].AvailRate
		}
		return validRecs[i].Observations > validRecs[j].Observations
	})

	// Display recommendations
	fmt.Println("推荐时段（按可预约概率排序）：")
	fmt.Println()

	// Load client for store info
	tokens, ok := tryLoadConfig()
	if ok {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		settings := tokens.toSettings()
		client := NewClient(settings)
		reg := GetStoreRegistry()

		limit := 10
		if len(validRecs) < limit {
			limit = len(validRecs)
		}

		for i := 0; i < limit; i++ {
			r := validRecs[i]
			storeInfo, _ := client.GetStoreInfo(ctx, r.StoreID)
			storeName := reg.DisplayName(r.StoreID, storeInfo.Name)

			confidence := "低"
			if r.AvailRate >= 70 {
				confidence = "高"
			} else if r.AvailRate >= 40 {
				confidence = "中"
			}

			fmt.Printf("  %d. %s %s-%s\n", i+1,
				r.Date, formatCompactTime(r.Start), formatCompactTime(defaultString(r.End, r.Start)))
			fmt.Printf("     门店: %s | 可预约概率: %.0f%% | 置信度: %s\n",
				storeName, r.AvailRate, confidence)
			fmt.Printf("     基于 %d 次观察\n\n", r.Observations)
		}
	} else {
		// Fallback without client
		for i, r := range validRecs {
			if i >= 10 {
				break
			}
			fmt.Printf("  %d. %s %s-%s — %.0f%% (%d次)\n",
				i+1, r.Date, formatCompactTime(r.Start),
				formatCompactTime(defaultString(r.End, r.Start)),
				r.AvailRate, r.Observations)
		}
	}
}
