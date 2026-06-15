package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	netTicketRoutineFile                    = "netticket_routine.json"
	netTicketRoutineDefaultNotifyBeforeMins = 10
	netTicketRoutineRefreshThrottle         = 5 * time.Minute
	netTicketRoutineStatusIdle              = "idle"
	netTicketRoutineStatusArmed             = "armed"
	netTicketRoutineStatusWaitingData       = "waiting_data"
	netTicketRoutineStatusNeedsNotify       = "needs_notify"
	netTicketRoutineStatusMissed            = "missed"
	netTicketRoutineStatusNotified          = "notified"
	netTicketRoutineStatusDone              = "done"
)

// NetTicketRoutine 是「每天想几点吃」配置：每天按历史等待倒推取号窗口，并提醒用户手动取号。
type NetTicketRoutine struct {
	Enabled              bool            `json:"enabled"`
	StoreID              string          `json:"store_id"`
	StoreName            string          `json:"store_name,omitempty"`
	TargetMealTime       string          `json:"target_meal_time"`
	TravelMinutes        int             `json:"travel_minutes"`
	NotifyBeforeMinutes  int             `json:"notify_before_minutes"`
	SafetyMinutes        int             `json:"safety_minutes,omitempty"` // legacy: v2.22 used this as auto-ticket safety offset.
	Status               string          `json:"status"`
	PlannedDate          string          `json:"planned_date,omitempty"`
	PlannedPickupTime    string          `json:"planned_pickup_time,omitempty"`
	PlannedPickupEndTime string          `json:"planned_pickup_end_time,omitempty"`
	ReminderTime         string          `json:"reminder_time,omitempty"`
	RecommendPickupRange *QueueTimeRange `json:"recommend_pickup_range,omitempty"`
	WaitMinutesRange     *QueueWaitRange `json:"wait_minutes_range,omitempty"`
	Risk                 string          `json:"risk,omitempty"`
	Basis                string          `json:"basis,omitempty"`
	LastPlannedAt        string          `json:"last_planned_at,omitempty"`
	LastReminderDate     string          `json:"last_reminder_date,omitempty"`
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
		return NetTicketRoutine{Status: netTicketRoutineStatusIdle, NotifyBeforeMinutes: netTicketRoutineDefaultNotifyBeforeMins}
	}
	var r NetTicketRoutine
	if json.Unmarshal(data, &r) != nil {
		return NetTicketRoutine{Status: netTicketRoutineStatusIdle, NotifyBeforeMinutes: netTicketRoutineDefaultNotifyBeforeMins}
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
	if r.NotifyBeforeMinutes == 0 && r.SafetyMinutes > 0 {
		r.NotifyBeforeMinutes = r.SafetyMinutes
	}
	if r.NotifyBeforeMinutes < 0 {
		r.NotifyBeforeMinutes = 0
	}
	if r.NotifyBeforeMinutes == 0 && !r.Enabled && r.TargetMealTime == "" && r.StoreID == "" {
		r.NotifyBeforeMinutes = netTicketRoutineDefaultNotifyBeforeMins
	}
	r.SafetyMinutes = 0
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
	plan := retireRoutineGeneratedNetTicketPlan(LoadNetTicketPlan())
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
	if netTicketPlanSuccessfulOn(plan, now) {
		routine.Status = netTicketRoutineStatusDone
		routine.PlannedDate = today
		routine.LastError = "今天已经取到排队号。"
		routine.LastPlannedAt = now.Format(time.RFC3339)
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	if routineSkipRefresh(routine, now, today) {
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
	}
	routine, plan = planNetTicketRoutineReminderForTodayLocked(routine, plan, now)
	return NetTicketRoutineResponse{Routine: routine, Plan: plan}
}

func planNetTicketRoutineReminderForTodayLocked(routine NetTicketRoutine, existing NetTicketPlan, now time.Time) (NetTicketRoutine, NetTicketPlan) {
	today := now.Format("2006-01-02")
	routine.PlannedDate = today
	routine.LastPlannedAt = now.Format(time.RFC3339)
	mealPlan := buildQueueMealPlan(context.Background(), routine.StoreID, routine.TargetMealTime, routine.TravelMinutes, now, false)
	routine.RecommendPickupRange = mealPlan.RecommendPickupRange
	routine.WaitMinutesRange = mealPlan.WaitMinutesRange
	routine.Risk = mealPlan.Risk
	routine.Basis = mealPlan.Basis

	startRaw, endRaw := routinePickupWindowFromMealPlan(mealPlan)
	if startRaw == "" {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.PlannedPickupTime = ""
		routine.PlannedPickupEndTime = ""
		routine.ReminderTime = ""
		routine.LastError = DefaultString(mealPlan.Message, "这家店历史样本不足，Routine 暂不提醒取号。")
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	start, ok := parseHHMM(startRaw, now)
	if !ok {
		routine.Status = netTicketRoutineStatusWaitingData
		routine.PlannedPickupTime = ""
		routine.PlannedPickupEndTime = ""
		routine.ReminderTime = ""
		routine.LastError = "推算出的取号时间不可用，Routine 暂不提醒取号。"
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	end, ok := parseHHMM(DefaultString(endRaw, startRaw), now)
	if !ok || end.Before(start) {
		end = start
	}
	routine.PlannedPickupTime = start.Format("15:04")
	routine.PlannedPickupEndTime = end.Format("15:04")
	reminderAt := start.Add(-time.Duration(routine.NotifyBeforeMinutes) * time.Minute)
	routine.ReminderTime = reminderAt.Format("15:04")

	if !routineNotifyConfigured() {
		routine.Status = netTicketRoutineStatusNeedsNotify
		routine.LastError = "启用 Routine 前必须先配置通知渠道；否则无法提醒你取号。"
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	if routine.LastReminderDate == today {
		routine.Status = netTicketRoutineStatusNotified
		routine.LastError = ""
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	if now.After(end) {
		routine.Status = netTicketRoutineStatusMissed
		routine.LastError = "今天推算出的取号提醒窗口已过，Routine 明天再提醒。"
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}
	if !now.Before(reminderAt) {
		sendRoutineTakeTicketReminder(context.Background(), routine)
		routine.LastReminderDate = today
		routine.Status = netTicketRoutineStatusNotified
		routine.LastError = ""
		_ = SaveNetTicketRoutine(routine)
		return routine, existing
	}

	routine.Status = netTicketRoutineStatusArmed
	routine.LastError = ""
	_ = SaveNetTicketRoutine(routine)
	return routine, existing
}

func routinePickupWindowFromMealPlan(plan QueueMealPlan) (string, string) {
	if plan.RecommendPickupRange != nil && strings.TrimSpace(plan.RecommendPickupRange.Early) != "" {
		return strings.TrimSpace(plan.RecommendPickupRange.Early), strings.TrimSpace(plan.RecommendPickupRange.Late)
	}
	if strings.TrimSpace(plan.StablePickup) != "" {
		return strings.TrimSpace(plan.StablePickup), strings.TrimSpace(plan.LatestPickup)
	}
	return strings.TrimSpace(plan.LatestPickup), strings.TrimSpace(plan.LatestPickup)
}

func sendRoutineTakeTicketReminder(ctx context.Context, routine NetTicketRoutine) {
	store := DefaultString(routine.StoreName, routine.StoreID)
	window := routine.PlannedPickupTime
	if routine.PlannedPickupEndTime != "" && routine.PlannedPickupEndTime != routine.PlannedPickupTime {
		window += "-" + routine.PlannedPickupEndTime
	}
	wait := ""
	if routine.WaitMinutesRange != nil {
		wait = "，预计等待 " + strconvItoa(routine.WaitMinutesRange.Low) + "-" + strconvItoa(routine.WaitMinutesRange.High) + " 分钟"
	}
	sendQueueAlert(ctx,
		"🍣 该取号了",
		store+"：建议在 "+window+" 之间取号，目标 "+displayTrendTime(routine.TargetMealTime)+" 吃"+wait+"。这只是提醒，不会自动向寿司郎取号。")
}

func routineNotifyConfigured() bool {
	return len(configuredNotificationChannels()) > 0
}

func netTicketPlanSuccessfulOn(plan NetTicketPlan, now time.Time) bool {
	if !netTicketPlanFiredOn(plan, now) {
		return false
	}
	switch strings.TrimSpace(plan.Status) {
	case "success", "issued_unknown":
		return true
	default:
		return strings.TrimSpace(plan.Number) != "" || plan.TicketID != 0
	}
}

func retireRoutineGeneratedNetTicketPlan(plan NetTicketPlan) NetTicketPlan {
	if plan.Source != netTicketPlanSourceRoutine || netTicketPlanTerminal(plan.Status) {
		return plan
	}
	plan.Enabled = false
	plan.Source = ""
	plan.TargetMealTime = ""
	plan.RoutinePlannedDate = ""
	plan.Status = "idle"
	plan.LastError = "Routine 已改为提醒取号，不再自动提交取号。"
	if err := SaveNetTicketPlan(plan); err != nil {
		LogMessage(time.Now(), "保存排队号计划失败: "+err.Error())
	}
	return plan
}

func routineSkipRefresh(r NetTicketRoutine, now time.Time, today string) bool {
	if r.PlannedDate == today {
		switch r.Status {
		case netTicketRoutineStatusMissed, netTicketRoutineStatusNotified, netTicketRoutineStatusDone:
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
	routine.PlannedPickupEndTime = ""
	routine.ReminderTime = ""
	routine.RecommendPickupRange = nil
	routine.WaitMinutesRange = nil
	routine.Risk = ""
	routine.Basis = ""
	routine.LastError = ""
	routine.LastPlannedAt = ""
	plan := retireRoutineGeneratedNetTicketPlan(LoadNetTicketPlan())
	if !routine.Enabled {
		routine.Status = netTicketRoutineStatusIdle
		_ = SaveNetTicketRoutine(routine)
		return NetTicketRoutineResponse{Routine: routine, Plan: plan}
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

func strconvItoa(v int) string {
	return strconv.Itoa(v)
}
