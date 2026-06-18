package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	historyInsightsCacheFile    = "history_insights_cache.json"
	historyInsightsCacheVersion = 1
)

type slotHistoryInsightsCache struct {
	Version           int                 `json:"version"`
	TopN              int                 `json:"top_n"`
	SourceSize        int64               `json:"source_size"`
	SourceModUnixNano int64               `json:"source_mod_unix_nano"`
	Analysis          SlotHistoryAnalysis `json:"analysis"`
}

func historyInsightsCachePath() string {
	return filepath.Join(AppDirPath(), historyInsightsCacheFile)
}

func loadSlotHistoryInsightsCache(topN int, info os.FileInfo) (SlotHistoryAnalysis, bool) {
	if info == nil {
		return SlotHistoryAnalysis{}, false
	}
	data, err := os.ReadFile(historyInsightsCachePath())
	if err != nil {
		return SlotHistoryAnalysis{}, false
	}
	var cache slotHistoryInsightsCache
	if json.Unmarshal(data, &cache) != nil {
		return SlotHistoryAnalysis{}, false
	}
	if cache.Version != historyInsightsCacheVersion || cache.TopN != topN {
		return SlotHistoryAnalysis{}, false
	}
	if cache.SourceSize != info.Size() || cache.SourceModUnixNano != info.ModTime().UnixNano() {
		return SlotHistoryAnalysis{}, false
	}
	return cache.Analysis, true
}

// atomicWriteFile 先写同目录临时文件再用 os.Rename 原子替换目标。
// os.Rename 在同一目录内是原子的；tmp 必须与目标同目录，否则跨文件系统会退化为非原子 copy。
// 避免 O_TRUNC+write 的截断窗口：同进程并发读会读到半截 JSON 解析失败（如采样循环读到
// Enabled=false 后静默停止）。统一委托给 core.AtomicWriteFile（用 os.CreateTemp 生成唯一
// 临时名，避免并发写同一目标时多个写者争抢固定 tmp 名）。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return AtomicWriteFile(path, data, perm)
}

func saveSlotHistoryInsightsCache(topN int, info os.FileInfo, analysis SlotHistoryAnalysis) {
	if info == nil {
		return
	}
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return
	}
	cache := slotHistoryInsightsCache{
		Version:           historyInsightsCacheVersion,
		TopN:              topN,
		SourceSize:        info.Size(),
		SourceModUnixNano: info.ModTime().UnixNano(),
		Analysis:          analysis,
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = atomicWriteFile(historyInsightsCachePath(), data, 0o600)
}
