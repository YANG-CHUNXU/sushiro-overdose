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
	Enabled    bool   `json:"enabled"`
	StoreID    string `json:"store_id"`
	StoreName  string `json:"store_name,omitempty"`
	TargetTime string `json:"target_time"` // "HHMM"
	Status     string `json:"status"`      // idle/armed/success/error/expired
	Number     string `json:"number,omitempty"`
	TicketID   int64  `json:"ticket_id,omitempty"`
	FiredDate  string `json:"fired_date,omitempty"` // 当天已执行(成功或放弃)的日期 YYYY-MM-DD
	FiredAt    string `json:"fired_at,omitempty"`
	LastError  string `json:"last_error,omitempty"`
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
	return p
}

func SaveNetTicketPlan(p NetTicketPlan) error {
	os.MkdirAll(AppDirPath(), 0o755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(netTicketPlanPath(), data, 0o600)
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
	if !plan.Enabled || strings.TrimSpace(plan.StoreID) == "" || len(strings.TrimSpace(plan.TargetTime)) < 3 {
		return
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	if plan.FiredDate == today {
		return // 今天已处理过
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
		plan.Status = "expired"
		plan.FiredDate = today
		plan.FiredAt = now.Format(time.RFC3339)
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⏰ 定时取号未执行", DefaultString(plan.StoreName, plan.StoreID)+"：超过 "+netTicketDisplayTime(plan.TargetTime)+" 窗口仍未取到号")
		return
	}
	// 命中窗口：按日期占位，确保只取一次（web/守护两个进程都安全）。
	if !reserveNetTicketFire(today) {
		return
	}
	plan.FiredDate = today
	plan.FiredAt = now.Format(time.RFC3339)
	_ = SaveNetTicketPlan(plan)

	client := currentAuthedClient()
	if client == nil {
		plan.Status = "error"
		plan.LastError = "尚未捕获认证参数（或已过期），无法自动取号"
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⚠️ 定时取号失败", plan.LastError)
		return
	}
	ticket, err := client.CreateNetTicket(ctx, plan.StoreID)
	if err != nil {
		if isOfficialServerHTTPError(err) {
			plan.Status = "retrying"
			plan.FiredDate = ""
			plan.FiredAt = ""
			plan.LastError = friendlyOfficialAPIError(err)
			_ = SaveNetTicketPlan(plan)
			clearNetTicketFire(today)
			return
		}
		plan.Status = "error"
		plan.LastError = err.Error()
		_ = SaveNetTicketPlan(plan)
		sendQueueAlert(ctx, "⚠️ 定时取号失败", DefaultString(plan.StoreName, plan.StoreID)+"："+err.Error())
		return
	}
	plan.Status = "success"
	plan.Number = ticket.Number
	plan.TicketID = ticket.TicketID
	plan.LastError = ""
	_ = SaveNetTicketPlan(plan)
	sendQueueAlert(ctx, "🎫 已自动取号", DefaultString(plan.StoreName, plan.StoreID)+"：号码 "+DefaultString(ticket.Number, "(详见我的预约)"))
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
