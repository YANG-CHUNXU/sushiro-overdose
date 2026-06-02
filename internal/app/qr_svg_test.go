package app

import (
	"strings"
	"testing"
)

func TestQRSVGGeneratesInlineSVG(t *testing.T) {
	got := qrSVG("http://192.168.1.10:54321/ua/test-token")
	if !strings.Contains(got, `<svg`) || !strings.Contains(got, `<path`) {
		t.Fatalf("qrSVG() = %q, want inline SVG", got)
	}
}

func TestQRSVGRejectsOverCapacityText(t *testing.T) {
	if got := qrSVG(strings.Repeat("x", 107)); got != "" {
		t.Fatalf("qrSVG(over capacity) returned %q, want empty", got)
	}
}
