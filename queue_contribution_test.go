package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildQueueContributionPayloadAggregatesOnlySafeStats(t *testing.T) {
	cfg := QueueContributionConfig{
		CollectorURL:        defaultCollectorURL,
		AnonymousInstallID:  "install-1",
		MinSamplesPerBucket: 3,
	}
	sessions := []QueueSession{
		{StoreID: "001", TakenAt: "2026-05-16T18:00:00+08:00", TicketNo: 101, DisplayCalledNoAtTake: 80, CalledForUserAt: "2026-05-16T18:40:00+08:00", ActualWaitMinutes: 40, CheckedInAt: "2026-05-16T18:20:00+08:00", PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-23T18:10:00+08:00", TicketNo: 102, DisplayCalledNoAtTake: 84, CalledForUserAt: "2026-05-23T19:00:00+08:00", ActualWaitMinutes: 50, CheckedInAt: "2026-05-23T18:35:00+08:00", PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-30T18:25:00+08:00", TicketNo: 103, DisplayCalledNoAtTake: 88, CalledForUserAt: "2026-05-30T19:25:00+08:00", ActualWaitMinutes: 60, CheckedInAt: "2026-05-30T18:55:00+08:00", PartySize: 2, TableType: "T", ExpiredOrMissed: true},
		{StoreID: "002", TakenAt: "2026-05-16T18:00:00+08:00", ActualWaitMinutes: 20, PartySize: 5, TableType: "C"},
	}

	payload, usable := BuildQueueContributionPayload(cfg, sessions, time.Date(2026, 5, 16, 20, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if usable != 4 {
		t.Fatalf("usable sessions = %d, want 4", usable)
	}
	if len(payload.Stats) != 1 {
		t.Fatalf("stats len = %d, want 1", len(payload.Stats))
	}
	got := payload.Stats[0]
	if got.StoreID != "001" || got.Weekday != 6 || got.TimeBucket != "18:00" || got.PartySizeBucket != "1-2" || got.TableType != "T" {
		t.Fatalf("unexpected bucket: %+v", got)
	}
	if got.Samples != 3 {
		t.Fatalf("samples = %d, want 3", got.Samples)
	}
	if got.WaitP50Minutes == nil || *got.WaitP50Minutes != 50 {
		t.Fatalf("wait p50 = %v, want 50", got.WaitP50Minutes)
	}
	if got.WaitP80Minutes == nil || *got.WaitP80Minutes != 56 {
		t.Fatalf("wait p80 = %v, want 56", got.WaitP80Minutes)
	}
	if got.MissedRate < 0.33 || got.MissedRate > 0.34 {
		t.Fatalf("missed rate = %.4f, want about 0.3333", got.MissedRate)
	}
}

func TestQueueContributionPayloadDoesNotExposeRawSensitiveFields(t *testing.T) {
	cfg := QueueContributionConfig{CollectorURL: defaultCollectorURL, AnonymousInstallID: "install-2", MinSamplesPerBucket: 3}
	sessions := []QueueSession{
		{StoreID: "001", TakenAt: "2026-05-16T18:00:00+08:00", TicketNo: 123, DisplayCalledNoAtTake: 100, ActualWaitMinutes: 30, PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-23T18:00:00+08:00", TicketNo: 124, DisplayCalledNoAtTake: 101, ActualWaitMinutes: 40, PartySize: 2, TableType: "T"},
		{StoreID: "001", TakenAt: "2026-05-30T18:00:00+08:00", TicketNo: 125, DisplayCalledNoAtTake: 102, ActualWaitMinutes: 50, PartySize: 2, TableType: "T"},
	}
	payload, _ := BuildQueueContributionPayload(cfg, sessions, time.Now())
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ticket_no", "display_called_no", "wechat", "authorization", "phone"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("payload contains forbidden field %q: %s", forbidden, data)
		}
	}
	if payload.InstallIDHash == "" || payload.InstallIDHash == cfg.AnonymousInstallID {
		t.Fatalf("install id hash not anonymized: %q", payload.InstallIDHash)
	}
}
