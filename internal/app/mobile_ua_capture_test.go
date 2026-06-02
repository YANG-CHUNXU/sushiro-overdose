package app

import (
	"net/http"
	"strings"
	"testing"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

func TestMobileUACaptureServerSavesUA(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	status, err := mobileUACapture.start()
	if err != nil {
		t.Fatalf("start() error = %v", err)
	}
	defer mobileUACapture.stop()

	target, ok := status["url"].(string)
	if !ok || target == "" {
		t.Fatalf("status url = %#v, want non-empty string", status["url"])
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; Pixel Build/TQ3A; wv) AppleWebKit/537.36 Mobile MicroMessenger/8.0.50")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("capture request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cfg, err := LoadMobileUA()
	if err != nil {
		t.Fatalf("LoadMobileUA() error = %v", err)
	}
	if !strings.Contains(strings.ToLower(cfg.NormalizedUserAgent), "miniprogramenv/android") {
		t.Fatalf("NormalizedUserAgent = %q, want android mini-program env", cfg.NormalizedUserAgent)
	}
}
