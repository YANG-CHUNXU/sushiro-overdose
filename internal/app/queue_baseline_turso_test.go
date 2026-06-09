package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func resetQueueBaselineRemoteCacheForTest(t *testing.T) {
	t.Helper()
	queueBaselineRemoteCache.Lock()
	queueBaselineRemoteCache.entries = map[string]queueBaselineRemoteCacheEntry{}
	queueBaselineRemoteCache.Unlock()
	t.Cleanup(func() {
		queueBaselineRemoteCache.Lock()
		queueBaselineRemoteCache.entries = map[string]queueBaselineRemoteCacheEntry{}
		queueBaselineRemoteCache.Unlock()
	})
}

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
				{"type":"integer","value":"180"},
				{"type":"integer","value":"540"},
				{"type":"text","value":"[{\"queueType\":\"A\",\"currentCalledNo\":540}]"}
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
			{"type":"integer","value":"11"},
			{"type":"float","value":510},
			{"type":"float","value":540},
			{"type":"float","value":580},
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
	if export.Latest[0].DisplayCalledNo != 540 || !strings.Contains(export.Latest[0].GroupQueuesJSON, "currentCalledNo") {
		t.Fatalf("latest called fields = %+v", export.Latest[0])
	}
	if len(export.Rollups) != 1 || export.Rollups[0].WaitTypicalMinutes == nil || *export.Rollups[0].WaitTypicalMinutes != 35 {
		t.Fatalf("rollups = %+v", export.Rollups)
	}
	if export.Rollups[0].CalledSampleCount != 11 || export.Rollups[0].CalledNoTypical == nil || *export.Rollups[0].CalledNoTypical != 540 {
		t.Fatalf("rollup called fields = %+v", export.Rollups[0])
	}
	if export.Stats.SourceUpdatedAt != "2026-06-03T22:30:00+08:00" {
		t.Fatalf("source updated at = %s", export.Stats.SourceUpdatedAt)
	}
}

func TestFetchQueueBaselineTursoFallsBackToOldSchema(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req tursoPipelineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		sql := *req.Requests[0].Stmt.SQL
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(sql, "display_called_no") || strings.Contains(sql, "called_sample_count") {
			w.Write([]byte(`{"results":[{"type":"error","error":{"code":"SQLITE_ERROR","message":"no such column"}}]}`))
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
	cfg := queueBaselineTursoConfig{DatabaseURL: server.URL, AuthToken: "test-token"}
	latest, _, err := fetchQueueBaselineLatest(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	rollups, _, err := fetchQueueBaselineRollups(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want 4", requests)
	}
	if len(latest) != 1 || latest[0].DisplayCalledNo != 0 {
		t.Fatalf("latest fallback = %+v", latest)
	}
	if len(rollups) != 1 || rollups[0].CalledSampleCount != 0 {
		t.Fatalf("rollup fallback = %+v", rollups)
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

func TestFetchQueueBaselineFromCloud(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/baseline/export" {
			t.Fatalf("path = %s, want /api/queue/baseline/export", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-session" {
			t.Fatalf("Authorization = %q", got)
		}
		writeJSON(w, QueueBaselineExport{
			Version:       1,
			GeneratedAt:   "2026-06-08T10:00:00+08:00",
			Source:        "turso-cloudflare",
			BucketMinutes: 10,
			DateTypes:     []string{"weekday"},
			Stores:        []QueueBaselineStore{{StoreID: 3006, Name: "太阳宫凯德店"}},
			Latest:        []QueueBaselineLatest{{StoreID: 3006, Name: "太阳宫凯德店", CollectedAt: "2026-06-08T10:00:00+08:00"}},
			Rollups:       []QueueBaselineRollup{{StoreID: 3006, DateType: "weekday", Weekday: 1, TimeBucket: "10:00", SampleCount: 5, Confidence: "medium"}},
			Stats: QueueBaselineStats{
				StoreCount:      1,
				LatestCount:     1,
				RollupCount:     1,
				SourceUpdatedAt: "2026-06-08T10:00:00+08:00",
			},
		})
	}))
	defer server.Close()

	export, err := fetchQueueBaselineFromCloud(context.Background(), CloudAuthConfig{
		BaseURL:      server.URL,
		SessionToken: "cloud-session",
	}, "", time.Date(2026, 6, 8, 10, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if export.Source != "turso-cloudflare" || export.Stats.StoreCount != 1 || len(export.Rollups) != 1 {
		t.Fatalf("export = %+v", export)
	}
}

func TestFetchQueueBaselineFromCloudForStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/baseline/store" {
			t.Fatalf("path = %s, want /api/queue/baseline/store", r.URL.Path)
		}
		if got := r.URL.Query().Get("store_id"); got != "3006" {
			t.Fatalf("store_id = %q, want 3006", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-session" {
			t.Fatalf("Authorization = %q", got)
		}
		writeJSON(w, queueBaselineCloudExportForTest(3006))
	}))
	defer server.Close()

	export, err := fetchQueueBaselineFromCloud(context.Background(), CloudAuthConfig{
		BaseURL:      server.URL,
		SessionToken: "cloud-session",
	}, "3006", time.Date(2026, 6, 8, 10, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if export.Stats.StoreCount != 1 || len(export.Stores) != 1 || export.Stores[0].StoreID != 3006 {
		t.Fatalf("export = %+v", export)
	}
}

func TestLoadRemoteQueuePressureBaselineCloudDoesNotUseFullCache(t *testing.T) {
	resetQueueBaselineRemoteCacheForTest(t)
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	cfg := CloudAuthConfig{BaseURL: "https://cloud.example", SessionToken: "cloud-session"}
	queueBaselineRemoteCache.Lock()
	queueBaselineRemoteCache.entries = map[string]queueBaselineRemoteCacheEntry{
		cfg.cacheKey(): {
			key:      cfg.cacheKey(),
			loadedAt: now,
			export:   queueBaselineCloudExportForTest(9999),
		},
	}
	queueBaselineRemoteCache.Unlock()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/queue/baseline/store" {
			t.Fatalf("path = %s, want /api/queue/baseline/store", r.URL.Path)
		}
		if got := r.URL.Query().Get("store_id"); got != "3006" {
			t.Fatalf("store_id = %q, want 3006", got)
		}
		writeJSON(w, queueBaselineCloudExportForTest(3006))
	}))
	defer server.Close()
	cfg.BaseURL = server.URL

	export, status, err := loadRemoteQueuePressureBaselineCloud(context.Background(), cfg, "3006", now)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !status.Used || export.Stats.StoreCount != 1 || len(export.Stores) != 1 || export.Stores[0].StoreID != 3006 {
		t.Fatalf("status = %+v export = %+v", status, export)
	}
}

