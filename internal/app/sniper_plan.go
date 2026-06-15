package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const sniperWindow = 3 * time.Minute

// sniperPlanMu 串行化「读-改-写」整段操作：engine runSniper goroutine 每 50ms
// 调 UpdateSniperPlanTarget，UI 保存/handleSniperPlan 也写 plan，并发时会出现
// lost-update（刚写 done 的 target 被 UI 的旧 plan 覆盖）。只保护 Update/Stop
// 这两个复合操作；单独的 Load/Save 不持锁，Save 用 atomicWriteFile 保证写原子。
var sniperPlanMu sync.Mutex

type SniperPlanState struct {
	UpdatedAt string             `json:"updated_at"`
	Targets   []SniperPlanTarget `json:"targets"`
}

type SniperPlanTarget struct {
	ID               string `json:"id"`
	Date             string `json:"date"`
	StartAfter       string `json:"start_after"`
	StartBefore      string `json:"start_before"`
	StoreID          string `json:"store_id"`
	OpenAt           string `json:"open_at"`
	Status           string `json:"status"`
	CountdownSeconds int64  `json:"countdown_seconds"`
	Attempts         int    `json:"attempts"`
	LastError        string `json:"last_error,omitempty"`
	LastAttemptAt    string `json:"last_attempt_at,omitempty"`
	CompletedAt      string `json:"completed_at,omitempty"`
}

func NormalizeSniperPlan(targets []SniperTarget, loc *time.Location) SniperPlanState {
	out := SniperPlanState{
		UpdatedAt: time.Now().In(loc).Format(time.RFC3339),
		Targets:   make([]SniperPlanTarget, 0, len(targets)),
	}
	seen := map[string]bool{}
	for _, target := range targets {
		target.Date = strings.TrimSpace(target.Date)
		target.StartAfter = NormalizeTimeStr(target.StartAfter)
		target.StartBefore = NormalizeTimeStr(target.StartBefore)
		target.StoreID = strings.TrimSpace(target.StoreID)
		if target.Date == "" || target.StartAfter == "" || target.StartBefore == "" || target.StoreID == "" {
			continue
		}
		item := sniperPlanTargetFromTarget(target, loc)
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out.Targets = append(out.Targets, item)
	}
	sortSniperPlanTargets(out.Targets)
	return out
}

func sniperPlanTargetFromTarget(target SniperTarget, loc *time.Location) SniperPlanTarget {
	openAt := sniperOpenTime(target, loc)
	id := strings.Join([]string{target.StoreID, target.Date, target.StartAfter, target.StartBefore}, ":")
	item := SniperPlanTarget{
		ID:          id,
		Date:        target.Date,
		StartAfter:  target.StartAfter,
		StartBefore: target.StartBefore,
		StoreID:     target.StoreID,
	}
	if !openAt.IsZero() {
		item.OpenAt = openAt.Format(time.RFC3339)
	}
	return item
}

func LoadSniperPlan(loc *time.Location) (SniperPlanState, error) {
	data, err := os.ReadFile(sniperConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return SniperPlanState{}, nil
		}
		return SniperPlanState{}, err
	}

	var state SniperPlanState
	if err := json.Unmarshal(data, &state); err == nil && (state.UpdatedAt != "" || state.Targets != nil) {
		state.Targets = normalizeLoadedPlanTargets(state.Targets, loc)
		sortSniperPlanTargets(state.Targets)
		return RefreshSniperPlan(state, time.Now().In(loc), loc), nil
	}

	var legacy []SniperTarget
	if err := json.Unmarshal(data, &legacy); err != nil {
		return SniperPlanState{}, err
	}
	return RefreshSniperPlan(NormalizeSniperPlan(legacy, loc), time.Now().In(loc), loc), nil
}

func SaveSniperPlan(state SniperPlanState, loc *time.Location) error {
	state.UpdatedAt = time.Now().In(loc).Format(time.RFC3339)
	state.Targets = normalizeLoadedPlanTargets(state.Targets, loc)
	sortSniperPlanTargets(state.Targets)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(AppDirPath(), 0o755)
	// 原子写：避免并发读读到半截 JSON。
	return atomicWriteFile(sniperConfigPath(), data, 0o600)
}

func RefreshSniperPlan(state SniperPlanState, now time.Time, loc *time.Location) SniperPlanState {
	state.Targets = normalizeLoadedPlanTargets(state.Targets, loc)
	for i := range state.Targets {
		state.Targets[i] = refreshSniperPlanTarget(state.Targets[i], now, loc)
	}
	sortSniperPlanTargets(state.Targets)
	return state
}

