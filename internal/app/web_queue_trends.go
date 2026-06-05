package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func handleQueueTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	q := r.URL.Query()
	query := QueueTrendQuery{
		StoreIDs:      queueTrendRequestStores(q["store"], q.Get("stores")),
		DateType:      q.Get("date_type"),
		From:          q.Get("from"),
		To:            q.Get("to"),
		Start:         q.Get("start"),
		End:           q.Get("end"),
		BucketMinutes: atoiDefault(q.Get("bucket"), 30),
	}
	writeJSON(w, BuildQueueTrendsWithContext(r.Context(), query, time.Now()))
}

func handleQueueDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	q := r.URL.Query()
	query := QueueDashboardQuery{
		StoreIDs:      queueTrendRequestStores(q["store"], q.Get("stores")),
		Scope:         q.Get("scope"),
		DateType:      q.Get("date_type"),
		WindowHours:   atoiDefault(q.Get("window"), queueDashboardDefaultWindowHours),
		BucketMinutes: atoiDefault(q.Get("bucket"), queueDashboardDefaultBucketMins),
		TargetNo:      atoiDefault(q.Get("target_no"), 0),
	}
	writeJSON(w, BuildQueueDashboardWithContext(r.Context(), query, time.Now()))
}

func queueTrendRequestStores(storeValues []string, storesValue string) []string {
	raw := append([]string{}, storeValues...)
	raw = append(raw, strings.Split(storesValue, ",")...)
	return UniqueNonEmptyStrings(raw)
}

func atoiDefault(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
