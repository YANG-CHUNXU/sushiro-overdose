package core

import (
	"strings"
	"testing"
	"time"
)

func TestMaskPhone(t *testing.T) {
	cases := map[string]string{
		"13800138000":    "138****8000",
		"123":            "***",
		"":               "***",
		"  13800138000 ": "138****8000",
	}
	for in, want := range cases {
		if got := MaskPhone(in); got != want {
			t.Errorf("MaskPhone(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSanitizeDiagnosticLine(t *testing.T) {
	// 手机号打码
	if got := SanitizeDiagnosticLine("call 13800138000 now"); !strings.Contains(got, "138****8000") || strings.Contains(got, "13800138000") {
		t.Errorf("phone not masked: %q", got)
	}
	// Authorization / Bearer 被替换
	for _, line := range []string{
		"Authorization: Bearer abcdef.token.value",
		"x-app-code=SECRETCODE",
		"phone_number: 13800138000",
		"reservation_authorization=tok123",
	} {
		got := SanitizeDiagnosticLine(line)
		if !strings.Contains(got, "***") {
			t.Errorf("secret not redacted in %q -> %q", line, got)
		}
		for _, leak := range []string{"abcdef.token.value", "SECRETCODE", "tok123", "13800138000"} {
			if strings.Contains(got, leak) {
				t.Errorf("leak %q remained in %q", leak, got)
			}
		}
	}
	// 普通行不动
	if got := SanitizeDiagnosticLine("store 3006 wait 30"); got != "store 3006 wait 30" {
		t.Errorf("benign line changed: %q", got)
	}
}

func TestDefaultString(t *testing.T) {
	if DefaultString("", "fb") != "fb" {
		t.Error("empty should fall back")
	}
	if DefaultString("  ", "fb") != "fb" {
		t.Error("blank should fall back")
	}
	if DefaultString("x", "fb") != "x" {
		t.Error("non-empty should pass through")
	}
}

func TestNormalizeErrorBody(t *testing.T) {
	if got := NormalizeErrorBody(nil); got != "<empty>" {
		t.Errorf("empty body = %q", got)
	}
	if got := NormalizeErrorBody([]byte(`{"code":"E010"}`)); !strings.Contains(got, "E010") {
		t.Errorf("json body lost detail: %q", got)
	}
	if got := NormalizeErrorBody([]byte("not json")); got != "not json" {
		t.Errorf("raw body = %q", got)
	}
}

func TestParseCompactTime(t *testing.T) {
	h, m, s, err := ParseCompactTime("193045")
	if err != nil || h != 19 || m != 30 || s != 45 {
		t.Fatalf("193045 -> %d:%d:%d err=%v", h, m, s, err)
	}
	if _, _, _, err := ParseCompactTime("1930"); err == nil {
		t.Error("len!=6 should error")
	}
}

func TestFormatCompactTime(t *testing.T) {
	if got := FormatCompactTime("193000"); got != "19:30" {
		t.Errorf("got %q", got)
	}
	if got := FormatCompactTime("bad"); got != "bad" {
		t.Errorf("invalid should pass through, got %q", got)
	}
}

func TestParseCompactDateAndSlotWindow(t *testing.T) {
	loc := time.UTC
	d, err := ParseCompactDate("20260605", loc)
	if err != nil || d.Year() != 2026 || d.Month() != time.June || d.Day() != 5 {
		t.Fatalf("date parse: %v err=%v", d, err)
	}
	if _, err := ParseCompactDate("2026", loc); err == nil {
		t.Error("bad date should error")
	}
	if got := FormatSlotWindow("20260605", "120000", "130000", loc); got != "2026-06-05 12:00-13:00" {
		t.Errorf("slot window = %q", got)
	}
}

func TestWeekdayIndexMon0(t *testing.T) {
	cases := map[time.Weekday]int{
		time.Monday: 0, time.Tuesday: 1, time.Friday: 4, time.Saturday: 5, time.Sunday: 6,
	}
	for wd, want := range cases {
		if got := WeekdayIndexMon0(wd); got != want {
			t.Errorf("WeekdayIndexMon0(%v)=%d want %d", wd, got, want)
		}
	}
}

func TestSlotDateTime(t *testing.T) {
	loc := time.UTC
	got, err := SlotDateTime(Slot{Date: "20260605", Start: "123000"}, loc)
	if err != nil || got.Hour() != 12 || got.Minute() != 30 {
		t.Fatalf("SlotDateTime -> %v err=%v", got, err)
	}
}

func TestNormalizePreferencesDefaults(t *testing.T) {
	got := NormalizePreferences(UserPreferences{})
	if got.Adult != 2 {
		t.Errorf("adult default = %d want 2", got.Adult)
	}
	if got.TableType != "T" {
		t.Errorf("tableType default = %q want T", got.TableType)
	}
	if got.DayPriorityMode != DayPriorityDate {
		t.Errorf("day priority mode = %q", got.DayPriorityMode)
	}
	if got.SlotStrategy != SlotStrategyEarliest {
		t.Errorf("slot strategy = %q", got.SlotStrategy)
	}
	if got.TargetTime != DefaultTargetTime {
		t.Errorf("target time = %q", got.TargetTime)
	}
	// 显式人数应被保留
	if g := NormalizePreferences(UserPreferences{Adult: 4, TableType: "C"}); g.Adult != 4 || g.TableType != "C" {
		t.Errorf("explicit values not preserved: adult=%d table=%q", g.Adult, g.TableType)
	}
}

func TestNormalizePreferencesUIMode(t *testing.T) {
	got := NormalizePreferences(UserPreferences{})
	if got.UIMode != UIModeSimple {
		t.Fatalf("default ui mode = %q want %q", got.UIMode, UIModeSimple)
	}

	for _, mode := range []string{UIModeSimple, UIModeAdvanced} {
		got := NormalizePreferences(UserPreferences{UIMode: mode})
		if got.UIMode != mode {
			t.Errorf("valid ui mode %q normalized to %q", mode, got.UIMode)
		}
	}

	got = NormalizePreferences(UserPreferences{UIMode: "debug"})
	if got.UIMode != UIModeSimple {
		t.Fatalf("invalid ui mode normalized to %q want %q", got.UIMode, UIModeSimple)
	}
}

func TestNormalizePreferencePhoneNumber(t *testing.T) {
	cases := map[string]string{
		"138-0013-8000": "13800138000",
		"13800138000":   "13800138000",
		"abc":           "",
		"12345":         "", // <8 位数字
		"  ":            "",
	}
	for in, want := range cases {
		if got := NormalizePreferencePhoneNumber(in); got != want {
			t.Errorf("NormalizePreferencePhoneNumber(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseTimeSeconds(t *testing.T) {
	if got := ParseTimeSeconds("1930"); got != 19*3600+30*60 {
		t.Errorf("1930 -> %d", got)
	}
	if got := ParseTimeSeconds("193045"); got != 19*3600+30*60+45 {
		t.Errorf("193045 -> %d", got)
	}
	for _, bad := range []string{"", "9999", "2500", "abc"} {
		if got := ParseTimeSeconds(bad); got != -1 {
			t.Errorf("ParseTimeSeconds(%q)=%d want -1", bad, got)
		}
	}
}
