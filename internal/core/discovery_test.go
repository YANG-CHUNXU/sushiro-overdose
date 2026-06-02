package core

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestAPIDiscoveryPayloadKeysExtractsOnlyKeys(t *testing.T) {
	jsonKeys := APIDiscoveryPayloadKeys([]byte(`{"phone_number":"13800138000","wechat_id":"wx-secret","storeId":"1012"}`))
	if want := []string{"phone_number", "storeId", "wechat_id"}; !reflect.DeepEqual(jsonKeys, want) {
		t.Fatalf("json keys = %#v, want %#v", jsonKeys, want)
	}

	formKeys := APIDiscoveryPayloadKeys([]byte(`Authorization=Bearer-secret&storeId=1012&phone_number=13800138000`))
	if want := []string{"Authorization", "phone_number", "storeId"}; !reflect.DeepEqual(formKeys, want) {
		t.Fatalf("form keys = %#v, want %#v", formKeys, want)
	}
}

func TestBuildAPIDiscoveryRecordDoesNotPersistSensitiveValues(t *testing.T) {
	target, err := url.Parse("https://crm-cn-prd.sushiro.com.cn/wechat/api_auth/2.0/ticketing/getReservations?phone_number=13800138000&token=secret-token&storeId=1012")
	if err != nil {
		t.Fatal(err)
	}
	requestKeys := APIDiscoveryPayloadKeys([]byte(`{"Authorization":"Bearer real-token","phone_number":"13800138000","wechat_id":"wx-secret","storeId":"1012"}`))
	record := BuildAPIDiscoveryRecord(
		"post",
		target,
		404,
		"HTTP/2.0",
		requestKeys,
		[]byte(`{"error":"Not Found","message":"13800138000 wx-secret real-token","data":[{"reservationId":"rv-secret","status":"active"}]}`),
	)

	if record.Path != "/wechat/api_auth/2.0/ticketing/getReservations" {
		t.Fatalf("path = %q", record.Path)
	}
	if want := []string{"phone_number", "storeId", "token"}; !reflect.DeepEqual(record.QueryKeys, want) {
		t.Fatalf("query keys = %#v, want %#v", record.QueryKeys, want)
	}
	if want := []string{"Authorization", "phone_number", "storeId", "wechat_id"}; !reflect.DeepEqual(record.RequestBodyKeys, want) {
		t.Fatalf("request body keys = %#v, want %#v", record.RequestBodyKeys, want)
	}
	if want := []string{"data", "error", "message"}; !reflect.DeepEqual(record.ResponseKeys, want) {
		t.Fatalf("response keys = %#v, want %#v", record.ResponseKeys, want)
	}
	if want := []string{"reservationId", "status"}; !reflect.DeepEqual(record.ResponseDataArrayItemKeys, want) {
		t.Fatalf("response data item keys = %#v, want %#v", record.ResponseDataArrayItemKeys, want)
	}

	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, sensitive := range []string{"13800138000", "wx-secret", "real-token", "secret-token", "rv-secret"} {
		if strings.Contains(text, sensitive) {
			t.Fatalf("record leaked sensitive value %q: %s", sensitive, text)
		}
	}
}
