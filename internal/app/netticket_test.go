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
