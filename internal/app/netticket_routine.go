package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	netTicketRoutineFile              = "netticket_routine.json"
	netTicketRoutineDefaultSafetyMins = 10
	netTicketRoutineRefreshThrottle   = 5 * time.Minute
	netTicketRoutineAuthReminderLead  = 90 * time.Minute
	netTicketRoutineStatusIdle        = "idle"
	netTicketRoutineStatusArmed       = "armed"
	netTicketRoutineStatusWaitingData = "waiting_data"
	netTicketRoutineStatusBlocked     = "blocked"
	netTicketRoutineStatusMissed      = "missed"
	netTicketRoutineStatusDone        = "done"
	netTicketRoutineStatusNeedsAuth   = "needs_auth"
)

// NetTicketRoutine 是「每天想几点吃」配置：每天按历史等待倒推取号时间，再写入一次性的取号计划。
type NetTicketRoutine struct {
	Enabled              bool            `json:"enabled"`
	StoreID              string          `json:"store_id"`
	StoreName            string          `json:"store_name,omitempty"`
	TargetMealTime       string          `json:"target_meal_time"`
	TravelMinutes        int             `json:"travel_minutes"`
	SafetyMinutes        int             `json:"safety_minutes"`
	Status               string          `json:"status"`
	PlannedDate          string          `json:"planned_date,omitempty"`
	PlannedPickupTime    string          `json:"planned_pickup_time,omitempty"`
	RecommendPickupRange *QueueTimeRange `json:"recommend_pickup_range,omitempty"`
	WaitMinutesRange     *QueueWaitRange `json:"wait_minutes_range,omitempty"`
	Risk                 string          `json:"risk,omitempty"`
	Basis                string          `json:"basis,omitempty"`
	LastPlannedAt        string          `json:"last_planned_at,omitempty"`
	LastAuthReminderDate string          `json:"last_auth_reminder_date,omitempty"`
	LastError            string          `json:"last_error,omitempty"`
}

type NetTicketRoutineResponse struct {
	Routine NetTicketRoutine `json:"routine"`
	Plan    NetTicketPlan    `json:"plan"`
}

func netTicketRoutinePath() string { return filepath.Join(AppDirPath(), netTicketRoutineFile) }

func LoadNetTicketRoutine() NetTicketRoutine {
	data, err := os.ReadFile(netTicketRoutinePath())
	if err != nil {
		return NetTicketRoutine{Status: netTicketRoutineStatusIdle, SafetyMinutes: netTicketRoutineDefaultSafetyMins}
	}
	var r NetTicketRoutine
	if json.Unmarshal(data, &r) != nil {
		return NetTicketRoutine{Status: netTicketRoutineStatusIdle, SafetyMinutes: netTicketRoutineDefaultSafetyMins}
	}
	return normalizeNetTicketRoutine(r)
}

func SaveNetTicketRoutine(r NetTicketRoutine) error {
	os.MkdirAll(AppDirPath(), 0o755)
	r = normalizeNetTicketRoutine(r)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(netTicketRoutinePath(), data, 0o600)
}

func normalizeNetTicketRoutine(r NetTicketRoutine) NetTicketRoutine {
	r.StoreID = strings.TrimSpace(r.StoreID)
	r.StoreName = strings.TrimSpace(r.StoreName)
	r.TargetMealTime = compactRoutineHHMM(r.TargetMealTime)
	if r.TravelMinutes < 0 {
		r.TravelMinutes = 0
	}
	if r.SafetyMinutes < 0 {
		r.SafetyMinutes = 0
	}
	if r.SafetyMinutes == 0 && !r.Enabled && r.TargetMealTime == "" && r.StoreID == "" {
		r.SafetyMinutes = netTicketRoutineDefaultSafetyMins
	}
	if strings.TrimSpace(r.Status) == "" {
		r.Status = netTicketRoutineStatusIdle
	}
	return r
}

