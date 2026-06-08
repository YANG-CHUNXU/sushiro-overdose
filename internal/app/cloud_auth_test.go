package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudAuthConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")

	want := CloudAuthConfig{
		BaseURL:      "https://example.workers.dev",
		SessionToken: "session-token",
		UserLogin:    "octocat",
	}
	if err := SaveCloudAuthConfig(want); err != nil {
		t.Fatal(err)
	}
	got := LoadCloudAuthConfig()
	if got.BaseURL != want.BaseURL || got.SessionToken != want.SessionToken || got.UserLogin != want.UserLogin {
		t.Fatalf("LoadCloudAuthConfig() = %+v, want %+v", got, want)
	}
}

func TestHandleCloudAuthCallbackSavesVerifiedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-session" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(cloudMeResponse{
			OK: true,
			User: cloudUser{
				Login:     "octocat",
				Name:      "The Octocat",
				AvatarURL: "https://avatars.githubusercontent.com/u/583231",
			},
			Session: cloudSession{ExpiresAt: "2026-07-08T00:00:00Z"},
		})
	}))
	defer server.Close()

	if err := SaveCloudAuthConfig(CloudAuthConfig{BaseURL: server.URL}); err != nil {
		t.Fatal(err)
	}
	state, err := newCloudOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/cloud/auth/callback?state="+state+"&token=test-session", nil)
	rr := httptest.NewRecorder()

	handleCloudAuthCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	got := LoadCloudAuthConfig()
	if got.SessionToken != "test-session" || got.UserLogin != "octocat" || got.ExpiresAt == "" {
		t.Fatalf("cloud config not saved from callback: %+v", got)
	}
}

func TestHandleCloudAuthStartRequiresWorkerURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")

	req := httptest.NewRequest(http.MethodGet, "/api/cloud/auth/start", nil)
	rr := httptest.NewRecorder()

	handleCloudAuthStart(rr, req)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "Cloudflare Worker URL") {
		t.Fatalf("unexpected response: %d %s", rr.Code, rr.Body.String())
	}
}
