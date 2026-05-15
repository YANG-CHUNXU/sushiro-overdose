package main

import (
	"testing"
	"time"
)

func TestNormalizeSamplingConfigBoundsIntervalAndTimes(t *testing.T) {
	cfg := NormalizeSamplingConfig(SamplingConfig{
		IntervalSeconds: 10,
		ActiveStart:     "bad",
		ActiveEnd:       "22:30",
		StoreIDs:        []string{"001", "001", " 002 "},
	})
	if cfg.IntervalSeconds != 60 {
		t.Fatalf("IntervalSeconds = %d, want 60", cfg.IntervalSeconds)
	}
	if cfg.ActiveStart != "100000" {
		t.Fatalf("ActiveStart = %q, want 100000", cfg.ActiveStart)
	}
	if cfg.ActiveEnd != "223000" {
		t.Fatalf("ActiveEnd = %q, want 223000", cfg.ActiveEnd)
	}
	if got := len(cfg.StoreIDs); got != 2 {
		t.Fatalf("StoreIDs len = %d, want 2", got)
	}
}

func TestSamplingActiveWindowSupportsOvernight(t *testing.T) {
	cfg := NormalizeSamplingConfig(SamplingConfig{IntervalSeconds: 300, ActiveStart: "2200", ActiveEnd: "0200"})
	loc := testLocation(t)
	if !samplingInActiveWindow(cfg, time.Date(2026, 5, 15, 23, 0, 0, 0, loc)) {
		t.Fatal("23:00 should be inside overnight window")
	}
	if !samplingInActiveWindow(cfg, time.Date(2026, 5, 16, 1, 0, 0, 0, loc)) {
		t.Fatal("01:00 should be inside overnight window")
	}
	if samplingInActiveWindow(cfg, time.Date(2026, 5, 16, 12, 0, 0, 0, loc)) {
		t.Fatal("12:00 should be outside overnight window")
	}
}

func TestCompareVersions(t *testing.T) {
	if compareVersions("v2.4.0", "2.3.9") <= 0 {
		t.Fatal("v2.4.0 should be newer than 2.3.9")
	}
	if compareVersions("v2.3.0", "2.3.0") != 0 {
		t.Fatal("same versions should compare equal")
	}
}
