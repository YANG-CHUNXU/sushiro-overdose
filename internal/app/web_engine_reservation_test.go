package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/Ryujoxys/sushiro-overdose/internal/api"
	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

func TestRefreshReservationItemsPreservesReservationTicketStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wechat/api_auth/2.0/ticket/status" {
			t.Fatalf("path = %q, want ticket/status", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reservationTicket":{"ticketId":7160,"number":"7160","status":"WAITING","queueDate":"20260611","start":"193000","end":"200000","wait":110,"storeId":"3006","store_name":"太阳宫凯德店"}}`))
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
	})
	client.SetHTTPClient(server.Client())

	items := refreshReservationItemsWithCurrentNetTicket(context.Background(), client, nil)

	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	got := items[0]
	if got.Kind != "reservation" {
		t.Fatalf("kind = %q, want reservation; record = %+v", got.Kind, got)
	}
	if isLocalNetTicketRecord(got) {
		t.Fatalf("reservation ticket must not be treated as local net ticket: %+v", got)
	}
	if netTicketLooksSuccessful(got) {
		t.Fatalf("reservation ticket must not count as net ticket success: %+v", got)
	}
}

func TestReservationWithWaitingStatusIsNotLocalNetTicket(t *testing.T) {
	record := ReservationRecord{
		Kind:      "reservation",
		Status:    "WAITING",
		Number:    "7160",
		QueueDate: "20260611",
		Start:     "193000",
		End:       "200000",
		Wait:      110,
	}

	if isLocalNetTicketRecord(record) {
		t.Fatalf("reservation with WAITING status/wait should not be local net ticket: %+v", record)
	}
	if netTicketLooksSuccessful(record) {
		t.Fatalf("reservation with WAITING status/wait should not count as net ticket success: %+v", record)
	}
}
