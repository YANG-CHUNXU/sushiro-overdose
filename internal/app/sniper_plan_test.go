package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

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
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
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

func TestValidateSniperTargetsAcceptsWebDateAndTime(t *testing.T) {
	loc := testLocation(t)
	settings := Settings{StoreIDs: []string{"001"}, Location: loc}
	targets, rejected := validateSniperTargetsForSettings([]SniperTarget{{
		Date:        "2026-06-14",
		StartAfter:  "19:30",
		StartBefore: "20:30",
		StoreID:     "001",
	}}, settings)
	if len(rejected) != 0 {
		t.Fatalf("rejected = %#v", rejected)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(targets))
	}
	if targets[0].Date != "20260614" || targets[0].StartAfter != "193000" || targets[0].StartBefore != "203000" {
		t.Fatalf("target = %#v", targets[0])
	}
}

func TestValidateSniperTargetsRejectsInvalidRows(t *testing.T) {
	loc := testLocation(t)
	settings := Settings{StoreIDs: []string{"001"}, Location: loc}
	targets, rejected := validateSniperTargetsForSettings([]SniperTarget{
		{Date: "2026-06-14", StartAfter: "20:30", StartBefore: "19:30", StoreID: "001"},
		{Date: "2026-06-14", StartAfter: "19:30", StartBefore: "20:30", StoreID: "999"},
	}, settings)
	if len(targets) != 0 {
		t.Fatalf("targets = %#v, want none", targets)
	}
	if len(rejected) != 2 {
		t.Fatalf("rejected = %d, want 2", len(rejected))
	}
}

func TestParseSniperArgsAcceptsColonTime(t *testing.T) {
	targets := parseSniperArgs("20260614", "19:30-20:30", "001", nil)
	if len(targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(targets))
	}
	if targets[0].StartAfter != "193000" || targets[0].StartBefore != "203000" {
		t.Fatalf("target = %#v", targets[0])
	}
}
