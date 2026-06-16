package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ChineseWeekdayNames 是按「周一=0」编号的中文星期名，配合 WeekdayIndexMon0 使用。
var ChineseWeekdayNames = [...]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

// Slot 是 timeslots 接口返回的单个时段。日期/时间字段都是紧凑数字串：
//   - Date：8 位 YYYYMMDD
//   - Start / End：6 位 HHMMSS
//   - Availability：可约状态文本（如 "AVAILABLE"），与 Settings.AvailableStatuses 比对判断是否可约。
type Slot struct {
	StoreID      string `json:"storeId"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Availability string `json:"availability"`
}

// QueueGroupQueues 描述门店各类桌位的排队号列表。
// 注意：自定义了 UnmarshalJSON，因为官方这个字段有时是数组、有时是单值，需要兼容两种形态。
type QueueGroupQueues struct {
	ReservationQueue []string `json:"reservationQueue,omitempty"`
	CounterQueue     []string `json:"counterQueue,omitempty"`
	BoothQueue       []string `json:"boothQueue,omitempty"`
	MixedQueue       []string `json:"mixedQueue,omitempty"`
}

// UnmarshalJSON 兼容官方 groupQueues 字段的不稳定结构：
// 数组形态（["A1","A2"]）和单值形态（"A1"）都要能解析成 []string。
// 解析失败时静默返回零值（不返回 error）——这是辅助展示字段，不应让整条记录解析失败。
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

// parseQueueGroupQueue 把一个队列字段（可能是数组或单值）解析成 []string。
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

	// 不是数组就当单值试一次（官方有时直接给标量）。
	if value, ok := parseQueueGroupQueueValue(raw); ok {
		return []string{value}
	}
	return nil
}

// parseQueueGroupQueueValue 把单个队列项转成字符串。用 UseNumber 是为了避免大号码被转成 float64 丢精度。
// string/json.Number/bool 都接受；空白字符串视为无效（丢弃）。
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

// StoreInfo 是 getStoreById 返回的门店详情。各状态字段语义（取值来自官方，均可空）：
//   - StoreStatus：门店营业状态（如 OPEN/CLOSE）。
//   - Wait / WaitTimeCounter：当前等位数 / 预计等待桌数。
//   - NetTicketStatus：远程取号（日常排队）是否可用及状态。
//   - RemoteTicketing（remoteTicketingManualStatus）：人工配置的远程取号开关，优先级高于自动判定。
//   - ReservationStatus：预约是否开放。
//   - CheckinStatus：到店签到状态。
//   - GroupQueues / GroupQueuesCount：各类桌位当前排队号及总数。
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

// ReservationRecord 是预约/取号结果与状态的统一结构，同时承载「下单成功返回」和「查状态返回」两种用途。
// 字段语义（注意：来源不同，部分字段只在某一种返回里出现）：
//   - Kind：记录类型标记（reservation / net_ticket），由 parseReservationRecord 按调用接口补上，
//     用于区分这条记录是「预约」还是「日常排队取号」。
//   - Number：取号后的排队号；TicketID：官方票据 ID。两者任一非空即视为成功（见 reservationLooksSuccessful）。
//   - MonitoredStoreID / StoreName / StoreAddress / SlotLabel：不是官方字段，是本地补充的展示/监控字段
//     （json tag 带下划线，官方接口不会回传，由本地代码填充）。
//   - Start/End/QueueTime/QueueDate：时段/排队时间，注意时区按 Settings.Location 解释。
//   - CheckedIn：是否已签到。
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

// ParseCompactDate 把 8 位紧凑日期 YYYYMMDD 解析成指定时区的 time.Time。
// 用 ParseInLocation 而不是 Parse，确保跨时区场景下「那一天」的归属正确（避免 UTC 偏移导致日期错位）。
func ParseCompactDate(raw string, loc *time.Location) (time.Time, error) {
	if len(raw) != 8 {
		return time.Time{}, fmt.Errorf("invalid date: %s", raw)
	}
	return time.ParseInLocation("20060102", raw, loc)
}

// ParseCompactTime 把 6 位紧凑时间 HHMMSS 拆成时/分/秒。不做范围校验（如 hour>23），
// 调用方拿到的是原始数字；后续 SlotDateTime 用 time.Date 构造时会自然做范围归一。
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

// FormatCompactTime 把紧凑时间 HHMMSS 渲染成 "HH:MM"（丢秒）。解析失败原样返回。
func FormatCompactTime(raw string) string {
	hour, minute, _, err := ParseCompactTime(raw)
	if err != nil {
		return raw
	}
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

// FormatSlotWindow 把「日期+起止时间」渲染成 "YYYY-MM-DD HH:MM-HH:MM"。
// 日期解析失败时降级用原始字符串拼接，保证总能展示点东西。
func FormatSlotWindow(slotDate, start, end string, loc *time.Location) string {
	day, err := ParseCompactDate(slotDate, loc)
	if err != nil {
		return slotDate + " " + FormatCompactTime(start) + "-" + FormatCompactTime(end)
	}
	return fmt.Sprintf("%s %s-%s", day.Format("2006-01-02"), FormatCompactTime(start), FormatCompactTime(end))
}

// FormatDateWithWeekday 渲染成 "YYYY-MM-DD（周X）"，周几用中文。
func FormatDateWithWeekday(day time.Time) string {
	return fmt.Sprintf("%s（%s）", day.Format("2006-01-02"), ChineseWeekdayNames[WeekdayIndexMon0(day.Weekday())])
}

// WeekdayIndexMon0 把 time.Weekday（周日=0）转成「周一=0」的索引，匹配 ChineseWeekdayNames。
func WeekdayIndexMon0(day time.Weekday) int {
	if day == time.Sunday {
		return 6
	}
	return int(day) - 1
}

// BeginningOfDay 返回 now 所在时区的当天 0 点。跨天判定（如「这是今天的号还是明天的」）都基于它。
func BeginningOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// SlotDateTime 把 Slot 的紧凑日期+开始时间组合成完整 time.Time，用于排序、过期判断等。
// 时区按传入的 loc 解释（一般用 Settings.Location）。
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
