package app

import (
	"context"
	"strings"
	"testing"
)

func TestQueueAlertWaitBelowHysteresis(t *testing.T) {
	rule := QueueAlertRule{StoreID: "3006", Type: queueAlertWaitBelow, WaitMinutes: 60, Enabled: true}
	state := map[string]queueAlertRuleState{}

	// 等待高于阈值+迟滞，只武装不推送。
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{WaitMinutes: 100}, state); fire {
		t.Fatal("high wait should not fire")
	}
	if !state[rule.key()].Armed {
		t.Fatal("rule should be armed after high wait")
	}
	// 等待落到阈值内且已武装，推送一次并解除武装。
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{WaitMinutes: 50}, state); !fire {
		t.Fatal("wait dropping below threshold should fire")
	}
	if state[rule.key()].Armed {
		t.Fatal("rule should disarm after firing")
	}
	// 仍然低位但未重新武装，不再重复推送。
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{WaitMinutes: 50}, state); fire {
		t.Fatal("should not re-fire while disarmed")
	}
}

func TestQueueAlertCalledReachOnce(t *testing.T) {
	rule := QueueAlertRule{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 900, LeadGroups: 5, Enabled: true}
	state := map[string]queueAlertRuleState{}

	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 890}, state); fire {
		t.Fatal("called 890 vs target-lead 895 should not fire")
	}
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 896}, state); !fire {
		t.Fatal("called 896 should fire")
	}
	if !state[rule.key()].FiredOnce {
		t.Fatal("called_reach should mark FiredOnce")
	}
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 905}, state); fire {
		t.Fatal("called_reach should fire only once")
	}
}

func TestQueueAlertCalledReachAllowsMultipleLeadGroups(t *testing.T) {
	rules := []QueueAlertRule{
		{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1078, LeadGroups: 78, Enabled: true},
		{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1078, LeadGroups: 53, Enabled: true},
		{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1078, LeadGroups: 28, Enabled: true},
	}
	state := map[string]queueAlertRuleState{}

	if _, _, fire := queueAlertEvaluateRule(rules[0], QueueObservation{DisplayCalledNo: 1000}, state); !fire {
		t.Fatal("called 1000 should fire the 1000 threshold")
	}
	if _, _, fire := queueAlertEvaluateRule(rules[1], QueueObservation{DisplayCalledNo: 1000}, state); fire {
		t.Fatal("called 1000 should not fire the 1025 threshold")
	}
	if _, _, fire := queueAlertEvaluateRule(rules[2], QueueObservation{DisplayCalledNo: 1000}, state); fire {
		t.Fatal("called 1000 should not fire the 1050 threshold")
	}

	if _, _, fire := queueAlertEvaluateRule(rules[1], QueueObservation{DisplayCalledNo: 1025}, state); !fire {
		t.Fatal("called 1025 should fire the 1025 threshold")
	}
	if _, _, fire := queueAlertEvaluateRule(rules[2], QueueObservation{DisplayCalledNo: 1050}, state); !fire {
		t.Fatal("called 1050 should fire the 1050 threshold")
	}
}

func TestQueueAlertCalledReachExpandsNotifyAtNos(t *testing.T) {
	cfg := normalizeQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{{
		StoreID:     "3006",
		StoreName:   "太阳宫凯德店",
		Type:        queueAlertCalledReach,
		TargetNo:    1078,
		NotifyAtNos: []int{1000, 1025, 1025, 1050},
		Enabled:     true,
	}}})

	if len(cfg.Rules) != 3 {
		t.Fatalf("rules len = %d, want 3: %#v", len(cfg.Rules), cfg.Rules)
	}
	wants := []struct {
		notifyAt int
		lead     int
	}{
		{1000, 78},
		{1025, 53},
		{1050, 28},
	}
	for i, want := range wants {
		if cfg.Rules[i].NotifyAtNo != want.notifyAt || cfg.Rules[i].LeadGroups != want.lead {
			t.Fatalf("rule %d = %#v, want notify_at_no=%d lead=%d", i, cfg.Rules[i], want.notifyAt, want.lead)
		}
	}
}