func refreshSniperPlanTarget(target SniperPlanTarget, now time.Time, loc *time.Location) SniperPlanTarget {
	openAt := parsePlanOpenAt(target, loc)
	if !openAt.IsZero() {
		target.OpenAt = openAt.Format(time.RFC3339)
	}
	if target.CompletedAt != "" {
		target.Status = "done"
		target.CountdownSeconds = 0
		return target
	}
	if target.Status == "stopped" {
		target.CountdownSeconds = 0
		return target
	}
	if target.Status == "running" {
		if !openAt.IsZero() && !now.Before(openAt.Add(sniperWindow)) {
			target.Status = "expired"
			target.CountdownSeconds = 0
			if target.LastError == "" {
				target.LastError = "开放窗口内未预约成功"
			}
			return target
		}
		target.CountdownSeconds = maxInt64(0, int64(openAt.Sub(now).Seconds()))
		return target
	}
	if target.LastError != "" && target.Status == "error" {
		target.CountdownSeconds = maxInt64(0, int64(openAt.Sub(now).Seconds()))
		return target
	}
	if openAt.IsZero() {
		target.Status = "error"
		target.LastError = "无效开放时间"
		target.CountdownSeconds = 0
		return target
	}
	if now.Before(openAt) {
		target.Status = "pending"
		target.CountdownSeconds = int64(openAt.Sub(now).Seconds())
		return target
	}
	if now.Before(openAt.Add(sniperWindow)) {
		target.Status = "open"
		target.CountdownSeconds = 0
		return target
	}
	target.Status = "expired"
	target.CountdownSeconds = 0
	return target
}

func UpdateSniperPlanTarget(targetID string, loc *time.Location, update func(*SniperPlanTarget)) {
	sniperPlanMu.Lock()
	defer sniperPlanMu.Unlock()
	state, err := LoadSniperPlan(loc)
	if err != nil {
		return
	}
	for i := range state.Targets {
		if state.Targets[i].ID == targetID {
			update(&state.Targets[i])
			break
		}
	}
	_ = SaveSniperPlan(state, loc)
}

func StopRemainingSniperPlanTargetsAfterSuccess(doneTargetID string, loc *time.Location) {
	sniperPlanMu.Lock()
	defer sniperPlanMu.Unlock()
	state, err := LoadSniperPlan(loc)
	if err != nil {
		return
	}
	now := time.Now().In(loc).Format(time.RFC3339)
	changed := false
	for i := range state.Targets {
		target := &state.Targets[i]
		if target.ID == doneTargetID || target.CompletedAt != "" || target.Status == "done" || target.Status == "expired" {
			continue
		}
		switch strings.TrimSpace(target.Status) {
		case "", "pending", "open", "running":
			target.Status = "stopped"
			target.CountdownSeconds = 0
			target.LastError = "已因预约成功停止"
			target.LastAttemptAt = now
			changed = true
		}
	}
	if changed {
		_ = SaveSniperPlan(state, loc)
	}
}

func normalizeLoadedPlanTargets(targets []SniperPlanTarget, loc *time.Location) []SniperPlanTarget {
	out := make([]SniperPlanTarget, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		target.Date = strings.TrimSpace(target.Date)
		target.StartAfter = NormalizeTimeStr(target.StartAfter)
		target.StartBefore = NormalizeTimeStr(target.StartBefore)
		target.StoreID = strings.TrimSpace(target.StoreID)
		if target.ID == "" {
			target.ID = strings.Join([]string{target.StoreID, target.Date, target.StartAfter, target.StartBefore}, ":")
		}
		if target.ID == "" || seen[target.ID] {
			continue
		}
		if target.OpenAt == "" {
			openAt := sniperOpenTime(SniperTarget{
				Date:        target.Date,
				StartAfter:  target.StartAfter,
				StartBefore: target.StartBefore,
				StoreID:     target.StoreID,
			}, loc)
			if !openAt.IsZero() {
				target.OpenAt = openAt.Format(time.RFC3339)
			}
		}
		seen[target.ID] = true
		out = append(out, target)
	}
	return out
}

func parsePlanOpenAt(target SniperPlanTarget, loc *time.Location) time.Time {
	if target.OpenAt != "" {
		if t, err := time.Parse(time.RFC3339, target.OpenAt); err == nil {
			return t.In(loc)
		}
	}
	return sniperOpenTime(SniperTarget{
		Date:        target.Date,
		StartAfter:  target.StartAfter,
		StartBefore: target.StartBefore,
		StoreID:     target.StoreID,
	}, loc)
}

func sortSniperPlanTargets(targets []SniperPlanTarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].OpenAt != targets[j].OpenAt {
			return targets[i].OpenAt < targets[j].OpenAt
		}
		return targets[i].ID < targets[j].ID
	})
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
