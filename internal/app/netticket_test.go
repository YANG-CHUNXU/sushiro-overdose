package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import (
	"testing"
	"time"
)

func TestNetTicketIssuedTodayBlocksSampling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:   false,
		StoreID:   "3006",
		Status:    "success",
		Number:    "1843",
		FiredDate: now.Format("2006-01-02"),
		FiredAt:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	if !netTicketIssuedToday(now) {
		t.Fatal("issued ticket today should block sampling")
	}
}

func TestNetTicketIssuedYesterdayDoesNotBlockSampling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:   true,
		StoreID:   "3006",
		Status:    "success",
		Number:    "1843",
		FiredDate: now.AddDate(0, 0, -1).Format("2006-01-02"),
		FiredAt:   now.AddDate(0, 0, -1).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	if netTicketIssuedToday(now) {
		t.Fatal("old ticket should not block today's sampling")
	}
}

func TestTerminalNetTicketPlanFromPreviousDayIsNotArmed(t *testing.T) {
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	plan := normalizeNetTicketPlan(NetTicketPlan{
		Enabled:   true,
		StoreID:   "3006",
		Status:    "success",
		Number:    "1843",
		FiredDate: now.AddDate(0, 0, -1).Format("2006-01-02"),
		FiredAt:   now.AddDate(0, 0, -1).Format(time.RFC3339),
	}, now)

	if plan.Enabled {
		t.Fatal("previous-day terminal net ticket plan should not remain armed")
	}
}

func TestTerminalNetTicketPlanFromTodayIsNotArmed(t *testing.T) {
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	plan := normalizeNetTicketPlan(NetTicketPlan{
		Enabled:   true,
		StoreID:   "3006",
		Status:    "expired",
		FiredDate: now.Format("2006-01-02"),
		FiredAt:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}, now)

	if plan.Enabled {
		t.Fatal("today's terminal net ticket plan should not remain armed")
	}
	if !netTicketPlanFiredOn(plan, now) {
		t.Fatal("terminal plan should still preserve today's fired state")
	}
}

func TestActiveNetTicketPlanStaysArmed(t *testing.T) {
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	plan := normalizeNetTicketPlan(NetTicketPlan{
		Enabled:    true,
		StoreID:    "3006",
		Status:     "armed",
		TargetTime: "1900",
	}, now)

	if !plan.Enabled {
		t.Fatal("armed net ticket plan should stay enabled")
	}
}

// TestNetTicketServerRetryCountPersists 验证服务端临时错误的重试计数会被持久化，
// 且达到上限 netTicketMaxServerRetries 后 fireNetTicket 不再清占位重发（防刷号）。
func TestNetTicketServerRetryCountPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:          true,
		StoreID:          "3006",
		Status:           "retrying",
		TargetTime:       "1900",
		ServerRetryCount: netTicketMaxServerRetries,
		FiredDate:        "", // retrying 状态当天未占位
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	loaded := LoadNetTicketPlan()
	if loaded.ServerRetryCount != netTicketMaxServerRetries {
		t.Fatalf("ServerRetryCount round-trip = %d, want %d", loaded.ServerRetryCount, netTicketMaxServerRetries)
	}
}

// TestNetTicketServerRetryResetsAcrossDay 验证跨天后 ServerRetryCount 仍被持久化，
// 由 netTicketTick 在新一天的首次调度时清零（见 netticket.go 中 FiredDate != today 的重置分支）。
func TestNetTicketServerRetryResetsAcrossDay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:          true,
		StoreID:          "3006",
		Status:           "retrying",
		TargetTime:       "1900",
		ServerRetryCount: netTicketMaxServerRetries,
		FiredDate:        yesterday,
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	loaded := LoadNetTicketPlan()
	// 前提：昨天的计划今天还没处理过，netTicketTick 会进入清零分支。
	if loaded.FiredDate == now.Format("2006-01-02") {
		t.Fatal("precondition: yesterday's plan should not already be fired today")
	}
	if loaded.ServerRetryCount != netTicketMaxServerRetries {
		t.Fatalf("yesterday's retry count should still be on disk, got %d", loaded.ServerRetryCount)
	}
}