func TestLoadRemoteQueueBaselineForStoresUsesCloudStoreEndpoints(t *testing.T) {
	resetQueueBaselineRemoteCacheForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(queueBaselineTursoURLEnv, "")
	t.Setenv(queueBaselineTursoTokenEnv, "")
	t.Setenv(queueBaselineTursoFallbackURL, "")
	t.Setenv(queueBaselineTursoFallbackAuth, "")
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/baseline/store" {
			t.Fatalf("path = %s, want /api/queue/baseline/store", r.URL.Path)
		}
		storeID := r.URL.Query().Get("store_id")
		calls = append(calls, storeID)
		switch storeID {
		case "3006":
			writeJSON(w, queueBaselineCloudExportForTest(3006))
		case "3015":
			writeJSON(w, queueBaselineCloudExportForTest(3015))
		default:
			t.Fatalf("unexpected store_id = %q", storeID)
		}
	}))
	defer server.Close()
	if err := SaveCloudAuthConfig(CloudAuthConfig{BaseURL: server.URL, SessionToken: "cloud-session", UserLogin: "octocat"}); err != nil {
		t.Fatal(err)
	}

	export, status, err := loadRemoteQueueBaselineForStores(context.Background(), []string{"3006", "3015"}, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"3006", "3015"}) {
		t.Fatalf("calls = %+v", calls)
	}
	if !status.Used || status.StoreCount != 2 || export.Stats.StoreCount != 2 || len(export.Latest) != 2 || len(export.Rollups) != 2 {
		t.Fatalf("status = %+v export = %+v", status, export)
	}
}

func TestLoadRemoteQueueBaselineUsesCloudWhenTursoMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(queueBaselineTursoURLEnv, "")
	t.Setenv(queueBaselineTursoTokenEnv, "")
	t.Setenv(queueBaselineTursoFallbackURL, "")
	t.Setenv(queueBaselineTursoFallbackAuth, "")
	t.Setenv(cloudAuthURLEnv, "")
	t.Setenv(cloudAuthSessionTokenEnv, "")
	queueBaselineRemoteCache.Lock()
	queueBaselineRemoteCache.entries = map[string]queueBaselineRemoteCacheEntry{}
	queueBaselineRemoteCache.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/baseline/export" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		writeJSON(w, QueueBaselineExport{
			Version:       1,
			GeneratedAt:   "2026-06-08T10:00:00+08:00",
			Source:        "turso-cloudflare",
			BucketMinutes: 10,
			DateTypes:     []string{"weekday"},
			Stores:        []QueueBaselineStore{{StoreID: 3006, Name: "太阳宫凯德店"}},
			Stats:         QueueBaselineStats{StoreCount: 1},
		})
	}))
	defer server.Close()
	if err := SaveCloudAuthConfig(CloudAuthConfig{BaseURL: server.URL, SessionToken: "cloud-session", UserLogin: "octocat"}); err != nil {
		t.Fatal(err)
	}

	export, status, err := loadRemoteQueueBaselineCached(context.Background(), time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !status.Used || status.Provider != "cloudflare" || !status.Authenticated || status.UserLogin != "octocat" {
		t.Fatalf("status = %+v", status)
	}
	if export.Stats.StoreCount != 1 {
		t.Fatalf("export = %+v", export)
	}
}

func queueBaselineCloudExportForTest(storeID int) QueueBaselineExport {
	storeName := fmt.Sprintf("门店%d", storeID)
	return QueueBaselineExport{
		Version:       1,
		GeneratedAt:   "2026-06-08T10:00:00+08:00",
		Source:        "turso-cloudflare",
		BucketMinutes: 10,
		DateTypes:     []string{"weekday"},
		Stores:        []QueueBaselineStore{{StoreID: storeID, Name: storeName}},
		Latest:        []QueueBaselineLatest{{StoreID: storeID, Name: storeName, CollectedAt: "2026-06-08T10:00:00+08:00"}},
		Rollups:       []QueueBaselineRollup{{StoreID: storeID, DateType: "weekday", Weekday: 1, TimeBucket: "10:00", SampleCount: 5, Confidence: "medium"}},
		Stats: QueueBaselineStats{
			StoreCount:      1,
			LatestCount:     1,
			RollupCount:     1,
			SourceUpdatedAt: "2026-06-08T10:00:00+08:00",
		},
	}
}
