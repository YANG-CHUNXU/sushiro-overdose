package main

import (
	"fmt"
	"strconv"
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
