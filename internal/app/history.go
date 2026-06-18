package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const historyUnchangedWriteInterval = 10 * time.Minute

// historyMaxLines 是 history.jsonl 的行数上限。超过则按时间保留最近的记录并 rewrite，
// 避免 daemon 长期运行导致文件膨胀到数十 MB、loadHistory 全量读入线性变慢拖垮响应。
const historyMaxLines = 50000

// historyTrimInterval：每写多少次检查一次是否需要裁剪，避免每次 append 都触发昂贵的 rewrite。
const historyTrimInterval = 200

type SlotSnapshot struct {
	Timestamp    string `json:"ts"`
	StoreID      string `json:"store_id"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Availability string `json:"availability"`
}

func historyPath() string {
	return filepath.Join(AppDirPath(), "history.jsonl")
}

var (
	lastHistoryWriteByStore     = map[string]time.Time{}
	lastHistorySignatureByStore = map[string]string{}
	historyMu                   sync.Mutex
	historyWriteCounter         = 0
)

func appendHistory(slots []Slot, storeID string) {
	historyMu.Lock()
	defer historyMu.Unlock()

	now := time.Now()
	lastWrite := lastHistoryWriteByStore[storeID]
	signature := historySlotsSignature(slots)
	if now.Sub(lastWrite) < 30*time.Second {
		return
	}
	if signature != "" && signature == lastHistorySignatureByStore[storeID] && now.Sub(lastWrite) < historyUnchangedWriteInterval {
		return
	}

	f, err := os.OpenFile(historyPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// 不更新 lastWrite/signature：写盘失败时若提前更新，会导致后续 30s/10min 内所有写入被
		// 节流分支提前 return，真实数据被持续静默丢弃。这里保留旧状态，下个周期会重试。
		LogMessage(now, "写入历史记录失败（打开文件）："+err.Error())
		return
	}

	ts := now.Format(time.RFC3339)
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
	if err := w.Flush(); err != nil {
		f.Close()
		LogMessage(now, "写入历史记录失败（刷盘）："+err.Error())
		return
	}
	if err := f.Close(); err != nil {
		LogMessage(now, "写入历史记录失败（关闭）："+err.Error())
		return
	}
	// 只有真正写盘成功后才更新「已写入」状态，避免写失败被静默吞掉且后续跳写。
	lastHistoryWriteByStore[storeID] = now
	lastHistorySignatureByStore[storeID] = signature

	// 周期性裁剪，防止文件无限增长。
	historyWriteCounter++
	if historyWriteCounter%historyTrimInterval == 0 {
		trimHistoryLocked(now)
	}
}

// trimHistoryLocked 在文件超过 historyMaxLines 时，保留最近的若干条记录并原子 rewrite。
// 调用方必须已持 historyMu。rewrite 失败只记日志不动原文件（原文件仍完整可读）。
func trimHistoryLocked(now time.Time) {
	orig, err := os.Open(historyPath())
	if err != nil {
		return
	}
	// 允许单行最大 256KB，兜住异常超长行（默认 64KB 遇超长行会让整个 loadHistory 失败）。
	scanner := bufio.NewScanner(orig)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	orig.Close()
	if err := scanner.Err(); err != nil {
		LogMessage(now, "裁剪历史记录时读取失败："+err.Error())
		return
	}
	if len(lines) <= historyMaxLines {
		return
	}
	keep := lines[len(lines)-historyMaxLines:]
	data := make([]byte, 0, len(keep)*128)
	for _, l := range keep {
		data = append(data, l...)
		data = append(data, '\n')
	}
	if err := AtomicWriteFile(historyPath(), data, 0o644); err != nil {
		LogMessage(now, "裁剪历史记录时写入失败："+err.Error())
		return
	}
	LogMessage(now, fmt.Sprintf("历史记录已裁剪：从 %d 条保留最近 %d 条", len(lines), len(keep)))
}

func historySlotsSignature(slots []Slot) string {
	if len(slots) == 0 {
		return ""
	}
	h := fnv.New64a()
	for _, s := range slots {
		h.Write([]byte(s.Date))
		h.Write([]byte{0})
		h.Write([]byte(s.Start))
		h.Write([]byte{0})
		h.Write([]byte(s.End))
		h.Write([]byte{0})
		h.Write([]byte(s.Availability))
		h.Write([]byte{0xff})
	}
	return fmt.Sprintf("%x", h.Sum64())
}

func loadHistory() ([]SlotSnapshot, error) {
	// 加锁与 appendHistory 的写互斥：避免读到正在被追加的最后一行被截断一半的 JSON。
	historyMu.Lock()
	defer historyMu.Unlock()

	f, err := os.Open(historyPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var snapshots []SlotSnapshot
	// 允许单行最大 256KB，兜住异常超长行（默认 64KB 会 ErrTooLong 让整个历史分析失败）。
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
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
		StoreID string
		Date    string
		Start   string
		End     string
	}
	slotStats := map[slotKey]*struct {
		total int
		avail int
		last  string
	}{}

	for _, s := range snapshots {
		key := slotKey{StoreID: s.StoreID, Date: s.Date, Start: s.Start, End: s.End}
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
		if keys[i].StoreID != keys[j].StoreID {
			return keys[i].StoreID < keys[j].StoreID
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
		fmt.Printf("  [%s] %s-%s %s %5.1f%% 可用 (%d/%d)\n",
			k.StoreID,
			FormatCompactTime(k.Start),
			FormatCompactTime(k.End),
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
			fmt.Printf("  %d. %s %s %s — %.0f%% 可用 (%d次观察)\n",
				i+1, r.key.StoreID, r.key.Date, FormatCompactTime(r.key.Start),
				r.rate*100, r.total)
		}
	}
}
