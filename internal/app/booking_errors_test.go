package app

import (
	"strings"
	"testing"

	api "github.com/Ryujoxys/sushiro-overdose/internal/api"
)

func TestKnownOfficialServerError(t *testing.T) {
	err := &api.APIError{
		StatusCode: 500,
		Body:       `{"code":"E010","message":"error.server:ご迷惑おかけします。サーバー側でエラーが発生しました。"}`,
	}
	if !isOfficialServerHTTPError(err) {
		t.Fatalf("isOfficialServerHTTPError() = false")
	}
	if !isKnownOfficialServerError(err) {
		t.Fatalf("isKnownOfficialServerError() = false")
	}
	msg := friendlyOfficialAPIError(err)
	if !strings.Contains(msg, "E010") || strings.Contains(msg, "重新捕获") {
		t.Fatalf("friendly message = %q", msg)
	}
}

func TestNonServerErrorIsNotOfficialServerError(t *testing.T) {
	err := &api.APIError{StatusCode: 403, Body: `{"message":"forbidden"}`}
	if isOfficialServerHTTPError(err) {
		t.Fatalf("403 should not be treated as official server error")
	}
	if isKnownOfficialServerError(err) {
		t.Fatalf("403 should not be known server error")
	}
}
