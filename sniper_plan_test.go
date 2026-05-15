package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestRefreshSniperPlanStatuses(t *testing.T) {
	loc := testLocation(t)
	targets := []SniperTarget{{
		Date:        "20260614",
		StartAfter:  "193000",
		StartBefore: "203000",
		StoreID:     "001",
	}}
	plan := NormalizeSniperPlan(targets, loc)
	if len(plan.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(plan.Targets))
	}

	openAt := sniperOpenTime(targets[0], loc)
	pending := RefreshSniperPlan(plan, openAt.Add(-time.Minute), loc)
	if pending.Targets[0].Status != "pending" || pending.Targets[0].CountdownSeconds <= 0 {
		t.Fatalf("pending target = %#v", pending.Targets[0])
	}

	open := RefreshSniperPlan(plan, openAt.Add(time.Minute), loc)
	if open.Targets[0].Status != "open" {
		t.Fatalf("status = %q, want open", open.Targets[0].Status)
	}

	expired := RefreshSniperPlan(plan, openAt.Add(4*time.Minute), loc)
	if expired.Targets[0].Status != "expired" {
		t.Fatalf("status = %q, want expired", expired.Targets[0].Status)
	}
}

func TestLoadSniperPlanReadsLegacyTargets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	loc := testLocation(t)

	legacy := []SniperTarget{{
		Date:        "20260614",
		StartAfter:  "193000",
		StartBefore: "203000",
		StoreID:     "001",
	}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(appDirPath(), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sniperConfigPath(), data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan, err := LoadSniperPlan(loc)
	if err != nil {
		t.Fatalf("LoadSniperPlan() error = %v", err)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(plan.Targets))
	}
	if plan.Targets[0].StoreID != "001" || plan.Targets[0].OpenAt == "" {
		t.Fatalf("target = %#v", plan.Targets[0])
	}
}
