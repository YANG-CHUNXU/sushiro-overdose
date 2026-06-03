package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchQueueBaselineFromTurso(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v2/pipeline" {
			t.Fatalf("path = %s, want /v2/pipeline", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		var req tursoPipelineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Requests) != 1 || req.Requests[0].Stmt.SQL == nil {
			t.Fatalf("unexpected request: %+v", req)
		}
		sql := *req.Requests[0].Stmt.SQL
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(sql, "FROM store_dimension") {
			w.Write([]byte(`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[[
				{"type":"integer","value":"3015"},
				{"type":"text","value":"深圳店"},
				{"type":"text","value":"深圳"},
				{"type":"text","value":"南山区"},
				{"type":"text","value":"地址"},
				{"type":"float","value":22.5},
				{"type":"float","value":114.0},
				{"type":"text","value":"2026-01-01"},
				{"type":"integer","value":"10"},
				{"type":"integer","value":"2"},
				{"type":"text","value":"2026-06-03T22:00:00+08:00"}
			]]}}}]}`))
			return
		}
		if strings.Contains(sql, "FROM store_latest") {
			w.Write([]byte(`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[[
				{"type":"integer","value":"3015"},
				{"type":"text","value":"2026-06-03T22:20:00+08:00"},
				{"type":"text","value":"深圳店"},
				{"type":"text","value":"深圳"},
				{"type":"text","value":"南山区"},
				{"type":"integer","value":"28"},
				{"type":"integer","value":"24"},
				{"type":"text","value":"OPEN"},
				{"type":"text","value":"ONLINE"},
				{"type":"text","value":"ON"},
				{"type":"integer","value":"1"},
				{"type":"integer","value":"0"},
				{"type":"integer","value":"180"}
			]]}}}]}`))
			return
		}
		w.Write([]byte(`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[[
			{"type":"integer","value":"3015"},
			{"type":"text","value":"weekend"},
			{"type":"integer","value":"6"},
			{"type":"text","value":"18:30"},
			{"type":"integer","value":"12"},
			{"type":"float","value":1},
			{"type":"float","value":0.8},
			{"type":"float","value":0.7},
			{"type":"float","value":35},
			{"type":"float","value":50},
			{"type":"integer","value":"80"},
			{"type":"float","value":22},
			{"type":"float","value":38},
			{"type":"text","value":"high"},
			{"type":"text","value":"2026-06-03T22:30:00+08:00"}
		]]}}}]}`))
	}))
	defer server.Close()

	export, err := fetchQueueBaselineFromTurso(context.Background(), queueBaselineTursoConfig{
		DatabaseURL: server.URL,
		AuthToken:   "test-token",
	}, time.Date(2026, 6, 3, 22, 30, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
	if len(export.Stores) != 1 || export.Stores[0].StoreID != 3015 || export.Stores[0].Latitude == nil {
		t.Fatalf("stores = %+v", export.Stores)
	}
	if len(export.Latest) != 1 || export.Latest[0].WaitMinutes != 28 || !export.Latest[0].OnlineOpen {
		t.Fatalf("latest = %+v", export.Latest)
	}
	if len(export.Rollups) != 1 || export.Rollups[0].WaitTypicalMinutes == nil || *export.Rollups[0].WaitTypicalMinutes != 35 {
		t.Fatalf("rollups = %+v", export.Rollups)
	}
	if export.Stats.SourceUpdatedAt != "2026-06-03T22:30:00+08:00" {
		t.Fatalf("source updated at = %s", export.Stats.SourceUpdatedAt)
	}
}

func TestQueueBaselineRemoteConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(queueBaselineTursoURLEnv, "")
	t.Setenv(queueBaselineTursoTokenEnv, "")
	t.Setenv(queueBaselineTursoFallbackURL, "")
	t.Setenv(queueBaselineTursoFallbackAuth, "")

	want := QueueBaselineRemoteConfig{
		DatabaseURL: "libsql://example.turso.io",
		AuthToken:   "readonly-token",
	}
	if err := SaveQueueBaselineRemoteConfig(want); err != nil {
		t.Fatal(err)
	}
	got := LoadQueueBaselineRemoteConfig()
	if got != want {
		t.Fatalf("LoadQueueBaselineRemoteConfig() = %+v, want %+v", got, want)
	}
}
