package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	errNoReservationAvailable = errors.New("no reservation available")
	chineseWeekdayNames       = [...]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
)

type Settings struct {
	StoreIDs           []string
	Adult              int
	Child              int
	TableType          string
	Debug              bool
	PhoneNumber        string
	WechatID           string
	XAppCode           string
	QueryAuthorization string
	ReservationAuth    string
	XAppClient         string
	UserAgent          string
	Referer            string
	FeishuWebhook      string
	StateFile          string
	Timezone           string
	Location           *time.Location
	PollInterval       time.Duration
	AvailableStatuses  map[string]struct{}
	BaseURL            string
}

func (s Settings) NumPersons() int {
	return s.Adult + s.Child
}

type rawConfig struct {
	StoreIDs            []string `json:"store_ids"`
	StoreID             string   `json:"store_id"`
	Adult               *int     `json:"adult"`
	Child               *int     `json:"child"`
	TableType           string   `json:"table_type"`
	Debug               bool     `json:"debug"`
	PhoneNumber         string   `json:"phone_number"`
	WechatID            string   `json:"wechat_id"`
	XAppCode            string   `json:"x_app_code"`
	QueryAuthorization  string   `json:"query_authorization"`
	ReservationAuth     string   `json:"reservation_authorization"`
	Authorization       string   `json:"authorization"`
	XAppClient          string   `json:"x_app_client"`
	UserAgent           string   `json:"user_agent"`
	Referer             string   `json:"referer"`
	FeishuWebhook       string   `json:"feishu_webhook"`
	StateFile           string   `json:"state_file"`
	Timezone            string   `json:"timezone"`
	PollIntervalSeconds *float64 `json:"poll_interval_seconds"`
	AvailableStatuses   []string `json:"available_statuses"`
	BaseURL             string   `json:"base_url"`
}

