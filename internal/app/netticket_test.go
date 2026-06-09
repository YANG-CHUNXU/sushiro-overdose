package app

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

func TestNetTicketRoutinePlansTodayFromHistoricalMealPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)
	observedAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.Local)
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

	resp := saveNetTicketRoutineConfigLocked(NetTicketRoutine{
		Enabled:        true,
		StoreID:        "3006",
		StoreName:      "太阳宫凯德店",
		TargetMealTime: "1300",
		SafetyMinutes:  10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusArmed {
		t.Fatalf("routine status = %q, want armed; error=%q", resp.Routine.Status, resp.Routine.LastError)
	}
	if resp.Plan.Source != netTicketPlanSourceRoutine {
		t.Fatalf("plan source = %q, want routine", resp.Plan.Source)
	}
	if resp.Plan.TargetTime != "1150" {
		t.Fatalf("plan target time = %q, want 1150", resp.Plan.TargetTime)
	}
	if resp.Plan.TargetMealTime != "1300" || resp.Plan.RoutinePlannedDate != now.Format("2006-01-02") {
		t.Fatalf("routine metadata missing on plan: %+v", resp.Plan)
	}
}

func TestNetTicketRoutineDoesNotOverwriteManualPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Date(2026, 6, 9, 9, 0, 0, 0, time.Local)
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
		Enabled:        true,
		StoreID:        "3006",
		TargetMealTime: "1300",
		SafetyMinutes:  10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusBlocked {
		t.Fatalf("routine status = %q, want blocked", resp.Routine.Status)
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
		Enabled:        false,
		StoreID:        "3006",
		TargetMealTime: "1300",
		SafetyMinutes:  10,
	}, now)

	if resp.Plan.Enabled {
		t.Fatalf("routine plan should be disabled: %+v", resp.Plan)
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
		Enabled:        true,
		StoreID:        "3006",
		TargetMealTime: "1300",
		SafetyMinutes:  10,
	}, now)

	if resp.Routine.Status != netTicketRoutineStatusWaitingData {
		t.Fatalf("routine status = %q, want waiting_data", resp.Routine.Status)
	}
	if resp.Plan.Enabled {
		t.Fatalf("routine should not arm a plan without data: %+v", resp.Plan)
	}
}
