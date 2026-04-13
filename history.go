package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type SlotSnapshot struct {
	Timestamp   string `json:"ts"`
	StoreID     string `json:"store_id"`
	Date        string `json:"date"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Availability string `json:"availability"`
}

func historyPath() string {
	return filepath.Join(appDirPath(), "history.jsonl")
}

var (
	lastHistoryWrite time.Time
	historyMu        sync.Mutex
)

func appendHistory(slots []Slot, storeID string) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if time.Since(lastHistoryWrite) < 30*time.Second {
		return
	}
	lastHistoryWrite = time.Now()

	f, err := os.OpenFile(historyPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	w := bufio.NewWriter(f)
	for _, s := range slots {
		snap := SlotSnapshot{
			Timestamp:    ts,
			StoreID:      storeID,
			Date:         s.Date,
			Start:        s.Start,
			End:          s.End,
			Availability: s.Availability,
		}
		data, _ := json.Marshal(snap)
		w.Write(data)
		w.WriteByte('\n')
	}
	w.Flush()
}

func loadHistory() ([]SlotSnapshot, error) {
	f, err := os.Open(historyPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var snapshots []SlotSnapshot
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var snap SlotSnapshot
		if json.Unmarshal(scanner.Bytes(), &snap) == nil {
			snapshots = append(snapshots, snap)
		}
	}
	return snapshots, scanner.Err()
}

func cmdTrends() {
	printBanner()

	snapshots, err := loadHistory()
	if err != nil || len(snapshots) == 0 {
		fmt.Println("暂无历史数据。运行 sushiro 一段时间后会自动记录。")
		return
	}

	fmt.Println("\n=== 时段趋势分析 ===")
	fmt.Printf("历史记录: %d 条\n\n", len(snapshots))

	// Analyze: group by (date, start_time) → availability rate
	type slotKey struct {
		Date  string
		Start string
		End   string
	}
	slotStats := map[slotKey]*struct {
		total int
		avail int
		last  string
	}{}

	for _, s := range snapshots {
		key := slotKey{Date: s.Date, Start: s.Start, End: s.End}
		if _, ok := slotStats[key]; !ok {
			slotStats[key] = &struct {
				total int
				avail int
				last  string
			}{}
		}
		slotStats[key].total++
		if s.Availability == "AVAILABLE" {
			slotStats[key].avail++
		}
		slotStats[key].last = s.Timestamp
	}

	// Sort by date then start time
	keys := make([]slotKey, 0, len(slotStats))
	for k := range slotStats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Date != keys[j].Date {
			return keys[i].Date < keys[j].Date
		}
		return keys[i].Start < keys[j].Start
	})

	// Group by date for display
	currentDate := ""
	for _, k := range keys {
		stats := slotStats[k]
		if k.Date != currentDate {
			currentDate = k.Date
			fmt.Printf("\n%s\n", k.Date)
		}
		rate := float64(stats.avail) / float64(stats.total) * 100
		bar := ""
		blocks := int(rate / 10)
		for i := 0; i < 10; i++ {
			if i < blocks {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		fmt.Printf("  %s-%s %s %5.1f%% 可用 (%d/%d)\n",
			formatCompactTime(k.Start),
			formatCompactTime(k.End),
			bar, rate, stats.avail, stats.total)
	}

	// Summary
	fmt.Println("\n--- 摘要 ---")
	// Find best slots (highest availability)
	type slotRank struct {
		key   slotKey
		rate  float64
		total int
	}
	ranks := make([]slotRank, 0, len(slotStats))
	for k, s := range slotStats {
		if s.total < 3 {
			continue
		}
		ranks = append(ranks, slotRank{key: k, rate: float64(s.avail) / float64(s.total), total: s.total})
	}
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].rate > ranks[j].rate
	})

	if len(ranks) > 0 {
		fmt.Println("\n最容易抢到的时段 TOP 5:")
		for i, r := range ranks {
			if i >= 5 {
				break
			}
			fmt.Printf("  %d. %s %s — %.0f%% 可用 (%d次观察)\n",
				i+1, r.key.Date, formatCompactTime(r.key.Start),
				r.rate*100, r.total)
		}
	}
}
