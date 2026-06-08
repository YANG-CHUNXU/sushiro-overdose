package app

import (
	"errors"
	"testing"
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