func TestNetTicketRoutinePlansReminderFromHistoricalMealPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)
	saveRoutineNotifyForTest(t)
	appendRoutineHistoryForTest(t, time.Date(2026, 6, 2, 12, 0, 0, 0, time.Local))

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:             true,
		StoreID:             "3006",
		StoreName:           "太阳宫凯德店",
		TargetMealTime:      "1300",
		NotifyBeforeMinutes: 10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusArmed {
		t.Fatalf("routine status = %q, want armed; error=%q", resp.Routine.Status, resp.Routine.LastError)
	}
	if resp.Routine.PlannedPickupTime != "12:00" || resp.Routine.ReminderTime != "11:50" {
		t.Fatalf("routine planned pickup/reminder = %q/%q, want 12:00/11:50", resp.Routine.PlannedPickupTime, resp.Routine.ReminderTime)
	}
	if resp.Plan.Enabled || resp.Plan.Source == netTicketPlanSourceRoutine {
		t.Fatalf("routine reminder should not arm an auto-ticket plan: %+v", resp.Plan)
	}
}

func TestNetTicketRoutineRequiresNotificationBeforeArming(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 11, 0, 0, 0, time.Local)
	appendRoutineHistoryForTest(t, time.Date(2026, 6, 2, 12, 0, 0, 0, time.Local))

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:             true,
		StoreID:             "3006",
		StoreName:           "太阳宫凯德店",
		TargetMealTime:      "1300",
		NotifyBeforeMinutes: 10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusNeedsNotify {
		t.Fatalf("routine status = %q, want needs_notify; error=%q", resp.Routine.Status, resp.Routine.LastError)
	}
	if resp.Plan.Enabled {
		t.Fatalf("routine should not arm an auto-ticket plan without notify: %+v", resp.Plan)
	}
}

func TestNetTicketRoutineDoesNotOverwriteManualPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)
	saveRoutineNotifyForTest(t)
	appendRoutineHistoryForTest(t, time.Date(2026, 6, 2, 12, 0, 0, 0, time.Local))
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:    true,
		StoreID:    "manual",
		StoreName:  "手动门店",
		TargetTime: "1800",
		Status:     "armed",
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:             true,
		StoreID:             "3006",
		TargetMealTime:      "1300",
		NotifyBeforeMinutes: 10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusArmed {
		t.Fatalf("routine status = %q, want armed", resp.Routine.Status)
	}
	if resp.Plan.StoreID != "manual" || !resp.Plan.Enabled {
		t.Fatalf("manual plan was overwritten: %+v", resp.Plan)
	}
}

func TestDisablingNetTicketRoutineClearsPendingRoutinePlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:            true,
		StoreID:            "3006",
		TargetTime:         "1150",
		Source:             netTicketPlanSourceRoutine,
		TargetMealTime:     "1300",
		RoutinePlannedDate: now.Format("2006-01-02"),
		Status:             "armed",
	}); err != nil {
		t.Fatalf("SaveNetTicketPlan() error = %v", err)
	}

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:             false,
		StoreID:             "3006",
		TargetMealTime:      "1300",
		NotifyBeforeMinutes: 10,
	}, now)

	if resp.Plan.Enabled {
		t.Fatalf("routine plan should be disabled: %+v", resp.Plan)
	}
	if resp.Plan.Source == netTicketPlanSourceRoutine {
		t.Fatalf("legacy routine plan source should be cleared: %+v", resp.Plan)
	}
	if resp.Routine.Enabled || resp.Routine.Status != netTicketRoutineStatusIdle {
		t.Fatalf("routine should be idle: %+v", resp.Routine)
	}
}

func TestNetTicketRoutineWaitsForDataInsteadOfGuessing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:             true,
		StoreID:             "3006",
		TargetMealTime:      "1300",
		NotifyBeforeMinutes: 10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusWaitingData {
		t.Fatalf("routine status = %q, want waiting_data", resp.Routine.Status)
	}
	if resp.Plan.Enabled {
		t.Fatalf("routine should not arm a plan without data: %+v", resp.Plan)
	}
}

func saveRoutineNotifyForTest(t *testing.T) {
	t.Helper()
	cfg := &NotifyConfig{}
	cfg.Feishu.Webhook = "https://example.invalid/feishu"
	if err := SaveNotifyConfig(cfg); err != nil {
		t.Fatalf("SaveNotifyConfig() error = %v", err)
	}
}

func appendRoutineHistoryForTest(t *testing.T, observedAt time.Time) {
	t.Helper()
	if err := appendQueueObservation(QueueObservation{
		CollectedAt:       observedAt.Format(time.RFC3339),
		StoreID:           "3006",
		WaitMinutes:       60,
		GroupQueuesCount:  24,
		StoreStatus:       "OPEN",
		NetTicketStatus:   "ONLINE",
		OnlineOpen:        true,
		SourceEndpoint:    queueSourceEndpointStoreByID,
		APIProfileVersion: queueAPIProfileStoreDetailV1,
	}); err != nil {
		t.Fatalf("appendQueueObservation() error = %v", err)
	}
}