func LoadSettings(configPath string) (Settings, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Settings{}, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Settings{}, fmt.Errorf("invalid JSON in config file %s: %w", configPath, err)
	}

	if value := os.Getenv("SUSHIRO_PHONE_NUMBER"); value != "" {
		raw.PhoneNumber = value
	}
	if value := os.Getenv("SUSHIRO_WECHAT_ID"); value != "" {
		raw.WechatID = value
	}
	if value := os.Getenv("SUSHIRO_X_APP_CODE"); value != "" {
		raw.XAppCode = value
	}
	if value := os.Getenv("SUSHIRO_QUERY_AUTHORIZATION"); value != "" {
		raw.QueryAuthorization = value
	} else if value := os.Getenv("SUSHIRO_AUTHORIZATION"); value != "" && raw.QueryAuthorization == "" {
		raw.QueryAuthorization = value
	}
	if value := os.Getenv("SUSHIRO_RESERVATION_AUTHORIZATION"); value != "" {
		raw.ReservationAuth = value
	} else if value := os.Getenv("SUSHIRO_AUTHORIZATION"); value != "" && raw.ReservationAuth == "" {
		raw.ReservationAuth = value
	}
	if value := os.Getenv("SUSHIRO_FEISHU_WEBHOOK"); value != "" {
		raw.FeishuWebhook = value
	}

	if raw.Authorization != "" {
		if raw.QueryAuthorization == "" {
			raw.QueryAuthorization = raw.Authorization
		}
		if raw.ReservationAuth == "" {
			raw.ReservationAuth = raw.Authorization
		}
	}

	storeIDs := normalizeStoreIDs(raw.StoreIDs, raw.StoreID)
	if len(storeIDs) == 0 {
		return Settings{}, fmt.Errorf("at least one store ID must be configured in store_ids")
	}

	adult := 2
	if raw.Adult != nil {
		adult = *raw.Adult
	}
	child := 0
	if raw.Child != nil {
		child = *raw.Child
	}
	tableType := strings.TrimSpace(raw.TableType)
	if tableType == "" {
		tableType = "T"
	}
	xAppClient := strings.TrimSpace(raw.XAppClient)
	if xAppClient == "" {
		xAppClient = "miniapp"
	}
	timezone := strings.TrimSpace(raw.Timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Settings{}, fmt.Errorf("load timezone %s: %w", timezone, err)
	}

	pollIntervalSeconds := 60.0
	if raw.PollIntervalSeconds != nil {
		pollIntervalSeconds = *raw.PollIntervalSeconds
	}
	if pollIntervalSeconds <= 0 {
		return Settings{}, fmt.Errorf("poll_interval_seconds must be greater than zero")
	}

	stateFile := strings.TrimSpace(raw.StateFile)
	if stateFile == "" {
		stateFile = ".sushiro_state.json"
	}
	if !filepath.IsAbs(stateFile) {
		stateFile = filepath.Join(filepath.Dir(configPath), stateFile)
	}
	stateFile, err = filepath.Abs(stateFile)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve state file: %w", err)
	}

	baseURL := strings.TrimSpace(raw.BaseURL)
	if baseURL == "" {
		baseURL = "https://crm-cn-prd.sushiro.com.cn"
	}

	availableStatuses := normalizeStatusSet(raw.AvailableStatuses)
	if len(availableStatuses) == 0 {
		availableStatuses["AVAILABLE"] = struct{}{}
	}

	settings := Settings{
		StoreIDs:           storeIDs,
		Adult:              adult,
		Child:              child,
		TableType:          tableType,
		Debug:              raw.Debug,
		PhoneNumber:        strings.TrimSpace(raw.PhoneNumber),
		WechatID:           strings.TrimSpace(raw.WechatID),
		XAppCode:           strings.TrimSpace(raw.XAppCode),
		QueryAuthorization: strings.TrimSpace(raw.QueryAuthorization),
		ReservationAuth:    strings.TrimSpace(raw.ReservationAuth),
		XAppClient:         xAppClient,
		UserAgent:          strings.TrimSpace(raw.UserAgent),
		Referer:            strings.TrimSpace(raw.Referer),
		FeishuWebhook:      strings.TrimSpace(raw.FeishuWebhook),
		StateFile:          stateFile,
		Timezone:           timezone,
		Location:           location,
		PollInterval:       time.Duration(pollIntervalSeconds * float64(time.Second)),
		AvailableStatuses:  availableStatuses,
		BaseURL:            strings.TrimRight(baseURL, "/"),
	}

	if err := settings.Validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func normalizeStoreIDs(values []string, fallback string) []string {
	if len(values) == 0 && strings.TrimSpace(fallback) != "" {
		values = []string{fallback}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeStatusSet(values []string) map[string]struct{} {
	statuses := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		statuses[normalized] = struct{}{}
	}
	return statuses
}

func (s Settings) Validate() error {
	required := map[string]string{
		"phone_number":              s.PhoneNumber,
		"wechat_id":                 s.WechatID,
		"x_app_code":                s.XAppCode,
		"query_authorization":       s.QueryAuthorization,
		"reservation_authorization": s.ReservationAuth,
		"user_agent":                s.UserAgent,
		"referer":                   s.Referer,
	}
	missing := make([]string, 0)
	for key, value := range required {
		if value == "" || strings.HasPrefix(value, "REPLACE_") {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("missing required config values: %s", strings.Join(missing, ", "))
	}
	if len(s.StoreIDs) == 0 {
		return fmt.Errorf("at least one store ID must be configured in store_ids")
	}
	if s.Adult < 0 || s.Child < 0 {
		return fmt.Errorf("adult and child must be zero or greater")
	}
	if s.NumPersons() <= 0 {
		return fmt.Errorf("adult + child must be greater than zero")
	}
	if s.PollInterval <= 0 {
		return fmt.Errorf("poll_interval_seconds must be greater than zero")
	}
	return nil
}

type Slot struct {
	StoreID      string `json:"storeId"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Availability string `json:"availability"`
}

type StoreInfo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type ReservationRecord struct {
	Status           string `json:"status,omitempty"`
	Start            string `json:"start,omitempty"`
	End              string `json:"end,omitempty"`
	Number           string `json:"number,omitempty"`
	QueueTime        string `json:"queueTime,omitempty"`
	StoreID          string `json:"storeId,omitempty"`
	CheckedIn        bool   `json:"checkedIn,omitempty"`
	TableType        string `json:"tableType,omitempty"`
	NumAdult         int    `json:"numAdult,omitempty"`
	NumChild         int    `json:"numChild,omitempty"`
	TicketID         int64  `json:"ticketId,omitempty"`
	Wait             int    `json:"wait,omitempty"`
	QueueDate        string `json:"queueDate,omitempty"`
	MonitoredStoreID string `json:"monitored_store_id,omitempty"`
	StoreName        string `json:"store_name,omitempty"`
	StoreAddress     string `json:"store_address,omitempty"`
	SlotLabel        string `json:"slot_label,omitempty"`
}

type State struct {
	ActiveReservation      *ReservationRecord `json:"active_reservation,omitempty"`
	NotificationSent       bool               `json:"notification_sent,omitempty"`
	SavedAt                string             `json:"saved_at,omitempty"`
	NotifiedAt             string             `json:"notified_at,omitempty"`
	LastWeekendSummaryHour string             `json:"last_weekend_summary_hour,omitempty"`
	LastWeekendSummaryAt   string             `json:"last_weekend_summary_at,omitempty"`
}

func (s State) IsZero() bool {
	return s.ActiveReservation == nil &&
		!s.NotificationSent &&
		s.SavedAt == "" &&
		s.NotifiedAt == "" &&
		s.LastWeekendSummaryHour == "" &&
		s.LastWeekendSummaryAt == ""
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("invalid JSON in state file %s: %w", path, err)
	}
	return state, nil
}

func saveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func clearState(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}
	return nil
}

func pruneExpiredReservation(state *State, today time.Time) bool {
	if state.ActiveReservation == nil {
		return false
	}
	if activeReservation(*state, today) != nil {
		return false
	}
	state.ActiveReservation = nil
	state.NotificationSent = false
	state.SavedAt = ""
	state.NotifiedAt = ""
	return true
}

func activeReservation(state State, today time.Time) *ReservationRecord {
	if state.ActiveReservation == nil || strings.TrimSpace(state.ActiveReservation.QueueDate) == "" {
		return nil
	}
	reservationDay, err := parseCompactDate(state.ActiveReservation.QueueDate, today.Location())
	if err != nil {
		return nil
	}
	if reservationDay.Before(beginningOfDay(today)) {
		return nil
	}
	return state.ActiveReservation
}

func currentSummaryHourKey(now time.Time) string {
	return now.Format("2006-01-02T15")
}

func currentMinuteKey(now time.Time) string {
	return now.Format("2006-01-02T15:04")
}

func shouldSendWeekendSummary(state State, now time.Time) bool {
	return state.LastWeekendSummaryHour != currentSummaryHourKey(now)
}

func logMessage(now time.Time, message string) {
	fmt.Printf("[%s] %s\n", now.Format(time.RFC3339), message)
}

func ensureBearer(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func parseCompactDate(raw string, loc *time.Location) (time.Time, error) {
	if len(raw) != 8 {
		return time.Time{}, fmt.Errorf("invalid date: %s", raw)
	}
	return time.ParseInLocation("20060102", raw, loc)
}

func parseCompactTime(raw string) (int, int, int, error) {
	if len(raw) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid time: %s", raw)
	}
	hour, err := strconv.Atoi(raw[0:2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hour in %s: %w", raw, err)
	}
	minute, err := strconv.Atoi(raw[2:4])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minute in %s: %w", raw, err)
	}
	second, err := strconv.Atoi(raw[4:6])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid second in %s: %w", raw, err)
	}
	return hour, minute, second, nil
}

func formatCompactTime(raw string) string {
	hour, minute, _, err := parseCompactTime(raw)
	if err != nil {
		return raw
	}
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func formatSlotWindow(slotDate, start, end string, loc *time.Location) string {
	day, err := parseCompactDate(slotDate, loc)
	if err != nil {
		return slotDate + " " + formatCompactTime(start) + "-" + formatCompactTime(end)
	}
	return fmt.Sprintf("%s %s-%s", day.Format("2006-01-02"), formatCompactTime(start), formatCompactTime(end))
}

func formatDateWithWeekday(day time.Time) string {
	return fmt.Sprintf("%s（%s）", day.Format("2006-01-02"), chineseWeekdayNames[weekdayIndexMon0(day.Weekday())])
}

func weekdayIndexMon0(day time.Weekday) int {
	if day == time.Sunday {
		return 6
	}
	return int(day) - 1
}

func beginningOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func startOfWeek(now time.Time) time.Time {
	return beginningOfDay(now).AddDate(0, 0, -weekdayIndexMon0(now.Weekday()))
}

func slotDateTime(slot Slot, loc *time.Location) (time.Time, error) {
	day, err := parseCompactDate(slot.Date, loc)
	if err != nil {
		return time.Time{}, err
	}
	hour, minute, second, err := parseCompactTime(slot.Start)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, 0, loc), nil
}

func isBookableStatus(status string, allowedStatuses map[string]struct{}) bool {
	_, ok := allowedStatuses[strings.ToUpper(strings.TrimSpace(status))]
	return ok
}

func targetWeekendDateSet(today time.Time) map[string]struct{} {
	start := startOfWeek(today)
	result := map[string]struct{}{}
	for _, offset := range []int{5, 6} {
		target := start.AddDate(0, 0, offset)
		if !target.Before(beginningOfDay(today)) {
			result[target.Format("2006-01-02")] = struct{}{}
		}
	}
	return result
}

func filterCandidateSlots(slots []Slot, now time.Time, allowedStatuses map[string]struct{}, debug bool) []Slot {
	targets := targetWeekendDateSet(now)
	if !debug && len(targets) == 0 {
		return nil
	}

	result := make([]Slot, 0)
	for _, slot := range slots {
		if !isBookableStatus(slot.Availability, allowedStatuses) {
			continue
		}
		slotStart, err := slotDateTime(slot, now.Location())
		if err != nil || !slotStart.After(now) {
			continue
		}
		if !debug {
			if _, ok := targets[slotStart.Format("2006-01-02")]; !ok {
				continue
			}
		}
		result = append(result, slot)
	}
	sortSlots(result, now.Location())
	return result
}

func filterMonthlyWeekendSlots(slots []Slot, now time.Time, allowedStatuses map[string]struct{}, daysAhead int) []Slot {
	limit := now.AddDate(0, 0, daysAhead)
	result := make([]Slot, 0)
	for _, slot := range slots {
		if !isBookableStatus(slot.Availability, allowedStatuses) {
			continue
		}
		slotStart, err := slotDateTime(slot, now.Location())
		if err != nil || !slotStart.After(now) || slotStart.After(limit) {
			continue
		}
		if slotStart.Weekday() != time.Saturday && slotStart.Weekday() != time.Sunday {
			continue
		}
		result = append(result, slot)
	}
	sortSlots(result, now.Location())
	return result
}

func filterCurrentWeekWeekdayEveningSlots(slots []Slot, now time.Time, allowedStatuses map[string]struct{}) []Slot {
	weekStart := startOfWeek(now)
	weekEnd := weekStart.AddDate(0, 0, 4)
	result := make([]Slot, 0)
	for _, slot := range slots {
		if !isBookableStatus(slot.Availability, allowedStatuses) {
			continue
		}
		slotStart, err := slotDateTime(slot, now.Location())
		if err != nil || !slotStart.After(now) {
			continue
		}
		if slotStart.Weekday() == time.Saturday || slotStart.Weekday() == time.Sunday {
			continue
		}
		if slotStart.Before(weekStart) || slotStart.After(weekEnd.Add(24*time.Hour-time.Nanosecond)) {
			continue
		}
		if slot.Start < "190000" {
			continue
		}
		result = append(result, slot)
	}
	sortSlots(result, now.Location())
	return result
}

func sortSlots(slots []Slot, loc *time.Location) {
	sort.Slice(slots, func(i, j int) bool {
		left, _ := slotDateTime(slots[i], loc)
		right, _ := slotDateTime(slots[j], loc)
		return left.Before(right)
	})
}

type StoreCandidate struct {
	StoreID string
	Slot    Slot
}

func collectCandidateSlotsByStore(timeslotsByStore map[string][]Slot, storeIDs []string, now time.Time, allowedStatuses map[string]struct{}, debug bool) []StoreCandidate {
	storeOrder := map[string]int{}
	for index, storeID := range storeIDs {
		storeOrder[storeID] = index
	}

	result := make([]StoreCandidate, 0)
	for _, storeID := range storeIDs {
		for _, slot := range filterCandidateSlots(timeslotsByStore[storeID], now, allowedStatuses, debug) {
			result = append(result, StoreCandidate{StoreID: storeID, Slot: slot})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		left, _ := slotDateTime(result[i].Slot, now.Location())
		right, _ := slotDateTime(result[j].Slot, now.Location())
		if left.Equal(right) {
			return storeOrder[result[i].StoreID] < storeOrder[result[j].StoreID]
		}
		return left.Before(right)
	})
	return result
}

func buildReservationSuccessCard(reservation ReservationRecord, generatedAt time.Time) map[string]any {
	storeName := fallbackString(reservation.StoreName, "未知门店")
	storeAddress := fallbackString(reservation.StoreAddress, "未提供")
	slotLabel := fallbackString(reservation.SlotLabel, "未提供")
	reservationNumber := fallbackString(reservation.Number, "未提供")

	detailLines := []string{
		fmt.Sprintf("**店名**：%s", storeName),
		fmt.Sprintf("**地址**：%s", storeAddress),
		fmt.Sprintf("**预约时间段**：%s", slotLabel),
		fmt.Sprintf("**预约号码**：`%s`", reservationNumber),
	}
	if strings.TrimSpace(reservation.Status) != "" {
		detailLines = append(detailLines, fmt.Sprintf("**状态**：%s", reservation.Status))
	}

	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "寿司郎预约成功",
			},
			"template": "green",
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("### %s\n预约已成功创建，请及时到店。", storeName),
				},
			},
			{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": strings.Join(detailLines, "\n"),
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "note",
				"elements": []map[string]any{
					{
						"tag":     "plain_text",
						"content": fmt.Sprintf("通知时间：%s", generatedAt.Format("2006-01-02 15:04:05")),
					},
				},
			},
		},
	}
}

func buildMultiStoreWeekendSummaryCard(storeSummaries []StoreSummary, generatedAt time.Time) map[string]any {
	elements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag": "lark_md",
				"content": strings.Join([]string{
					fmt.Sprintf("**更新时间**：%s", generatedAt.Format("2006-01-02 15:04:05")),
					fmt.Sprintf("**监控门店数**：%d", len(storeSummaries)),
					"**范围**：未来30天周末 `AVAILABLE` 时段",
				}, "\n"),
			},
		},
	}

	if len(storeSummaries) == 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": "当前未配置任何监控门店。",
			},
		})
	} else {
		for _, summary := range storeSummaries {
			elements = append(elements, map[string]any{"tag": "hr"})
			storeName := fallbackString(summary.Store.Name, "未知门店")
			storeAddress := fallbackString(summary.Store.Address, "未提供")
			lines := []string{
				fmt.Sprintf("### %s", storeName),
				fmt.Sprintf("地址：%s", storeAddress),
			}
			grouped := groupSlotsByDate(summary.Slots, generatedAt.Location())
			if len(grouped) == 0 {
				lines = append(lines, "- 未来30天周末暂无可预约的 `AVAILABLE` 时段。")
			} else {
				dates := make([]string, 0, len(grouped))
				for key := range grouped {
					dates = append(dates, key)
				}
				sort.Strings(dates)
				for _, dateKey := range dates {
					lines = append(lines, fmt.Sprintf("- %s：%s", dateKey, strings.Join(grouped[dateKey], "、")))
				}
			}
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": strings.Join(lines, "\n"),
				},
			})
		}
	}

	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "未来30天周末可预约时段",
			},
			"template": "blue",
		},
		"elements": elements,
	}
}

type StoreSummary struct {
	Store StoreInfo
	Slots []Slot
}

func buildWeekdayEveningConsoleReport(storeSummaries []StoreSummary, now time.Time) string {
	lines := []string{
		"本周工作日晚7点后可预约情况",
		fmt.Sprintf("更新时间：%s", now.Format("2006-01-02 15:04:05")),
	}
	for _, summary := range storeSummaries {
		storeName := fallbackString(summary.Store.Name, "未知门店")
		grouped := groupSlotsByDate(summary.Slots, now.Location())
		if len(grouped) == 0 {
			lines = append(lines, fmt.Sprintf("%s：无", storeName))
			continue
		}
		dates := make([]string, 0, len(grouped))
		for key := range grouped {
			dates = append(dates, key)
		}
		sort.Strings(dates)
		chunks := make([]string, 0, len(dates))
		for _, dateKey := range dates {
			chunks = append(chunks, fmt.Sprintf("%s %s", dateKey, strings.Join(grouped[dateKey], "、")))
		}
		lines = append(lines, fmt.Sprintf("%s：%s", storeName, strings.Join(chunks, "；")))
	}
	return strings.Join(lines, "\n")
}

func groupSlotsByDate(slots []Slot, loc *time.Location) map[string][]string {
	grouped := map[string][]string{}
	for _, slot := range slots {
		day, err := parseCompactDate(slot.Date, loc)
		if err != nil {
			continue
		}
		key := formatDateWithWeekday(day)
		grouped[key] = append(grouped[key], fmt.Sprintf("%s-%s", formatCompactTime(slot.Start), formatCompactTime(defaultString(slot.End, slot.Start))))
	}
	for key := range grouped {
		sort.Strings(grouped[key])
	}
	return grouped
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

type Client struct {
	settings   Settings
	httpClient *http.Client
	mu         sync.Mutex
	storeCache map[string]StoreInfo
}

func NewClient(settings Settings) *Client {
	return &Client{
		settings: settings,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		storeCache: map[string]StoreInfo{},
	}
}

func (c *Client) GetStoreInfo(ctx context.Context, storeID string) (StoreInfo, error) {
	c.mu.Lock()
	if store, ok := c.storeCache[storeID]; ok {
		c.mu.Unlock()
		return store, nil
	}
	c.mu.Unlock()

	query := url.Values{}
	query.Set("storeId", storeID)
	target := c.settings.BaseURL + "/wechat/api/2.0/getStoreById?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, target, c.baseHeaders(c.settings.QueryAuthorization, ""), nil)
	if err != nil {
		return StoreInfo{}, err
	}

	var store StoreInfo
	if err := json.Unmarshal(body, &store); err != nil {
		return StoreInfo{}, fmt.Errorf("store info response is not a JSON object: %w", err)
	}
	c.mu.Lock()
	c.storeCache[storeID] = store
	c.mu.Unlock()
	return store, nil
}

func (c *Client) GetTimeslots(ctx context.Context, storeID string) ([]Slot, error) {
	query := url.Values{}
	query.Set("tableType", c.settings.TableType)
	query.Set("storeId", storeID)
	query.Set("numpersons", strconv.Itoa(c.settings.NumPersons()))
	target := c.settings.BaseURL + "/wechat/api/2.0/store/timeslots?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, target, c.baseHeaders(c.settings.QueryAuthorization, ""), nil)
	if err != nil {
		return nil, err
	}

	var slots []Slot
	if err := json.Unmarshal(body, &slots); err != nil {
		return nil, fmt.Errorf("timeslots response is not a JSON array: %w", err)
	}
	return slots, nil
}

func (c *Client) CreateReservation(ctx context.Context, storeID, slotDate, slotTime string) (ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/createReservation"
	payload := map[string]any{
		"storeId":     storeID,
		"adult":       c.settings.Adult,
		"child":       c.settings.Child,
		"tableType":   c.settings.TableType,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
		"date":        slotDate,
		"time":        slotTime,
	}
	body, err := c.doJSON(ctx, http.MethodPost, target, c.baseHeaders(c.settings.ReservationAuth, "application/json"), payload)
	if err != nil {
		if strings.Contains(err.Error(), "E044") || strings.Contains(err.Error(), "no_more_reservations") {
			return ReservationRecord{}, errNoReservationAvailable
		}
		return ReservationRecord{}, err
	}

	var reservation ReservationRecord
	if err := json.Unmarshal(body, &reservation); err != nil {
		return ReservationRecord{}, fmt.Errorf("reservation response is not a JSON object: %w", err)
	}
	return reservation, nil
}

func (c *Client) SendFeishuCard(ctx context.Context, card map[string]any) error {
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	body, err := c.doJSON(ctx, http.MethodPost, c.settings.FeishuWebhook, map[string]string{"Content-Type": "application/json"}, payload)
	if err != nil {
		return err
	}

	var response map[string]any
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("invalid Feishu response: %w", err)
	}
	if statusCode, ok := response["StatusCode"]; ok {
		switch value := statusCode.(type) {
		case float64:
			if value != 0 {
				return fmt.Errorf("Feishu bot error: %s", stringifyJSON(response))
			}
		case string:
			if value != "" && value != "0" {
				return fmt.Errorf("Feishu bot error: %s", stringifyJSON(response))
			}
		}
	}
	return nil
}

func (c *Client) baseHeaders(authorization, contentType string) map[string]string {
	headers := map[string]string{
		"Authorization": ensureBearer(authorization),
		"X-App-Code":    c.settings.XAppCode,
		"X-App-Client":  c.settings.XAppClient,
		"User-Agent":    c.settings.UserAgent,
		"Referer":       c.settings.Referer,
	}
	if strings.TrimSpace(contentType) != "" {
		headers["Content-Type"] = contentType
	}
	return headers
}

func (c *Client) doJSON(ctx context.Context, method, target string, headers map[string]string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, normalizeErrorBody(responseBody))
	}
	if len(responseBody) == 0 {
		return nil, nil
	}
	if !json.Valid(responseBody) {
		return nil, fmt.Errorf("response is not JSON: %s", string(responseBody))
	}
	return responseBody, nil
}

func normalizeErrorBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		return stringifyJSON(payload)
	}
	return string(body)
}

func stringifyJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

type Monitor struct {
	settings                 Settings
	client                   *Client
	dryRun                   bool
	lastWeekdayEveningMinute string
}

func NewMonitor(settings Settings, dryRun bool) *Monitor {
	return &Monitor{
		settings: settings,
		client:   NewClient(settings),
		dryRun:   dryRun,
	}
}

func (m *Monitor) RunOnce(ctx context.Context) (bool, error) {
	now := time.Now().In(m.settings.Location)
	state, err := loadState(m.settings.StateFile)
	if err != nil {
		return false, err
	}
	if pruneExpiredReservation(&state, now) {
		if state.IsZero() {
			if err := clearState(m.settings.StateFile); err != nil {
				return false, err
			}
		} else if err := saveState(m.settings.StateFile, state); err != nil {
			return false, err
		}
	}

	existing := activeReservation(state, now)
	summaryDue := shouldSendWeekendSummary(state, now)
	consoleDue := m.lastWeekdayEveningMinute != currentMinuteKey(now)

	var (
		timeslotsByStore map[string][]Slot
		storeInfos       map[string]StoreInfo
	)
	needTimeslots := summaryDue || consoleDue || existing == nil
	needStoreInfos := summaryDue || consoleDue
	if needTimeslots || needStoreInfos {
		timeslotsByStore, storeInfos, err = m.fetchRoundData(ctx, needTimeslots, needStoreInfos)
		if err != nil {
			return false, err
		}
	}

	if summaryDue {
		summaries := make([]StoreSummary, 0, len(m.settings.StoreIDs))
		for _, storeID := range m.settings.StoreIDs {
			summaries = append(summaries, StoreSummary{
				Store: storeInfos[storeID],
				Slots: filterMonthlyWeekendSlots(timeslotsByStore[storeID], now, m.settings.AvailableStatuses, 30),
			})
		}
		if err := m.sendWeekendSummary(ctx, summaries, &state, now); err != nil {
			return false, err
		}
	}

	if consoleDue {
		summaries := make([]StoreSummary, 0, len(m.settings.StoreIDs))
		for _, storeID := range m.settings.StoreIDs {
			summaries = append(summaries, StoreSummary{
				Store: storeInfos[storeID],
				Slots: filterCurrentWeekWeekdayEveningSlots(timeslotsByStore[storeID], now, m.settings.AvailableStatuses),
			})
		}
		logMessage(now, buildWeekdayEveningConsoleReport(summaries, now))
		m.lastWeekdayEveningMinute = currentMinuteKey(now)
	}

	if existing != nil {
		logMessage(now, "An active reservation already exists, skipping new reservation attempts.")
		if !state.NotificationSent {
			if err := m.sendReservationNotification(ctx, *existing, &state, now); err != nil {
				return true, err
			}
		}
		return true, nil
	}

	if timeslotsByStore == nil {
		timeslotsByStore, err = m.fetchTimeslotsByStore(ctx)
		if err != nil {
			return false, err
		}
	}

	candidates := collectCandidateSlotsByStore(timeslotsByStore, m.settings.StoreIDs, now, m.settings.AvailableStatuses, m.settings.Debug)
	if len(candidates) == 0 {
		if m.settings.Debug {
			logMessage(now, "No bookable slots found from now onward across configured stores.")
		} else {
			logMessage(now, "No bookable slots found for this week's weekend across configured stores.")
		}
		return false, nil
	}

	for _, candidate := range candidates {
		storeInfo, ok := storeInfos[candidate.StoreID]
		if !ok {
			storeInfo, err = m.client.GetStoreInfo(ctx, candidate.StoreID)
			if err != nil {
				return false, err
			}
		}

		slotLabel := formatSlotWindow(candidate.Slot.Date, candidate.Slot.Start, defaultString(candidate.Slot.End, candidate.Slot.Start), m.settings.Location)
		if m.dryRun {
			logMessage(now, fmt.Sprintf("Dry run found candidate slot: %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
			return false, nil
		}

		logMessage(now, fmt.Sprintf("Attempting reservation for %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
		reservation, err := m.client.CreateReservation(ctx, candidate.StoreID, candidate.Slot.Date, candidate.Slot.Start)
		if err != nil {
			if errors.Is(err, errNoReservationAvailable) {
				logMessage(now, fmt.Sprintf("Slot disappeared before reservation completed: %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
				continue
			}
			return false, err
		}

		reservation.MonitoredStoreID = candidate.StoreID
		reservation.StoreName = storeInfo.Name
		reservation.StoreAddress = storeInfo.Address
		reservation.SlotLabel = formatSlotWindow(defaultString(reservation.QueueDate, candidate.Slot.Date), defaultString(reservation.Start, candidate.Slot.Start), defaultString(reservation.End, defaultString(candidate.Slot.End, candidate.Slot.Start)), m.settings.Location)

		state.ActiveReservation = &reservation
		state.NotificationSent = false
		state.SavedAt = now.Format(time.RFC3339)
		state.NotifiedAt = ""
		if err := saveState(m.settings.StateFile, state); err != nil {
			return false, err
		}
		if err := m.sendReservationNotification(ctx, reservation, &state, now); err != nil {
			return true, err
		}
		logMessage(now, fmt.Sprintf("Reservation succeeded at %s with number %s", fallbackString(storeInfo.Name, candidate.StoreID), reservation.Number))
		return true, nil
	}

	logMessage(now, "Candidates existed across configured stores, but no reservation could be created.")
	return false, nil
}

func (m *Monitor) fetchRoundData(ctx context.Context, needTimeslots, needStoreInfos bool) (map[string][]Slot, map[string]StoreInfo, error) {
	var (
		wg               sync.WaitGroup
		timeslotsByStore map[string][]Slot
		storeInfos       map[string]StoreInfo
		timeslotsErr     error
		storeInfosErr    error
	)

	if needTimeslots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timeslotsByStore, timeslotsErr = m.fetchTimeslotsByStore(ctx)
		}()
	}
	if needStoreInfos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			storeInfos, storeInfosErr = m.fetchStoreInfos(ctx)
		}()
	}
	wg.Wait()

	if timeslotsErr != nil {
		return nil, nil, timeslotsErr
	}
	if storeInfosErr != nil {
		return nil, nil, storeInfosErr
	}
	return timeslotsByStore, storeInfos, nil
}

func (m *Monitor) fetchTimeslotsByStore(ctx context.Context) (map[string][]Slot, error) {
	type result struct {
		storeID string
		slots   []Slot
		err     error
	}
	results := make(chan result, len(m.settings.StoreIDs))
	for _, storeID := range m.settings.StoreIDs {
		storeID := storeID
		go func() {
			slots, err := m.client.GetTimeslots(ctx, storeID)
			results <- result{storeID: storeID, slots: slots, err: err}
		}()
	}

	timeslotsByStore := make(map[string][]Slot, len(m.settings.StoreIDs))
	for range m.settings.StoreIDs {
		result := <-results
		if result.err != nil {
			return nil, fmt.Errorf("fetch timeslots for store %s: %w", result.storeID, result.err)
		}
		timeslotsByStore[result.storeID] = result.slots
	}
	return timeslotsByStore, nil
}

func (m *Monitor) fetchStoreInfos(ctx context.Context) (map[string]StoreInfo, error) {
	type result struct {
		storeID string
		store   StoreInfo
		err     error
	}
	results := make(chan result, len(m.settings.StoreIDs))
	for _, storeID := range m.settings.StoreIDs {
		storeID := storeID
		go func() {
			store, err := m.client.GetStoreInfo(ctx, storeID)
			results <- result{storeID: storeID, store: store, err: err}
		}()
	}

	storeInfos := make(map[string]StoreInfo, len(m.settings.StoreIDs))
	for range m.settings.StoreIDs {
		result := <-results
		if result.err != nil {
			return nil, fmt.Errorf("fetch store info for store %s: %w", result.storeID, result.err)
		}
		storeInfos[result.storeID] = result.store
	}
	return storeInfos, nil
}

func (m *Monitor) sendReservationNotification(ctx context.Context, reservation ReservationRecord, state *State, now time.Time) error {
	// Always print to terminal
	storeName := fallbackString(reservation.StoreName, "未知门店")
	slotLabel := fallbackString(reservation.SlotLabel, "未提供")
	reservationNumber := fallbackString(reservation.Number, "未提供")
	logMessage(now, fmt.Sprintf("=== 预约成功 ==="))
	logMessage(now, fmt.Sprintf("  门店：%s", storeName))
	logMessage(now, fmt.Sprintf("  时段：%s", slotLabel))
	logMessage(now, fmt.Sprintf("  号码：%s", reservationNumber))
	if reservation.StoreAddress != "" {
		logMessage(now, fmt.Sprintf("  地址：%s", reservation.StoreAddress))
	}

	// macOS system notification
	title := fmt.Sprintf("寿司郎预约成功 - %s", storeName)
	message := fmt.Sprintf("号码: %s | 时段: %s", reservationNumber, slotLabel)
	_ = exec.Command("osascript", "-e",
		fmt.Sprintf(`display notification "%s" with title "%s"`, message, title),
	).Run()

	// Feishu notification (optional)
	if m.settings.FeishuWebhook != "" {
		card := buildReservationSuccessCard(reservation, now)
		if err := m.client.SendFeishuCard(ctx, card); err != nil {
			logMessage(now, fmt.Sprintf("Feishu notification failed: %v", err))
		}
	}

	state.NotificationSent = true
	state.NotifiedAt = now.Format(time.RFC3339)
	return saveState(m.settings.StateFile, *state)
}

func (m *Monitor) sendWeekendSummary(ctx context.Context, summaries []StoreSummary, state *State, now time.Time) error {
	// Always print summary to terminal
	logMessage(now, "=== 未来30天周末可预约时段 ===")
	for _, summary := range summaries {
		storeName := fallbackString(summary.Store.Name, "未知门店")
		grouped := groupSlotsByDate(summary.Slots, now.Location())
		if len(grouped) == 0 {
			logMessage(now, fmt.Sprintf("  %s：暂无", storeName))
			continue
		}
		dates := make([]string, 0, len(grouped))
		for key := range grouped {
			dates = append(dates, key)
		}
		sort.Strings(dates)
		for _, dateKey := range dates {
			logMessage(now, fmt.Sprintf("  %s %s：%s", storeName, dateKey, strings.Join(grouped[dateKey], "、")))
		}
	}

	// Feishu notification (optional)
	if m.settings.FeishuWebhook != "" {
		card := buildMultiStoreWeekendSummaryCard(summaries, now)
		if err := m.client.SendFeishuCard(ctx, card); err != nil {
			logMessage(now, fmt.Sprintf("Feishu notification failed: %v", err))
		}
	}

	state.LastWeekendSummaryHour = currentSummaryHourKey(now)
	state.LastWeekendSummaryAt = now.Format(time.RFC3339)
	return saveState(m.settings.StateFile, *state)
}

func runLoop(ctx context.Context, monitor *Monitor, interval time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := monitor.RunOnce(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func main() {
	configPathFlag := flag.String("config", "config.json", "Path to the JSON config file.")
	onceFlag := flag.Bool("once", false, "Run one check and exit.")
	loopFlag := flag.Bool("loop", false, "Poll repeatedly using poll_interval_seconds.")
	dryRunFlag := flag.Bool("dry-run", false, "Detect candidate slots without creating a reservation.")
	flag.Parse()

	configPath, err := filepath.Abs(*configPathFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	settings, err := LoadSettings(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	monitor := NewMonitor(settings, *dryRunFlag)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *loopFlag {
		err = runLoop(ctx, monitor, settings.PollInterval)
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	_ = *onceFlag
	if _, err := monitor.RunOnce(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
