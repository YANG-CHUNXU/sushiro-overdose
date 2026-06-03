package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ChineseWeekdayNames = [...]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

type Slot struct {
	StoreID      string `json:"storeId"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Availability string `json:"availability"`
}

type QueueGroupQueues struct {
	ReservationQueue []string `json:"reservationQueue,omitempty"`
	CounterQueue     []string `json:"counterQueue,omitempty"`
	BoothQueue       []string `json:"boothQueue,omitempty"`
	MixedQueue       []string `json:"mixedQueue,omitempty"`
}

func (q *QueueGroupQueues) UnmarshalJSON(data []byte) error {
	*q = QueueGroupQueues{}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	q.ReservationQueue = parseQueueGroupQueue(raw["reservationQueue"])
	q.CounterQueue = parseQueueGroupQueue(raw["counterQueue"])
	q.BoothQueue = parseQueueGroupQueue(raw["boothQueue"])
	q.MixedQueue = parseQueueGroupQueue(raw["mixedQueue"])
	return nil
}

func parseQueueGroupQueue(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		values := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := parseQueueGroupQueueValue(item); ok {
				values = append(values, value)
			}
		}
		return values
	}

	if value, ok := parseQueueGroupQueueValue(raw); ok {
		return []string{value}
	}
	return nil
}

func parseQueueGroupQueueValue(raw json.RawMessage) (string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}

	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case json.Number:
		trimmed := strings.TrimSpace(typed.String())
		return trimmed, trimmed != ""
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

type StoreInfo struct {
	ID                int              `json:"id"`
	Name              string           `json:"name"`
	Address           string           `json:"address"`
	StoreStatus       string           `json:"storeStatus,omitempty"`
	Wait              int              `json:"wait,omitempty"`
	WaitTimeCounter   int              `json:"waitTimeCounter,omitempty"`
	NetTicketStatus   string           `json:"netTicketStatus,omitempty"`
	RemoteTicketing   string           `json:"remoteTicketingManualStatus,omitempty"`
	ReservationStatus string           `json:"reservationStatus,omitempty"`
	CheckinStatus     string           `json:"checkinStatus,omitempty"`
	GroupQueuesCount  int              `json:"groupQueuesCount,omitempty"`
	GroupQueues       QueueGroupQueues `json:"groupQueues,omitempty"`
}

type ReservationRecord struct {
	Kind             string `json:"kind,omitempty"`
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

func ParseCompactDate(raw string, loc *time.Location) (time.Time, error) {
	if len(raw) != 8 {
		return time.Time{}, fmt.Errorf("invalid date: %s", raw)
	}
	return time.ParseInLocation("20060102", raw, loc)
}

func ParseCompactTime(raw string) (int, int, int, error) {
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

func FormatCompactTime(raw string) string {
	hour, minute, _, err := ParseCompactTime(raw)
	if err != nil {
		return raw
	}
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func FormatSlotWindow(slotDate, start, end string, loc *time.Location) string {
	day, err := ParseCompactDate(slotDate, loc)
	if err != nil {
		return slotDate + " " + FormatCompactTime(start) + "-" + FormatCompactTime(end)
	}
	return fmt.Sprintf("%s %s-%s", day.Format("2006-01-02"), FormatCompactTime(start), FormatCompactTime(end))
}

func FormatDateWithWeekday(day time.Time) string {
	return fmt.Sprintf("%s（%s）", day.Format("2006-01-02"), ChineseWeekdayNames[WeekdayIndexMon0(day.Weekday())])
}

func WeekdayIndexMon0(day time.Weekday) int {
	if day == time.Sunday {
		return 6
	}
	return int(day) - 1
}

func BeginningOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func SlotDateTime(slot Slot, loc *time.Location) (time.Time, error) {
	day, err := ParseCompactDate(slot.Date, loc)
	if err != nil {
		return time.Time{}, err
	}
	hour, minute, second, err := ParseCompactTime(slot.Start)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, 0, loc), nil
}
