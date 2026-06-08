package app

import (
	"strings"
	"testing"
)

func TestParseAuthImportJSON(t *testing.T) {
	raw := `{
		"x_app_code": "app-code",
		"query_authorization": "Bearer query",
		"reservation_authorization": "Bearer reservation",
		"user_agent": "Mozilla/5.0 MicroMessenger/8.0 MiniProgramEnv/ios",
		"referer": "https://servicewechat.com/wx123/1/page-frame.html",
		"wechatId": "wx-user",
		"phoneNumber": "13800138000",
		"store_ids": ["3006"]
	}`
	tokens, sources, err := parseAuthImportText(raw)
	if err != nil {
		t.Fatalf("parseAuthImportText() error = %v", err)
	}
	finalizeImportedTokens(tokens)
	if err := tokens.ValidateForReservation(); err != nil {
		t.Fatalf("ValidateForReservation() error = %v", err)
	}
	if !containsString(sources, "json") {
		t.Fatalf("sources = %v, want json", sources)
	}
}

func TestParseAuthImportCurl(t *testing.T) {
	raw := `curl 'https://crm-cn-prd.sushiro.com.cn/wechat/api_auth/2.0/ticketing/take?storeId=3006' \
		-H 'X-App-Code: app-code' \
		-H 'Authorization: Bearer reservation' \
		-H 'User-Agent: Mozilla/5.0 (iPhone) MicroMessenger/8.0 MiniProgramEnv/ios' \
		-H 'Referer: https://servicewechat.com/wx123/1/page-frame.html' \
		--data-raw '{"wechatId":"wx-user","phoneNumber":"13800138000"}'`
	tokens, sources, err := parseAuthImportText(raw)
	if err != nil {
		t.Fatalf("parseAuthImportText() error = %v", err)
	}
	finalizeImportedTokens(tokens)
	if err := tokens.ValidateForReservation(); err != nil {
		t.Fatalf("ValidateForReservation() error = %v", err)
	}
	if !containsString(sources, "curl") {
		t.Fatalf("sources = %v, want curl", sources)
	}
}

func TestParseAuthImportReportsMissingFields(t *testing.T) {
	tokens, _, err := parseAuthImportText("X-App-Code: app-code")
	if err != nil {
		t.Fatalf("parseAuthImportText() error = %v", err)
	}
	finalizeImportedTokens(tokens)
	missing := strings.Join(tokens.MissingFields(true), ",")
	for _, want := range []string{"查询凭证", "预约凭证", "User-Agent", "Referer", "微信ID", "手机号", "门店"} {
		if !strings.Contains(missing, want) {
			t.Fatalf("missing = %q, want %q", missing, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
