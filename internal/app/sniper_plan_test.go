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

	running := plan
	running.Targets[0].Status = "running"
	runningExpired := RefreshSniperPlan(running, openAt.Add(4*time.Minute), loc)
	if runningExpired.Targets[0].Status != "expired" {
		t.Fatalf("running status = %q, want expired", runningExpired.Targets[0].Status)
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

func TestStopRemainingSniperPlanTargetsAfterSuccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	loc := testLocation(t)
	// sniper 的"开放时间"= 目标日期 - 30 天，用一个足够远的未来日期，
	// 使 (日期-30天) 仍晚于现在，避免 refreshSniperPlanTarget 把 running 判成 expired。
	tomorrow := time.Now().In(loc).Add(60 * 24 * time.Hour).Format("20060102")
	targets := []SniperTarget{
		{Date: tomorrow, StartAfter: "193000", StartBefore: "203000", StoreID: "001"},
		{Date: tomorrow, StartAfter: "203000", StartBefore: "210000", StoreID: "001"},
	}
	plan := NormalizeSniperPlan(targets, loc)
	if len(plan.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(plan.Targets))
	}
	doneID := plan.Targets[0].ID
	plan.Targets[0].Status = "done"
	plan.Targets[0].CompletedAt = time.Now().In(loc).Format(time.RFC3339)
	plan.Targets[1].Status = "running"
	if err := SaveSniperPlan(plan, loc); err != nil {
		t.Fatal(err)
	}

	StopRemainingSniperPlanTargetsAfterSuccess(doneID, loc)

	got, err := LoadSniperPlan(loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].Status != "done" {
		t.Fatalf("done target = %#v", got.Targets[0])
	}
	if got.Targets[1].Status != "stopped" || got.Targets[1].LastError == "" {
		t.Fatalf("remaining target = %#v, want stopped with reason", got.Targets[1])
	}
}

func TestSaveSniperPlanReplacingTargetsPreservesRuntimeState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	loc := testLocation(t)
	future := time.Now().In(loc).Add(60 * 24 * time.Hour).Format("20060102")
	target := SniperTarget{Date: future, StartAfter: "193000", StartBefore: "203000", StoreID: "001"}
	plan := NormalizeSniperPlan([]SniperTarget{target}, loc)
	if len(plan.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(plan.Targets))
	}
	completedAt := time.Now().In(loc).Format(time.RFC3339)
	plan.Targets[0].Status = "done"
	plan.Targets[0].Attempts = 17
	plan.Targets[0].LastAttemptAt = completedAt
	plan.Targets[0].CompletedAt = completedAt
	if err := SaveSniperPlan(plan, loc); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveSniperPlanReplacingTargets([]SniperTarget{target}, loc); err != nil {
		t.Fatalf("SaveSniperPlanReplacingTargets() error = %v", err)
	}

	got, err := LoadSniperPlan(loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(got.Targets))
	}
	if got.Targets[0].Status != "done" || got.Targets[0].Attempts != 17 || got.Targets[0].CompletedAt != completedAt {
		t.Fatalf("运行中/已完成状态不应被 UI 保存覆盖: %#v", got.Targets[0])
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
