package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMobileAuthGuideContainsCAAndProxyInstructions(t *testing.T) {
	rr := httptest.NewRecorder()

	writeMobileAuthGuide(rr, mobileAuthGuideData{
		Hosts:     []string{"192.168.1.20"},
		ProxyPort: 8080,
		CAURL:     "/mobile-auth/token/ca.crt",
	})

	body := rr.Body.String()
	for _, want := range []string{"/mobile-auth/token/ca.crt", "192.168.1.20:8080", "关闭手机 Wi-Fi 代理"} {
		if !strings.Contains(body, want) {
			t.Fatalf("guide page missing %q: %s", want, body)
		}
	}
}