func refreshNetTicketRoutineLocked(now time.Time) NetTicketRoutineResponse {
	if now.IsZero() {
		now = time.Now()
	}
	routine := LoadNetTicketRoutine()
	plan := LoadNetTicketPlan()
	if !routine.Enabled {
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if strings.TrimSpace(routine.StoreID) == "" {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.LastError = "Routine 还没有选择门店。"
		routine.LastPlannedAt = now.Format(time.RFC3339)
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if _, ok := parseHHMM(routine.TargetMealTime, now); !ok {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.LastError = "Routine 还没有有效的目标就餐时间。"
		routine.LastPlannedAt = now.Format(time.RFC3339)
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	today := now.Format("2006-01-02")
	if netTicketPlanFiredOn(plan, now) {
		routine.Status = netTicketRoutineStatusDone
		routine.PlannedDate = today
		routine.LastError = "今天已经处理过自动取号。"
		routine.LastPlannedAt = now.Format(time.RFC3339)
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if plan.Enabled && plan.Source != netTicketPlanSourceRoutine && strings.TrimSpace(plan.StoreID) != "" {
		routine.Status = netTicketRoutineStatusBlocked
		routine.LastError = "已有手动自动取号计划，Routine 不会覆盖它。"
		routine.LastPlannedAt = now.Format(time.RFC3339)
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if plan.Enabled && plan.Source == netTicketPlanSourceRoutine && plan.RoutinePlannedDate == today {
		routine.Status = netTicketRoutineStatusArmed
		routine.PlannedDate = today
		routine.PlannedPickupTime = displayTrendTime(plan.TargetTime)
		if ok, reason, _ := authReadyForDailyRoutine(now); !ok {
			plan.Enabled = false
			plan.Status = "idle"
			plan.LastError = reason
			_ = SaveNetTicketPlan(plan)
			routine.Status = netTicketRoutineStatusNeedsAuth
			routine.LastError = reason
			if pickup, ok := netTicketTargetToday(plan.TargetTime, now); ok {
				maybeNotifyRoutineAuthRefresh(&routine, pickup, now, reason)
			}
			_ = SaveNetTicketRoutine(routine)
			return NetTicketRoutineResponse{Routine: routine, Plan: plan}
		}
		if strings.TrimSpace(routine.LastError) != "" {
			routine.LastError = ""
			routine.LastPlannedAt = now.Format(time.RFC3339)
			_ = SaveNetTicketRoutine(routine)
		}
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if routineSkipRefresh(routine, now, today) {
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	routine, plan = planNetTicketRoutineForTodayLocked(routine, plan, now)
	return NetTicketRoutineResponse{Routine: routine, Plan: plan}
}

func planNetTicketRoutineForTodayLocked(routine NetTicketRoutine, existing NetTicketPlan, now time.Time) (NetTicketRoutine, NetTicketPlan) {
	today := now.Format("2006-01-02")
	routine.PlannedDate = today
	routine.LastPlannedAt = now.Format(time.RFC3339)
	mealPlan := buildQueueMealPlan(routine.StoreID, routine.TargetMealTime, routine.TravelMinutes, now)
	routine.RecommendPickupRange = mealPlan.RecommendPickupRange
	routine.WaitMinutesRange = mealPlan.WaitMinutesRange
	routine.Risk = mealPlan.Risk
	routine.Basis = mealPlan.Basis

	pickupRaw := routinePickupFromMealPlan(mealPlan)
	if pickupRaw == "" {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.PlannedPickupTime = ""
		routine.LastError = DefaultString(mealPlan.Message, "这家店历史样本不足，Routine 暂不自动取号。")
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	pickup, ok := parseHHMM(pickupRaw, now)
	if !ok {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.PlannedPickupTime = ""
		routine.LastError = "推算出的取号时间不可用，Routine 暂不自动取号。"
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	if routine.SafetyMinutes > 0 {
		pickup = pickup.Add(-time.Duration(routine.SafetyMinutes) * time.Minute)
	}
	routine.PlannedPickupTime = pickup.Format("15:04")
	if now.After(pickup.Add(netTicketWindowMinutes * time.Minute)) {
		routine.Status = netTicketRoutineStatusMissed
		routine.LastError = "今天推算出的取号时间已过，Routine 明天再自动规划。"
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	if ok, reason, _ := authReadyForDailyRoutine(now); !ok {
		if existing.Source == netTicketPlanSourceRoutine && !netTicketPlanTerminal(existing.Status) {
			existing.Enabled = false
			existing.Status = "idle"
			existing.LastError = reason
			_ = SaveNetTicketPlan(existing)
		}
		routine.Status = netTicketRoutineStatusNeedsAuth
		routine.LastError = reason
		maybeNotifyRoutineAuthRefresh(&routine, pickup, now, reason)
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}

	plan := existing
	plan.Enabled = true
	plan.StoreID = routine.StoreID
	plan.StoreName = routine.StoreName
	plan.TriggerMode = "time"
	plan.TargetTime = pickup.Format("1504")
	plan.Source = netTicketPlanSourceRoutine
	plan.TargetMealTime = routine.TargetMealTime
	plan.RoutinePlannedDate = today
	plan.Status = "armed"
	plan.Number = ""
	plan.TicketID = 0
	plan.FiredDate = ""
	plan.FiredAt = ""
	plan.LastError = ""
	if err := SaveNetTicketPlan(plan); err != nil {
		routine.Status = "error"
		routine.LastError = err.Error()
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	clearNetTicketFire(today)
	routine.Status = netTicketRoutineStatusArmed
	routine.LastError = ""
	_ = SaveNetTicketRoutine(routine)
	return routine, plan
}

func maybeNotifyRoutineAuthRefresh(routine *NetTicketRoutine, pickup, now time.Time, reason string) {
	if routine == nil || pickup.IsZero() {
		return
	}
	today := now.Format("2006-01-02")
	if routine.LastAuthReminderDate == today {
		return
	}
	if now.Before(pickup.Add(-netTicketRoutineAuthReminderLead)) {
		return
	}
	routine.LastAuthReminderDate = today
	store := DefaultString(routine.StoreName, routine.StoreID)
	sendQueueAlert(context.Background(),
		"⚠️ Routine 需要刷新认证",
		store+"：今天预计 "+routine.PlannedPickupTime+" 自动取号，但寿司郎凭证需要今天重新认证。"+DefaultString(reason, "请重新获取凭证后再让 Routine 自动取号。"))
}

func routinePickupFromMealPlan(plan QueueMealPlan) string {
	if strings.TrimSpace(plan.StablePickup) != "" {
		return plan.StablePickup
	}
	if plan.RecommendPickupRange != nil && strings.TrimSpace(plan.RecommendPickupRange.Early) != "" {
		return plan.RecommendPickupRange.Early
	}
	return strings.TrimSpace(plan.LatestPickup)
}

func routineSkipRefresh(r NetTicketRoutine, now time.Time, today string) bool {
	if r.PlannedDate == today {
		switch r.Status {
		case netTicketRoutineStatusMissed, netTicketRoutineStatusDone:
			return true
		}
	}
	if r.Status != netTicketRoutineStatusWaitingData {
		return false
	}
	last, ok := parseRFC3339Local(r.LastPlannedAt)
	return ok && now.Sub(last) < netTicketRoutineRefreshThrottle
}

func saveNetTicketRoutineConfigLocked(routine NetTicketRoutine, now time.Time) NetTicketRoutineResponse {
	routine = normalizeNetTicketRoutine(routine)
	routine.Status = netTicketRoutineStatusIdle
	routine.PlannedDate = ""
	routine.PlannedPickupTime = ""
	routine.RecommendPickupRange = nil
	routine.WaitMinutesRange = nil
	routine.Risk = ""
	routine.Basis = ""
	routine.LastError = ""
	routine.LastPlannedAt = ""
	plan := LoadNetTicketPlan()
	if !routine.Enabled {
		routine.Status = netTicketRoutineStatusIdle
		if plan.Source == netTicketPlanSourceRoutine && !netTicketPlanTerminal(plan.Status) {
			plan.Enabled = false
			plan.Status = "idle"
			plan.LastError = ""
			_ = SaveNetTicketPlan(plan)
		}
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: LoadNetTicketPlan()}
	}
	if plan.Source == netTicketPlanSourceRoutine && !netTicketPlanTerminal(plan.Status) {
		plan.Enabled = false
		plan.Status = "idle"
		plan.LastError = ""
		_ = SaveNetTicketPlan(plan)
	}
	_ = SaveNetTicketRoutine(routine)
	return refreshNetTicketRoutineLocked(now)
}

func compactRoutineHHMM(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ":", ""))
	if len(raw) == 6 && strings.HasSuffix(raw, "00") {
		raw = raw[:4]
	}
	return raw
}
