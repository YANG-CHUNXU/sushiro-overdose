package app

import (
	"testing"
	"time"
)

func TestNetTicketIssuedTodayBlocksSampling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	if err := SaveNetTicketPlan(NetTicketPlan{
		Enabled:   true,
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
