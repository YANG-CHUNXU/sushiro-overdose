package app

import (
	"errors"
	"strings"
	"testing"

	api "github.com/Ryujoxys/sushiro-overdose/internal/api"
)

func TestAuthHealthStateMachine(t *testing.T) {
	authHealth = &authHealthTracker{status: authHealthUnknown}
	t.Cleanup(func() { authHealth = &authHealthTracker{status: authHealthUnknown} })

	if got := getAuthHealth().Status; got != authHealthUnknown {
		t.Fatalf("initial status = %q, want %q", got, authHealthUnknown)
	}

	markAuthStale("test 401")
	h := getAuthHealth()
	if h.Status != authHealthStale {
		t.Fatalf("after markAuthStale status = %q, want %q", h.Status, authHealthStale)
	}
	if h.Reason != "test 401" {
		t.Fatalf("reason = %q, want %q", h.Reason, "test 401")
	}
	if h.CheckedAt == "" {
		t.Fatalf("checked_at should be set after a transition")
	}

	markAuthHealthy()
	h = getAuthHealth()
	if h.Status != authHealthOK {
		t.Fatalf("after markAuthHealthy status = %q, want %q", h.Status, authHealthOK)
	}
	if h.Reason != "" {
		t.Fatalf("reason should be cleared after healthy, got %q", h.Reason)
	}

	// noteAuthResult(nil) → healthy
	markAuthStale("x")
	noteAuthResult(nil)
	if got := getAuthHealth().Status; got != authHealthOK {
		t.Fatalf("noteAuthResult(nil) status = %q, want %q", got, authHealthOK)
	}

	// 非凭证错误不改变凭证健康
	noteAuthResult(errors.New("some non-auth failure"))
	if got := getAuthHealth().Status; got != authHealthOK {
		t.Fatalf("noteAuthResult(non-auth) changed status to %q, want %q", got, authHealthOK)
	}
}

func TestAuthHealthMarksKnownServerErrorStale(t *testing.T) {
	authHealth = &authHealthTracker{status: authHealthUnknown}
	t.Cleanup(func() { authHealth = &authHealthTracker{status: authHealthUnknown} })

	noteAuthResult(&api.APIError{
		StatusCode: 500,
		Body:       `{"code":"E010","message":"error.server"}`,
	})

	h := getAuthHealth()
	if h.Status != authHealthStale {
		t.Fatalf("status = %q, want %q", h.Status, authHealthStale)
	}
	if !strings.Contains(h.Reason, "E010") {
		t.Fatalf("reason = %q, want E010 context", h.Reason)
	}
}
