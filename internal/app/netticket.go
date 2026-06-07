package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	netTicketPlanFile      = "netticket_plan.json"
	netTicketWindowMinutes = 12 // 到点后多久内仍尝试取号；过窗口则放弃
	netTicketTickSeconds   = 20
)

// NetTicketPlan 是「定时取号」计划：到指定时间自动远程取号。
type NetTicketPlan struct {
	Enabled     bool   `json:"enabled"`
	StoreID     string `json:"store_id"`
	StoreName   string `json:"store_name,omitempty"`
	TriggerMode string `json:"trigger_mode,omitempty"` // "time"(默认到点) / "on_open"(一开放就取号)
	TargetTime  string `json:"target_time"`            // "HHMM"，仅 time 模式使用
	Status      string `json:"status"`                 // idle/armed/success/error/expired
	Number      string `json:"number,omitempty"`
	TicketID    int64  `json:"ticket_id,omitempty"`
	FiredDate   string `json:"fired_date,omitempty"` // 当天已执行(成功或放弃)的日期 YYYY-MM-DD
	FiredAt     string `json:"fired_at,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

func netTicketPlanPath() string { return filepath.Join(AppDirPath(), netTicketPlanFile) }

func LoadNetTicketPlan() NetTicketPlan {
	data, err := os.ReadFile(netTicketPlanPath())
	if err != nil {
		return NetTicketPlan{Status: "idle"}
	}
	var p NetTicketPlan
	if json.Unmarshal(data, &p) != nil {
		return NetTicketPlan{Status: "idle"}
	}
	if p.Status == "error" && isTicketAlreadyIssuedText(p.LastError) {
		p.Status = "issued_unknown"
	}
	return normalizeNetTicketPlan(p, time.Now())
}

func SaveNetTicketPlan(p NetTicketPlan) error {
	os.MkdirAll(AppDirPath(), 0o755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(netTicketPlanPath(), data, 0o600)
}

func normalizeNetTicketPlan(p NetTicketPlan, now time.Time) NetTicketPlan {
	if now.IsZero() {
		now = time.Now()
	}
	if p.Status == "" {
		p.Status = "idle"
	}
	if p.Enabled && netTicketPlanTerminal(p.Status) {
		p.Enabled = false
	}
	return p
}

func netTicketPlanTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case "success", "issued_unknown", "expired", "error":
		return true
	default:
		return false
	}
}

func netTicketPlanFiredOn(p NetTicketPlan, day time.Time) bool {
	if day.IsZero() {
		day = time.Now()
	}
	today := day.Format("2006-01-02")
	if strings.TrimSpace(p.FiredDate) == today {
		return true
	}
	if strings.TrimSpace(p.FiredAt) == "" {
		return false
	}
	firedAt, ok := parseRFC3339Local(p.FiredAt)
	return ok && firedAt.Format("2006-01-02") == today
}

var netTicketMu sync.Mutex

// NetTicketScheduler 在后台轮询，到点自动取号。可同时跑在 web 进程和后台守护里，
// 用按日期的独占锁文件保证一天只取一次、不会重复。
type NetTicketScheduler struct {
	mu      sync.Mutex
	running bool
}

var netTicketSched = &NetTicketScheduler{}

func (s *NetTicketScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		t := time.NewTicker(netTicketTickSeconds * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				netTicketTick(ctx)
			}
		}
	}()
}

func netTicketTick(ctx context.Context) {
	netTicketMu.Lock()
	defer netTicketMu.Unlock()

	plan := LoadNetTicketPlan()
	if !plan.Enabled || strings.TrimSpace(plan.StoreID) == "" {
		return
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	if plan.FiredDate == today {
		return // 今天已处理过
	}
	if plan.TriggerMode == "on_open" {
		netTicketTickOnOpen(ctx, plan, now, today)
		return
	}
	netTicketTickByTime(ctx, plan, now, today)
}

// netTicketTickByTime 是「到点取号」：到设定时间的窗口内自动取号。
func netTicketTickByTime(ctx context.Context, plan NetTicketPlan, now time.Time, today string) {
	if len(strings.TrimSpace(plan.TargetTime)) < 3 {
		return
	}
	target, ok := netTicketTargetToday(plan.TargetTime, now)
	if !ok {
		return
	}
	if now.Before(target) {
		if plan.Status != "armed" {
			plan.Status = "armed"
			plan.LastError = ""
			_ = SaveNetTicketPlan(plan)
		}
		return
	}
	if now.After(target.Add(netTicketWindowMinutes * time.Minute)) {
		plan.Enabled = false
		plan.Status = "expired"
		plan.FiredDate = today
		plan.FiredAt = now.Format(time.RFC3339)
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⏰ 定时取号未执行", DefaultString(plan.StoreName, plan.StoreID)+"：超过 "+netTicketDisplayTime(plan.TargetTime)+" 窗口仍未取到号")
		return
	}
	fireNetTicket(ctx, plan, now, today)
}

// netTicketTickOnOpen 是「一开放就取号」：轮询门店线上取号状态，翻为开放即取号。
func netTicketTickOnOpen(ctx context.Context, plan NetTicketPlan, now time.Time, today string) {
	stores, err := NewQueueLiveClient().CachedAllStores(ctx)
	if err != nil {
		return // 拿不到门店状态，下个 tick 再试
	}
	open := false
	for _, s := range stores {
		if strconv.Itoa(s.ID) == strings.TrimSpace(plan.StoreID) {
			open = queueLiveStoreOnlineOpen(s)
			break
		}
	}
	if !open {
		if plan.Status != "armed" {
			plan.Status = "armed"
			plan.LastError = ""
			_ = SaveNetTicketPlan(plan)
		}
		return
	}
	fireNetTicket(ctx, plan, now, today)
}

// fireNetTicket 是 time/on_open 两种触发共用的取号执行路径。
func fireNetTicket(ctx context.Context, plan NetTicketPlan, now time.Time, today string) {
	// 命中条件：按日期占位，确保只取一次（web/守护两个进程都安全）。
	if !reserveNetTicketFire(today) {
		return
	}
	plan.FiredDate = today
	plan.FiredAt = now.Format(time.RFC3339)
	_ = SaveNetTicketPlan(plan)

	client := currentAuthedClient()
	if client == nil {
		plan.Enabled = false
		plan.Status = "error"
		plan.LastError = "尚未捕获认证参数（或已过期），无法自动取号"
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⚠️ 定时取号失败", plan.LastError)
		return
	}
	ticket, err := client.CreateNetTicket(ctx, plan.StoreID)
	if err != nil {
		if isTicketAlreadyIssuedError(err) {
			if recovered, ok := recoverExistingNetTicket(ctx, client, &plan); ok {
				plan = recovered
				_ = SaveNetTicketPlan(plan)
				sendQueueAlert(ctx, "🎫 已恢复排队号", DefaultString(plan.StoreName, plan.StoreID)+"：号码 "+DefaultString(plan.Number, "(详见我的预约)"))
				return
			}
			markNetTicketIssuedUnknown(&plan, friendlyNetTicketError(err))
			sendQueueAlert(ctx, "⚠️ 已有排队号", DefaultString(plan.StoreName, plan.StoreID)+"："+plan.LastError)
			return
		}
		if isOfficialServerHTTPError(err) {
			plan.Status = "retrying"
			plan.FiredDate = ""
			plan.FiredAt = ""
			plan.LastError = friendlyNetTicketError(err)
			_ = SaveNetTicketPlan(plan)
			clearNetTicketFire(today)
			return
		}
		plan.Enabled = false
		plan.Status = "error"
		plan.LastError = friendlyNetTicketError(err)
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⚠️ 定时取号失败", DefaultString(plan.StoreName, plan.StoreID)+"："+plan.LastError)
		return
	}
	applyNetTicketSuccess(ctx, client, &plan, ticket)
	_ = SaveNetTicketPlan(plan)
	sendQueueAlert(ctx, "🎫 已自动取号", DefaultString(plan.StoreName, plan.StoreID)+"：号码 "+DefaultString(ticket.Number, "(详见我的预约)"))
}

func markNetTicketIssuedUnknown(plan *NetTicketPlan, message string) {
	plan.Enabled = false
	plan.Status = "issued_unknown"
	plan.LastError = message
	_ = SaveNetTicketPlan(*plan)
}

func recoverExistingNetTicket(ctx context.Context, client *Client, plan *NetTicketPlan) (NetTicketPlan, bool) {
	ticket, err := client.GetNetTicketStatus(ctx)
	if err != nil || !netTicketLooksSuccessful(ticket) {
		markNetTicketIssuedUnknown(plan, friendlyNetTicketError(err))
		return *plan, false
	}
	applyNetTicketSuccess(ctx, client, plan, ticket)
	return *plan, true
}

func applyNetTicketSuccess(ctx context.Context, client *Client, plan *NetTicketPlan, ticket ReservationRecord) {
	plan.Enabled = false
	plan.Status = "success"
	plan.Number = ticket.Number
	plan.TicketID = ticket.TicketID
	plan.LastError = ""
	storeName := DefaultString(plan.StoreName, plan.StoreID)
	storeAddress := ""
	if info, err := client.GetStoreInfo(ctx, plan.StoreID); err == nil {
		storeName = DefaultString(info.Name, storeName)
		storeAddress = info.Address
	}
	ticket.MonitoredStoreID = plan.StoreID
	onBookingSuccess(ticket, storeName, storeAddress, "排队取号", "取号")
}

func netTicketLooksSuccessful(ticket ReservationRecord) bool {
	return strings.TrimSpace(ticket.Number) != "" || ticket.TicketID != 0
}

// reserveNetTicketFire 用独占创建的锁文件占位，返回 true 表示本进程抢到了今天的取号执行权。
func reserveNetTicketFire(date string) bool {
	p := filepath.Join(AppDirPath(), "netticket_fire_"+date+".lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
	_ = f.Close()
	return true
}

// clearNetTicketFire 在用户重新设定计划时清掉当天占位，允许再次取号。
func clearNetTicketFire(date string) {
	_ = os.Remove(filepath.Join(AppDirPath(), "netticket_fire_"+date+".lock"))
}

func netTicketTargetToday(hhmm string, now time.Time) (time.Time, bool) {
	hhmm = strings.TrimSpace(hhmm)
	for len(hhmm) < 4 {
		hhmm = "0" + hhmm
	}
	if len(hhmm) < 4 {
		return time.Time{}, false
	}
	h, err1 := strconv.Atoi(hhmm[:2])
	m, err2 := strconv.Atoi(hhmm[2:4])
	if err1 != nil || err2 != nil || h > 23 || m > 59 {
		return time.Time{}, false
	}
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location()), true
}

func netTicketDisplayTime(hhmm string) string {
	for len(hhmm) < 4 {
		hhmm = "0" + hhmm
	}
	return hhmm[:2] + ":" + hhmm[2:4]
}

// currentAuthedClient 从本地配置构造一个带认证的 API 客户端（headless 守护也能用）。
func currentAuthedClient() *Client {
	tokens, err := LoadLocalConfig()
	if err != nil {
		return nil
	}
	if tokens.ValidateForReservation() != nil {
		return nil
	}
	prefs := LoadPreferences()
	return NewClient(tokens.ToSettingsWithPrefs(prefs))
}
