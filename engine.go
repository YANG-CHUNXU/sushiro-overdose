package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type EngineStatus string

const (
	EngineIdle      EngineStatus = "idle"
	EngineCapturing EngineStatus = "capturing"
	EngineBooking   EngineStatus = "booking"
	EngineSniping   EngineStatus = "sniping"
	EngineSuccess   EngineStatus = "success"
	EngineError     EngineStatus = "error"
)

type EngineState struct {
	Status      EngineStatus       `json:"status"`
	Message     string             `json:"message"`
	LastCheck   string             `json:"last_check,omitempty"`
	Attempts    int                `json:"attempts"`
	Capture     *CaptureStatusJSON `json:"capture,omitempty"`
	Reservation *ReservationRecord `json:"reservation,omitempty"`
}

type CaptureStatusJSON struct {
	XAppCode        bool `json:"x_app_code"`
	QueryAuth       bool `json:"query_auth"`
	ReservationAuth bool `json:"reservation_auth"`
	UserAgent       bool `json:"user_agent"`
	Referer         bool `json:"referer"`
	WechatID        bool `json:"wechat_id"`
	PhoneNumber     bool `json:"phone_number"`
	Complete        bool `json:"complete"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Level   string `json:"level,omitempty"`
}

// BookingEngine manages capture and booking lifecycle, controllable from Web UI.
type BookingEngine struct {
	mu     sync.RWMutex
	state  EngineState
	cancel context.CancelFunc
	tokens *CapturedTokens
	proxy  *proxyServer
	logs   []LogEntry
}

var engine = &BookingEngine{
	state: EngineState{Status: EngineIdle},
}

func (e *BookingEngine) GetState() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.state
	if e.tokens != nil && e.state.Status == EngineCapturing {
		s.Capture = e.captureStatus()
	}
	return s
}

func (e *BookingEngine) captureStatus() *CaptureStatusJSON {
	if e.tokens == nil {
		return nil
	}
	e.tokens.mu.Lock()
	defer e.tokens.mu.Unlock()
	return &CaptureStatusJSON{
		XAppCode:        e.tokens.XAppCode != "",
		QueryAuth:       e.tokens.QueryAuth != "",
		ReservationAuth: e.tokens.ReservationAuth != "",
		UserAgent:       e.tokens.UserAgent != "",
		Referer:         e.tokens.Referer != "",
		WechatID:        e.tokens.WechatID != "",
		PhoneNumber:     e.tokens.PhoneNumber != "",
		Complete:        e.tokens.IsCompleteUnlocked(),
	}
}

func (e *BookingEngine) setState(status EngineStatus, message string) {
	e.mu.Lock()
	e.state.Status = status
	e.state.Message = message
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
}

func (e *BookingEngine) addLog(msg string) {
	e.addLogLevel(msg, "info")
}

func (e *BookingEngine) addLogLevel(msg, level string) {
	e.mu.Lock()
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Message: msg,
		Level:   level,
	}
	e.logs = append(e.logs, entry)
	if len(e.logs) > 500 {
		e.logs = e.logs[len(e.logs)-500:]
	}
	e.mu.Unlock()
	bus.publish("log", mustJSON(entry))
	logMessage(time.Now(), msg)
}

func (e *BookingEngine) GetLogs() []LogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]LogEntry, len(e.logs))
	copy(out, e.logs)
	return out
}

// StartCapture begins the MITM proxy to capture WeChat tokens.
func (e *BookingEngine) StartCapture() error {
	e.mu.Lock()
	if e.state.Status == EngineCapturing || e.state.Status == EngineBooking {
		e.mu.Unlock()
		return fmt.Errorf("引擎正在运行中")
	}
	e.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()

	go e.runCapture(ctx)
	return nil
}

func (e *BookingEngine) runCapture(ctx context.Context) {
	e.setState(EngineCapturing, "正在准备证书...")

	caCert, caKey, err := loadOrGenerateCA()
	if err != nil {
		e.setState(EngineError, "CA证书加载失败: "+err.Error())
		e.addLogLevel("CA证书加载失败: "+err.Error(), "error")
		return
	}

	trusted, _ := IsCertTrusted()
	if !trusted {
		e.addLog("首次运行，正在安装CA证书...")
		if err := InstallCert(); err != nil {
			e.setState(EngineError, "证书安装失败，请查看设置页面的安装指南")
			e.addLogLevel("证书安装失败: "+err.Error(), "error")
			return
		}
		e.addLog("证书安装成功")
	}

	tokens := newCapturedTokens()
	e.mu.Lock()
	e.tokens = tokens
	e.mu.Unlock()

	proxy, err := startProxy(caCert, caKey, tokens)
	if err != nil {
		e.setState(EngineError, "启动代理失败: "+err.Error())
		e.addLogLevel("启动代理失败: "+err.Error(), "error")
		return
	}
	e.mu.Lock()
	e.proxy = proxy
	e.mu.Unlock()

	if err := SetSystemProxy(proxyPort); err != nil {
		proxy.close()
		e.setState(EngineError, "设置系统代理失败: "+err.Error())
		e.addLogLevel("设置系统代理失败: "+err.Error(), "error")
		return
	}
	markProxyActive(proxyPort, 0)

	e.setState(EngineCapturing, "等待捕获认证参数，请在 PC 微信中打开寿司郎小程序...")
	e.addLog("系统代理已设置 (127.0.0.1:8080)，请在 PC 微信中打开寿司郎小程序并操作一次排队/预约")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.cleanupProxy()
			e.setState(EngineIdle, "已停止捕获")
			return
		case <-ticker.C:
			bus.publish("engine", mustJSON(e.GetState()))
			if tokens.IsComplete() {
				e.cleanupProxy()
				if err := saveLocalConfig(tokens); err != nil {
					e.addLogLevel("保存配置失败: "+err.Error(), "error")
				} else {
					e.addLog("认证参数已捕获并保存！")
				}
				e.setState(EngineIdle, "认证参数捕获完成！")

				prefs := LoadPreferences()
				tokens.mu.Lock()
				if len(tokens.StoreIDs) > 0 && len(prefs.SelectedStores) == 0 {
					prefs.SelectedStores = tokens.StoreIDs
					SavePreferences(prefs)
				}
				tokens.mu.Unlock()
				return
			}
		}
	}
}

func (e *BookingEngine) cleanupProxy() {
	e.mu.Lock()
	if e.proxy != nil {
		e.proxy.close()
		e.proxy = nil
	}
	e.mu.Unlock()
	ClearSystemProxy()
	markProxyInactive()
}

// StartBooking begins the automated booking loop.
func (e *BookingEngine) StartBooking() error {
	e.mu.Lock()
	if e.state.Status == EngineCapturing || e.state.Status == EngineBooking {
		e.mu.Unlock()
		return fmt.Errorf("引擎正在运行中")
	}
	e.mu.Unlock()

	tokens, err := loadLocalConfig()
	if err != nil {
		return fmt.Errorf("暂无认证参数，请先完成参数捕获")
	}

	prefs := LoadPreferences()
	if len(prefs.SelectedStores) > 0 {
		tokens.mu.Lock()
		tokens.StoreIDs = prefs.SelectedStores
		tokens.mu.Unlock()
	}

	settings := tokens.toSettingsWithPrefs(prefs)
	client := NewClient(settings)

	// Quick verify
	if _, err := client.GetTimeslots(context.Background(), settings.StoreIDs[0]); err != nil {
		if isAuthError(err) {
			deleteLocalConfig()
			return fmt.Errorf("认证参数已过期，请重新捕获")
		}
		return fmt.Errorf("验证失败: %w", err)
	}

	setNotifier(BuildNotifierFromConfig())

	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.state.Attempts = 0
	e.mu.Unlock()

	go e.runBooking(ctx, client, settings, prefs)
	return nil
}

func (e *BookingEngine) runBooking(ctx context.Context, client *Client, settings Settings, prefs UserPreferences) {
	e.setState(EngineBooking, "正在抢号...")
	e.addLog(fmt.Sprintf("开始抢号 — 门店: %v, 成人: %d, 儿童: %d", settings.StoreIDs, settings.Adult, settings.Child))

	healthStop := startHealthCheck(ctx, client, settings.StoreIDs)
	defer close(healthStop)

	var booked map[string]bool
	errStreak := 0
	authErrors := 0

	for {
		select {
		case <-ctx.Done():
			e.setState(EngineIdle, "已停止抢号")
			e.addLog("抢号已停止")
			return
		default:
		}

		now := time.Now().In(settings.Location)

		var best *TargetSlot
		for _, storeID := range settings.StoreIDs {
			slots, err := client.GetTimeslots(ctx, storeID)
			if err != nil {
				if isAuthError(err) {
					authErrors++
					if authErrors >= 3 {
						e.addLogLevel("认证失败，请重新捕获参数", "error")
						sendNotification("寿司郎 - 认证失败", "认证参数已失效，请重新捕获")
						deleteLocalConfig()
						e.setState(EngineError, "认证参数已失效，请重新捕获")
						return
					}
				}
				errStreak++
				if errStreak >= 5 {
					e.addLog("连续失败过多，等待5秒...")
					time.Sleep(5 * time.Second)
					errStreak = 0
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			errStreak = 0
			authErrors = 0
			appendHistory(slots, storeID)

			for i := range slots {
				if !prefs.ShouldTarget(slots[i], settings.Location) {
					continue
				}
				if strings.ToUpper(slots[i].Availability) != "AVAILABLE" {
					continue
				}
				key := storeID + slots[i].Date + slots[i].Start
				if booked != nil && booked[key] {
					continue
				}
				t := &TargetSlot{
					StoreID: storeID,
					Date:    slots[i].Date,
					Start:   slots[i].Start,
					End:     slots[i].End,
				}
				if best == nil || t.Date+t.Start < best.Date+best.Start {
					best = t
				}
			}
		}

		if best == nil {
			e.mu.Lock()
			e.state.Attempts++
			e.state.LastCheck = now.Format("15:04:05")
			e.state.Message = fmt.Sprintf("查询中...第 %d 次，暂无目标时段", e.state.Attempts)
			e.mu.Unlock()
			bus.publish("engine", mustJSON(e.GetState()))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		slotLabel := formatSlotWindow(best.Date, best.Start, best.End, settings.Location)
		e.addLog(fmt.Sprintf("发现目标: %s — 尝试预约...", slotLabel))

		reservation, err := client.CreateReservation(ctx, best.StoreID, best.Date, best.Start)
		if err != nil {
			key := best.StoreID + best.Date + best.Start
			if booked == nil {
				booked = make(map[string]bool)
			}
			booked[key] = true

			if isAuthError(err) {
				authErrors++
				if authErrors >= 3 {
					e.addLogLevel("认证失败", "error")
					sendNotification("寿司郎 - 认证失败", "请重新捕获参数")
					deleteLocalConfig()
					e.setState(EngineError, "认证参数已失效")
					return
				}
			}

			if strings.Contains(err.Error(), "500") {
				e.addLogLevel("HTTP 500，参数已失效", "error")
				sendNotification("寿司郎 - HTTP 500", "参数已失效")
				deleteLocalConfig()
				e.setState(EngineError, "参数已失效，请重新捕获")
				return
			} else if errors.Is(err, errNoReservationAvailable) {
				e.addLog(fmt.Sprintf("%s — 名额已满，继续尝试", slotLabel))
			} else {
				e.addLog(fmt.Sprintf("%s — 预约失败: %s", slotLabel, err))
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Success!
		storeInfo, _ := client.GetStoreInfo(ctx, best.StoreID)
		storeName := storeInfo.Name
		if storeName == "" {
			storeName = best.StoreID
		}
		reservation.MonitoredStoreID = best.StoreID
		onBookingSuccess(reservation, storeName, storeInfo.Address, slotLabel, "预约")

		e.mu.Lock()
		e.state.Reservation = &reservation
		e.mu.Unlock()
		e.setState(EngineSuccess, fmt.Sprintf("预约成功！%s @ %s", slotLabel, storeName))
		e.addLog(fmt.Sprintf("🎉 预约成功！号码: %s, 时段: %s, 门店: %s", reservation.Number, slotLabel, storeName))
		return
	}
}

// Stop halts any running operation.
func (e *BookingEngine) Stop() {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if e.proxy != nil {
		e.proxy.close()
		e.proxy = nil
		ClearSystemProxy()
		markProxyInactive()
	}
	e.state.Status = EngineIdle
	e.state.Message = "已停止"
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
}

// HasValidConfig checks if we have saved authentication tokens.
func HasValidConfig() bool {
	_, err := loadLocalConfig()
	return err == nil
}
