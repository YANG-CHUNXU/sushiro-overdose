package app

import (
	"strings"
	"testing"
	"time"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

func TestNetTicketStatusPushDue(t *testing.T) {
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	plan := NetTicketPlan{
		Enabled:           true,
		Status:            "success",
		StoreID:           "3006",
		StatusPushEnabled: true,
		LastStatusPushAt:  now.Add(-11 * time.Minute).Format(time.RFC3339),
	}
	if !netTicketStatusPushDue(plan, now) {
		t.Fatal("status push should be due after default interval")
	}

	plan.LastStatusPushAt = now.Add(-5 * time.Minute).Format(time.RFC3339)
	if netTicketStatusPushDue(plan, now) {
		t.Fatal("status push should not be due before default interval")
	}
}

func TestShouldMonitorNetTicketStatusStopsAfterMaxAge(t *testing.T) {
	now := time.Date(2026, 6, 3, 18, 0, 0, 0, time.Local)
	plan := NetTicketPlan{
		Enabled:           true,
		Status:            "success",
		StoreID:           "3006",
		StatusPushEnabled: true,
		FiredAt:           now.Add(-9 * time.Hour).Format(time.RFC3339),
	}
	if shouldMonitorNetTicketStatus(plan, now) {
		t.Fatal("old ticket should stop status monitoring")
	}
}

func TestFormatNetTicketStatusPush(t *testing.T) {
	eta := 42
	title, body := formatNetTicketStatusPush(
		NetTicketPlan{StoreID: "3006", StoreName: "太阳宫凯德店", Number: "1843"},
		ReservationRecord{},
		QueueLivePanel{StoreID: "3006", StoreName: "太阳宫凯德店", CalledNo: 1801, WaitGroups: 1182, EtaMinutes: &eta},
	)
	if title != "📍 排队状态" {
		t.Fatalf("title = %q, want queue status", title)
	}
	for _, want := range []string{"太阳宫凯德店", "你的号 1843", "当前叫号 1801", "还差约 42 桌", "预计约 42 分钟"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, missing %q", body, want)
		}
	}
}

func TestFormatNetTicketStatusPushUrgent(t *testing.T) {
	title, body := formatNetTicketStatusPush(
		NetTicketPlan{StoreID: "3006", StoreName: "太阳宫凯德店", Number: "1843"},
		ReservationRecord{},
		QueueLivePanel{StoreID: "3006", StoreName: "太阳宫凯德店", CalledNo: 1830},
	)
	if title != "🔔 快叫到你了" {
		t.Fatalf("title = %q, want urgent title", title)
	}
	if !strings.Contains(body, "还差约 13 桌") {
		t.Fatalf("body = %q, missing remaining groups", body)
	}
}
