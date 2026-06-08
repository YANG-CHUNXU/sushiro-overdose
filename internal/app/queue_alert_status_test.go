package app

import (
	"testing"
	"time"
)

func TestBuildQueueAlertStatusReportsProgressAndNext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 6, 7, 18, 30, 0, 0, time.FixedZone("CST", 8*3600))
	cfg := normalizeQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{
		{
			StoreID:     "3006",
			StoreName:   "太阳宫凯德店",
			Label:       "我",
			Type:        queueAlertCalledReach,
			TargetNo:    1078,
			NotifyAtNos: []int{1000, 1025, 1050},
			TravelMin:   30,
			Enabled:     true,
		},
		{
			StoreID:    "3006",
			StoreName:  "太阳宫凯德店",
			Label:      "朋友",
			Type:       queueAlertCalledReach,
			TargetNo:   1269,
			NotifyAtNo: 1150,
			Enabled:    true,
		},
		{
			StoreID:    "3006",
			StoreName:  "太阳宫凯德店",
			Type:       queueAlertCalledReach,
			TargetNo:   1400,
			NotifyAtNo: 1300,
			Enabled:    false,
		},
	}})
	if err := SaveQueueAlertConfig(cfg); err != nil {
		t.Fatal(err)
	}
	saveQueueAlertState(map[string]queueAlertRuleState{
		cfg.Rules[0].key(): {FiredOnce: true, FiredAt: now.Add(-5 * time.Minute).Format(time.RFC3339)},
	})
	for _, observation := range []QueueObservation{
		{StoreID: "3006", CollectedAt: now.Add(-10 * time.Minute).Format(time.RFC3339), DisplayCalledNo: 980, WaitMinutes: 80, GroupQueuesCount: 80},
		{StoreID: "3006", CollectedAt: now.Format(time.RFC3339), DisplayCalledNo: 1005, WaitMinutes: 65, GroupQueuesCount: 65},
	} {
		if err := appendQueueObservation(observation); err != nil {
			t.Fatal(err)
		}
	}

	status := BuildQueueAlertStatus(now)
	if status.Notifications.Configured {
		t.Fatal("notifications should be unconfigured in a temporary HOME")
	}
	if status.Sampling.LastDataAt != now.Format(time.RFC3339) {
		t.Fatalf("last data at = %q, want latest observation time", status.Sampling.LastDataAt)
	}
	if len(status.Rules) != 5 {
		t.Fatalf("rules len = %d, want 5", len(status.Rules))
	}
	byThreshold := map[int]QueueAlertRuleStatus{}
	for _, rule := range status.Rules {
		byThreshold[rule.Threshold] = rule
	}
	if !byThreshold[1000].Fired || byThreshold[1000].Status != "fired" {
		t.Fatalf("1000 status = %#v, want fired", byThreshold[1000])
	}
	next := byThreshold[1025]
	if !next.Next || next.Status != "armed" {
		t.Fatalf("1025 status = %#v, want next armed rule", next)
	}
	if next.CurrentCalledNo != 1005 || next.RemainingToThreshold != 20 || next.RemainingToTicket != 73 {
		t.Fatalf("1025 progress = %#v, want called=1005 threshold_gap=20 ticket_gap=73", next)
	}
	if next.EstimateToThresholdMinutes == nil || *next.EstimateToThresholdMinutes != 8 {
		t.Fatalf("estimate to threshold = %#v, want 8 minutes", next.EstimateToThresholdMinutes)
	}
	friend := byThreshold[1150]
	if friend.Label != "朋友" || friend.RemainingToThreshold != 145 || !friend.Next {
		t.Fatalf("friend status = %#v, want labelled next rule for that ticket", friend)
	}
	disabled := byThreshold[1300]
	if disabled.Status != "disabled" || disabled.Next {
		t.Fatalf("disabled status = %#v, want disabled and not next", disabled)
	}
}
