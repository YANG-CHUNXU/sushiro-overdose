package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var chineseWeekdayNames = [...]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

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

type StoreCandidate struct {
	StoreID string
	Slot    Slot
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
