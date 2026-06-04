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
	tmp := historyInsightsCachePath() + ".tmp"
	if os.WriteFile(tmp, data, 0o600) != nil {
		return
	}
	if os.Rename(tmp, historyInsightsCachePath()) != nil {
		_ = os.Remove(tmp)
	}
}