func TestQueueAlertCalledReachFiresAfterThresholdJump(t *testing.T) {
	rule := QueueAlertRule{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1269, NotifyAtNo: 1150, Enabled: true}
	state := map[string]queueAlertRuleState{}

	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 1149}, state); fire {
		t.Fatal("called 1149 should not fire the 1150 threshold")
	}
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 1156}, state); !fire {
		t.Fatal("called 1156 should fire the crossed 1150 threshold")
	}
	if _, _, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 1160}, state); fire {
		t.Fatal("crossed threshold should fire only once")
	}
}

func TestQueueAlertCalledReachIncludesLabelAndTravel(t *testing.T) {
	cfg := normalizeQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{{
		StoreID:    "3006",
		StoreName:  "太阳宫凯德店",
		Label:      " 我 ",
		Type:       queueAlertCalledReach,
		TargetNo:   1078,
		NotifyAtNo: 1050,
		TravelMin:  25,
		Enabled:    true,
	}}})
	if len(cfg.Rules) != 1 {
		t.Fatalf("rules len = %d, want 1", len(cfg.Rules))
	}
	rule := cfg.Rules[0]
	if rule.Label != "我" || rule.TravelMin != 25 {
		t.Fatalf("normalized rule = %#v, want trimmed label and travel minutes", rule)
	}

	title, body, fire := queueAlertEvaluateRule(rule, QueueObservation{DisplayCalledNo: 1051}, map[string]queueAlertRuleState{})
	if !fire || title != "🔔 快叫到你了" {
		t.Fatalf("fire/title = %v/%q, want called reminder", fire, title)
	}
	for _, want := range []string{"【我】", "已达到提醒点 1050", "号码 1078", "路程约 25 分钟"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q missing %q", body, want)
		}
	}
}

func TestEvaluateQueueAlertsBurnsCalledReachRuleAfterFire(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := normalizeQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{
		{StoreID: "3006", StoreName: "太阳宫凯德店", Type: queueAlertCalledReach, TargetNo: 1078, NotifyAtNo: 1050, Enabled: true},
		{StoreID: "3006", StoreName: "太阳宫凯德店", Type: queueAlertCalledReach, TargetNo: 1078, NotifyAtNo: 1070, Enabled: true},
	}})
	if err := SaveQueueAlertConfig(cfg); err != nil {
		t.Fatal(err)
	}

	evaluateQueueAlerts(context.Background(), QueueObservation{StoreID: "3006", DisplayCalledNo: 1051}, "太阳宫凯德店")

	got := LoadQueueAlertConfig()
	if len(got.Rules) != 1 {
		t.Fatalf("rules len after fire = %d, want 1: %#v", len(got.Rules), got.Rules)
	}
	if got.Rules[0].NotifyAtNo != 1070 {
		t.Fatalf("remaining rule = %#v, want 1070 threshold", got.Rules[0])
	}
	if _, exists := loadQueueAlertState()[cfg.Rules[0].key()]; exists {
		t.Fatal("fired called_reach state should be burned with the rule")
	}
}

func TestSaveQueueAlertConfigPrunesDeletedRuleState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := normalizeQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{
		{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1078, NotifyAtNo: 1000, Enabled: true},
		{StoreID: "3006", Type: queueAlertCalledReach, TargetNo: 1078, NotifyAtNo: 1025, Enabled: true},
	}})
	if err := SaveQueueAlertConfig(cfg); err != nil {
		t.Fatal(err)
	}
	saveQueueAlertState(map[string]queueAlertRuleState{
		cfg.Rules[0].key(): {FiredOnce: true},
		cfg.Rules[1].key(): {FiredOnce: true},
		"stale":            {FiredOnce: true},
	})

	if err := SaveQueueAlertConfig(QueueAlertConfig{Rules: []QueueAlertRule{cfg.Rules[1]}}); err != nil {
		t.Fatal(err)
	}
	state := loadQueueAlertState()
	if len(state) != 1 {
		t.Fatalf("state len = %d, want 1: %#v", len(state), state)
	}
	if _, exists := state[cfg.Rules[1].key()]; !exists {
		t.Fatalf("remaining rule state missing: %#v", state)
	}
}
