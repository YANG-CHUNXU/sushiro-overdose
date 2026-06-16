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

// sniperWindow 是每个目标的「开放抢约窗口」长度：从放号时刻起 3 分钟内密集轮询抢约，
// 过窗口判 expired。3min 经验值——放号名额通常在 1-2 分钟内被抢光，留余量兜底。
const sniperWindow = 3 * time.Minute

// sniperPlanMu 串行化所有狙击计划写入：engine runSniper goroutine 每 50ms
// 调 UpdateSniperPlanTarget，UI 保存/handleSniperPlan 也写 plan，并发时会出现
// lost-update（刚写 done 的 target 被 UI 的旧 plan 覆盖）。
var sniperPlanMu sync.Mutex

// SniperPlanState 是落盘的狙击计划整体快照（.sushiro_sniper.json），UI 据此渲染计划列表
// 和倒计时。UpdatedAt 用于显示最近更新时间；Targets 已按 OpenAt 排序。
type SniperPlanState struct {
	UpdatedAt string             `json:"updated_at"`
	Targets   []SniperPlanTarget `json:"targets"`
}

// SniperPlanTarget 是计划内单个目标。Status 取值及含义（见 refreshSniperPlanTarget 维护）：
// pending=未到放号时间；open=放号窗口内、未开始抢；running=正在抢约；
// done=已抢到（CompletedAt 非空）；stopped=因别的目标成功被连带停止；
// expired=放号窗口已过未抢到；error=配置无效（如开放时间解析失败）。
// CountdownSeconds 是 UI 倒计时用，由 RefreshSniperPlan 按 OpenAt-now 实时算，不持久依赖。
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

// NormalizeSniperPlan 把原始 SniperTarget 列表规整成可落盘的 SniperPlanState：
// 去空白、规整时间格式、按 ID 去重（ID = 门店:日期:StartAfter:StartBefore）、按 OpenAt 排序。
// 不带运行时状态（attempts/last_error 等），纯「计划骨架」。
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

// LoadSniperPlan 读盘并刷新计划状态。兼容历史迁移：先按新格式（SniperPlanState）解析，
// 失败则按旧格式（裸 []SniperTarget 数组）解析后 NormalizeSniperPlan 升级——老版本配置文件
// 存的就是裸 targets 数组。读出后统一经 RefreshSniperPlan 重算各 target 的 status/倒计时。
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
	sniperPlanMu.Lock()
	defer sniperPlanMu.Unlock()
	return saveSniperPlanUnlocked(state, loc)
}

