package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebSecurityMiddlewareProtectsLocalPOST(t *testing.T) {
	setWebCSRFToken("test-token")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/example", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(webSecurityMiddleware(mux))
	defer server.Close()

	tests := []struct {
		name       string
		origin     string
		csrf       string
		wantStatus int
	}{
		{
			name:       "missing csrf is rejected",
			origin:     server.URL,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "bad origin is rejected",
			origin:     "http://example.com",
			csrf:       "test-token",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "matching local origin and csrf pass",
			origin:     server.URL,
			csrf:       "test-token",
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, server.URL+"/api/example", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.csrf != "" {
				req.Header.Set("X-Sushiro-CSRF", tt.csrf)
			}
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestHandleNotificationTestRequiresConfiguredChannel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/test", nil)
	rr := httptest.NewRecorder()

	handleNotificationTest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRequestedCalendarStoresFiltersAllowedAndDedupes(t *testing.T) {
	got := requestedCalendarStores([]string{"001", "999"}, "002,001", []string{"001", "002"})
	want := []string{"001", "002"}
	if len(got) != len(want) {
		t.Fatalf("stores = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stores = %v, want %v", got, want)
		}
	}
}

func TestFilterCalendarSlots(t *testing.T) {
	slots := []Slot{
		{Start: "103000", End: "110000", Availability: "AVAILABLE"},
		{Start: "123000", End: "130000", Availability: "FULL"},
		{Start: "193000", End: "200000", Availability: "AVAILABLE"},
	}

	lunchAvailable := filterCalendarSlots(slots, true, "lunch")
	if len(lunchAvailable) != 1 || lunchAvailable[0].Start != "103000" {
		t.Fatalf("lunch available = %#v", lunchAvailable)
	}

	dinner := filterCalendarSlots(slots, false, "dinner")
	if len(dinner) != 1 || dinner[0].Start != "193000" {
		t.Fatalf("dinner = %#v", dinner)
	}
}
