package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestHandleCancelReservationRequiresReservationKind(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing kind is rejected",
			body: `{"ticket_id":123}`,
		},
		{
			name: "net ticket kind is rejected",
			body: `{"ticket_id":123,"kind":"net_ticket"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/reservations/cancel", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			handleCancelReservation(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestClearLocalReservationOnlyPreservesNetTicket(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := SaveState(StateFilePath(), State{
		ActiveReservation: &ReservationRecord{Kind: "net_ticket", Number: "1843", Wait: 12},
	}); err != nil {
		t.Fatal(err)
	}
	clearLocalReservationOnly()
	state, err := LoadState(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveReservation == nil || state.ActiveReservation.Kind != "net_ticket" {
		t.Fatalf("net ticket should be preserved: %+v", state.ActiveReservation)
	}
}

func TestClearLocalReservationOnlyClearsReservation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := SaveState(StateFilePath(), State{
		ActiveReservation: &ReservationRecord{Kind: "reservation", Number: "A001", TicketID: 123},
	}); err != nil {
		t.Fatal(err)
	}
	clearLocalReservationOnly()
	state, err := LoadState(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveReservation != nil {
		t.Fatalf("reservation should be cleared: %+v", state.ActiveReservation)
	}
}

func TestClearLocalReservationOnlyClearsWaitingReservation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := SaveState(StateFilePath(), State{
		ActiveReservation: &ReservationRecord{
			Kind:      "reservation",
			Status:    "WAITING",
			Number:    "7160",
			TicketID:  7160,
			QueueDate: "20260704",
			Start:     "140000",
			End:       "141500",
			Wait:      110,
		},
	}); err != nil {
		t.Fatal(err)
	}

	clearLocalReservationOnly()

	state, err := LoadState(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveReservation != nil {
		t.Fatalf("waiting reservation should be cleared: %+v", state.ActiveReservation)
	}
}

func TestLoadReservationsFallbackDropsStaleNetTicket(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := SaveState(StateFilePath(), State{
		ActiveReservation: &ReservationRecord{Kind: "net_ticket", Number: "893", QueueDate: "20000101", Wait: 25},
		SavedAt:           "2000-01-01T16:40:00+08:00",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveNetTicketPlan(NetTicketPlan{Status: "success", Number: "893", TicketID: 123}); err != nil {
		t.Fatal(err)
	}

	got := loadReservationsFallback()

	if len(got) != 0 {
		t.Fatalf("fallback = %#v, want empty", got)
	}
	state, err := LoadState(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveReservation != nil {
		t.Fatalf("stale net ticket state should be cleared: %+v", state.ActiveReservation)
	}
	plan := LoadNetTicketPlan()
	if plan.Number != "" || plan.TicketID != 0 {
		t.Fatalf("stale net ticket plan should be cleared: %+v", plan)
	}
}

func TestNoCurrentNetTicketErrorClearsLocalNetTicket(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := SaveState(StateFilePath(), State{
		ActiveReservation: &ReservationRecord{Kind: "net_ticket", Number: "893", QueueDate: time.Now().Format("20060102"), Wait: 25},
		SavedAt:           time.Now().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveNetTicketPlan(NetTicketPlan{Status: "success", Number: "893", TicketID: 123}); err != nil {
		t.Fatal(err)
	}

	if !isNoCurrentNetTicketError(errors.New(`net ticket status response missing ticket id/number: {"netTicket":null}`)) {
		t.Fatal("expected no current ticket error")
	}
	clearLocalNetTicketState()

	state, err := LoadState(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveReservation != nil {
		t.Fatalf("net ticket state should be cleared: %+v", state.ActiveReservation)
	}
}

func TestBookingEngineFinishRunClearsHandle(t *testing.T) {
	e := &BookingEngine{state: EngineState{Status: EngineSuccess}}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.cancel = cancel
	e.done = done
	_ = ctx

	e.finishRun(done)

	if e.cancel != nil || e.done != nil {
		t.Fatalf("run handle not cleared: cancel=%v done=%v", e.cancel, e.done)
	}
	if e.GetState().Status != EngineSuccess {
		t.Fatalf("status = %s, want success", e.GetState().Status)
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
