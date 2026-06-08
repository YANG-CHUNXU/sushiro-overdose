package app

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeReservationsProbeTreatsNotFoundAsSkipped(t *testing.T) {
	result := AuthProbeResult{
		Name:   "凭证接口：当前预约",
		Method: http.MethodPost,
		Path:   "/wechat/api_auth/2.0/ticketing/getReservations",
		Status: http.StatusNotFound,
		Detail: `{"error":"Not Found"}`,
	}

	got := normalizeReservationsProbeResult(result)

	if !got.Skipped {
		t.Fatalf("Skipped = false, want true")
	}
	if got.OK {
		t.Fatalf("OK = true, want false for skipped probe")
	}
	if !strings.Contains(got.Detail, "当前预约查询接口不可用") {
		t.Fatalf("Detail = %q", got.Detail)
	}
}