func saveSniperPlanUnlocked(state SniperPlanState, loc *time.Location) error {
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

// SaveSniperPlanReplacingTargets 用新 targets 重建计划骨架，但保留旧计划里同 ID 目标的
// 运行时状态（status/attempts/last_error/completed_at 等）——这样用户在 UI 改偏好/增删目标
// 时不会把正在跑或已成功的历史记录抹掉。整个读-改-写在 sniperPlanMu 内，避免与 runSniper
// 的 UpdateSniperPlanTarget 并发产生 lost-update。
func SaveSniperPlanReplacingTargets(targets []SniperTarget, loc *time.Location) (SniperPlanState, error) {
	sniperPlanMu.Lock()
	defer sniperPlanMu.Unlock()

	current, err := LoadSniperPlan(loc)
	if err != nil {
		return SniperPlanState{}, err
	}
	runtimeByID := map[string]SniperPlanTarget{}
	for _, target := range current.Targets {
		if target.ID != "" && sniperPlanTargetHasRuntimeState(target) {
			runtimeByID[target.ID] = target
		}
	}

	next := NormalizeSniperPlan(targets, loc)
	for i := range next.Targets {
		if runtime, ok := runtimeByID[next.Targets[i].ID]; ok {
			mergeSniperPlanRuntimeState(&next.Targets[i], runtime)
		}
	}
	if err := saveSniperPlanUnlocked(next, loc); err != nil {
		return SniperPlanState{}, err
	}
	return RefreshSniperPlan(next, time.Now().In(loc), loc), nil
}

// sniperPlanTargetHasRuntimeState 判断 target 是否携带运行时状态（已跑过/有结果），
// 用于 SaveSniperPlanReplacingTargets 决定要不要把它的状态迁移到新计划里。
// 纯 pending/open（未开始执行）不算运行时状态，可被新计划直接覆盖。
func sniperPlanTargetHasRuntimeState(target SniperPlanTarget) bool {
	if target.Attempts > 0 || target.LastError != "" || target.LastAttemptAt != "" || target.CompletedAt != "" {
		return true
	}
	switch strings.TrimSpace(target.Status) {
	case "running", "done", "error", "stopped", "expired":
		return true
	default:
		return false
	}
}

func mergeSniperPlanRuntimeState(target *SniperPlanTarget, runtime SniperPlanTarget) {
	target.Status = runtime.Status
	target.CountdownSeconds = runtime.CountdownSeconds
	target.Attempts = runtime.Attempts
	target.LastError = runtime.LastError
	target.LastAttemptAt = runtime.LastAttemptAt
	target.CompletedAt = runtime.CompletedAt
	if runtime.OpenAt != "" {
		target.OpenAt = runtime.OpenAt
	}
}

func RefreshSniperPlan(state SniperPlanState, now time.Time, loc *time.Location) SniperPlanState {
	state.Targets = normalizeLoadedPlanTargets(state.Targets, loc)
	for i := range state.Targets {
		state.Targets[i] = refreshSniperPlanTarget(state.Targets[i], now, loc)
	}
	sortSniperPlanTargets(state.Targets)
	return state
}

// refreshSniperPlanTarget 是计划状态机的核心：根据 OpenAt 与当前时间 now 推导每个 target
// 应展示的 status 和 CountdownSeconds。优先级（自上而下，先命中先返回）：
//  1. CompletedAt 非空 → 强制 done（抢到的最终态，不再随时间漂移）。
//  2. stopped → 保持（被连带停止的终态，倒计时清零）。
//  3. running 且已过放号窗口(openAt+sniperWindow) → expired（运行循环异常退出没标 expired 时兜底）。
//  4. running 且在窗口内 → 保持 running，倒计时 = openAt-now（放号前还在等）。
//  5. error 且有 LastError → 保持 error，仍给倒计时（UI 可能想看还剩多久）。
//  6. OpenAt 零值（解析失败）→ error「无效开放时间」。
//  7. now<openAt → pending，倒计时 = openAt-now。
//  8. openAt<=now<openAt+sniperWindow → open（窗口内但运行循环还没把它转 running）。
//  9. 其余 → expired（窗口已过）。
//
// 注意：本函数只算「展示态」，不改运行循环的判断；runSniper 自行按 deadline 收尾。
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

// UpdateSniperPlanTarget 按 ID 找到目标并对它跑 update 回调（改 status/attempts/last_error 等），
// 然后落盘。runSniper 每 50ms 轮询时频繁调用（虽有落盘节流），故全程持 sniperPlanMu，
// 保证和 UI 的保存、SaveSniperPlanReplacingTargets 串行，避免互相覆盖。
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
	_ = saveSniperPlanUnlocked(state, loc)
}

// StopRemainingSniperPlanTargetsAfterSuccess 在某目标抢约成功后，把其余仍可执行的目标
// （pending/open/running 且未完成/未过期）标 stopped。原因：预约系统同一账号通常只允许
// 一个有效预约，继续抢别的目标无意义且浪费请求。doneTargetID 本身和已 done/expired 的不动。
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
		_ = saveSniperPlanUnlocked(state, loc)
	}
}

// normalizeLoadedPlanTargets 把从盘读出的 targets 规整一遍：去空白、规整时间、补 ID（旧文件可能没存）、
// 缺 OpenAt 的现算补上、按 ID 去重。在 Load/Save/Refresh 入口统一调用，保证下游拿到的数据是干净的。
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

// parsePlanOpenAt 取目标的放号时刻：优先用已存的 OpenAt 字段（RFC3339），解析失败或缺空
// 时回退到按 Date/StartAfter 现算（sniperOpenTime）。这样旧文件即使没存 OpenAt 也能恢复。
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
