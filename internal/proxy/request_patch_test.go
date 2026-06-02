package proxy

import (
	"net/http"
	"strings"
	"testing"
)

func TestPatchSushiroRequestForForwardRewritesWindowsUA(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://crm-cn-prd.sushiro.com.cn/wechat/api/2.0/stores", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) MicroMessenger/8.0 MiniProgramEnv/Windows")

	body, patches := patchSushiroRequestForForward(req, nil)

	if len(body) != 0 {
		t.Fatalf("body = %q, want empty", string(body))
	}
	if !strings.Contains(strings.Join(patches, ","), "ua=mobile-weixin") {
		t.Fatalf("patches = %#v, want ua patch", patches)
	}
	ua := strings.ToLower(req.Header.Get("User-Agent"))
	if strings.Contains(ua, "windows") || !strings.Contains(ua, "miniprogramenv/android") {
		t.Fatalf("ua = %q, want mobile android ua", req.Header.Get("User-Agent"))
	}
}

func TestPatchSushiroRequestBodyRewritesUnsignedWindowsPlatform(t *testing.T) {
	body := []byte(`{"platform":"windows","os":"Win32","storeId":"3006"}`)

	patched, patches := patchSushiroRequestBody("application/json", body)

	text := string(patched)
	if !strings.Contains(strings.Join(patches, ","), "body.platform=android") {
		t.Fatalf("patches = %#v, want platform patch", patches)
	}
	if strings.Contains(strings.ToLower(text), "windows") || strings.Contains(strings.ToLower(text), "win32") {
		t.Fatalf("patched body still contains windows markers: %s", text)
	}
	if !strings.Contains(text, `"platform":"android"`) || !strings.Contains(text, `"os":"android"`) {
		t.Fatalf("patched body = %s", text)
	}
}

func TestPatchSushiroRequestBodySkipsSignedBody(t *testing.T) {
	body := []byte(`{"platform":"windows","signature":"abc123"}`)

	patched, patches := patchSushiroRequestBody("application/json", body)

	if len(patches) != 0 {
		t.Fatalf("patches = %#v, want none for signed body", patches)
	}
	if string(patched) != string(body) {
		t.Fatalf("patched = %s, want original %s", string(patched), string(body))
	}
}
