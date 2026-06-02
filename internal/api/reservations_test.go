package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

func TestGetReservationsTreatsNotFoundAsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wechat/api_auth/2.0/ticketing/getReservations" {
			t.Fatalf("path = %q, want getReservations", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not Found"}`))
	}))
	defer server.Close()

	client := NewClient(Settings{
		BaseURL:         server.URL,
		ReservationAuth: "reservation-auth",
		XAppCode:        "app-code",
		XAppClient:      "miniapp",
		UserAgent:       "ua",
		Referer:         "ref",
		WechatID:        "wechat",
		PhoneNumber:     "phone",
	})

	_, err := client.GetReservations(context.Background())
	if !errors.Is(err, ErrReservationsEndpointUnavailable) {
		t.Fatalf("GetReservations() error = %v, want ErrReservationsEndpointUnavailable", err)
	}
}
