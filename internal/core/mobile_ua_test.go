package core

import (
	"strings"
	"testing"
)

func TestNormalizeMobileWeixinUAReplacesWindows(t *testing.T) {
	got := NormalizeMobileWeixinUA("Mozilla/5.0 (Windows NT 10.0) MicroMessenger/8.0 MiniProgramEnv/Windows")
	lower := strings.ToLower(got)
	if strings.Contains(lower, "windows") || !strings.Contains(lower, "miniprogramenv/android") {
		t.Fatalf("NormalizeMobileWeixinUA() = %q, want android mini-program UA without windows", got)
	}
}

func TestNormalizeMobileWeixinUAAppendsMiniProgramEnv(t *testing.T) {
	got := NormalizeMobileWeixinUA("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 MicroMessenger/8.0.50")
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "micromessenger/") || !strings.Contains(lower, "miniprogramenv/ios") {
		t.Fatalf("NormalizeMobileWeixinUA() = %q, want iOS mini-program env", got)
	}
}

func TestEffectiveUserAgentPrefersSavedMobileUA(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	saved, err := SaveMobileUA("Mozilla/5.0 (Linux; Android 13; Pixel Build/TQ3A; wv) AppleWebKit/537.36 Mobile MicroMessenger/8.0.50", "test", "127.0.0.1")
	if err != nil {
		t.Fatalf("SaveMobileUA() error = %v", err)
	}
	got := EffectiveUserAgent("Mozilla/5.0 (Windows NT 10.0) MicroMessenger/8.0 MiniProgramEnv/Windows")
	if got != saved.NormalizedUserAgent {
		t.Fatalf("EffectiveUserAgent() = %q, want saved %q", got, saved.NormalizedUserAgent)
	}
}

func TestLooksMobileWeixinUA(t *testing.T) {
	if !LooksMobileWeixinUA(DefaultMobileWeixinUA()) {
		t.Fatal("DefaultMobileWeixinUA() should look like mobile Weixin")
	}
	if LooksMobileWeixinUA("Mozilla/5.0 (Windows NT 10.0) MicroMessenger/8.0 MiniProgramEnv/Windows") {
		t.Fatal("Windows mini-program UA should not look like mobile Weixin")
	}
}
