package app

import "testing"

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
