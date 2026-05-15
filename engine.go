package main

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	StoreIDs        bool `json:"store_ids"`
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
		StoreIDs:        len(e.tokens.StoreIDs) > 0,
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
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.isRunningLocked() {
		e.mu.Unlock()
		cancel()
		return fmt.Errorf("引擎正在运行中")
	}
	e.cancel = cancel
	e.state.Status = EngineCapturing
	e.state.Message = "正在启动参数捕获..."
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))

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
	markProxyActive(proxyPort, os.Getpid())

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
					prefs := LoadPreferences()
					setWebSettings(tokens.toSettingsWithPrefs(prefs))
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
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.isRunningLocked() {
		e.mu.Unlock()
		cancel()
		return fmt.Errorf("引擎正在运行中")
	}
	e.cancel = cancel
	e.state.Status = EngineBooking
	e.state.Message = "正在验证认证参数..."
	e.state.Attempts = 0
	e.state.Reservation = nil
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))

	tokens, err := loadLocalConfig()
	if err != nil {
		cancel()
		e.setState(EngineIdle, "暂无认证参数")
		return fmt.Errorf("暂无认证参数，请先完成参数捕获")
	}

	prefs := LoadPreferences()
	if len(prefs.SelectedStores) > 0 {
		tokens.mu.Lock()
		tokens.StoreIDs = prefs.SelectedStores
		tokens.mu.Unlock()
	}
	if err := tokens.validateForReservation(); err != nil {
		cancel()
		e.setState(EngineIdle, "预约参数不完整")
		return err
	}

	settings := tokens.toSettingsWithPrefs(prefs)
	client := NewClient(settings)

	// Quick verify
	if _, err := client.GetTimeslots(context.Background(), settings.StoreIDs[0]); err != nil {
		cancel()
		e.setState(EngineIdle, "验证失败")
		if isAuthError(err) {
			deleteLocalConfig()
			return fmt.Errorf("认证参数已过期，请重新捕获")
		}
		return fmt.Errorf("验证失败: %w", err)
	}

	setNotifier(BuildNotifierFromConfig())

	go e.runBooking(ctx, client, settings, prefs)
	return nil
}

func (e *BookingEngine) isRunningLocked() bool {
	return e.state.Status == EngineCapturing ||
		e.state.Status == EngineBooking ||
		e.state.Status == EngineSniping
}

func (e *BookingEngine) StartSniper(targets []SniperTarget) error {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.isRunningLocked() {
		e.mu.Unlock()
		cancel()
		return fmt.Errorf("引擎正在运行中")
	}
	e.cancel = cancel
	e.state.Status = EngineSniping
	e.state.Message = "正在验证狙击计划..."
	e.state.Attempts = 0
	e.state.Reservation = nil
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))

	tokens, err := loadLocalConfig()
	if err != nil {
		cancel()
		e.setState(EngineIdle, "暂无认证参数")
		return fmt.Errorf("暂无认证参数，请先完成参数捕获")
	}
	prefs := LoadPreferences()
	if len(prefs.SelectedStores) > 0 {
		tokens.mu.Lock()
		tokens.StoreIDs = prefs.SelectedStores
		tokens.mu.Unlock()
	}
	if err := tokens.validateForReservation(); err != nil {
		cancel()
		e.setState(EngineIdle, "预约参数不完整")
		return err
	}
	settings := tokens.toSettingsWithPrefs(prefs)
	targets = normalizeSniperTargetsForSettings(targets, settings)
	if len(targets) == 0 {
		cancel()
		e.setState(EngineIdle, "暂无狙击目标")
		return fmt.Errorf("暂无有效狙击目标")
	}
	if err := SaveSniperPlan(NormalizeSniperPlan(targets, settings.Location), settings.Location); err != nil {
		cancel()
		e.setState(EngineIdle, "保存狙击计划失败")
		return err
	}

	client := NewClient(settings)
	if _, err := client.GetTimeslots(context.Background(), settings.StoreIDs[0]); err != nil {
		cancel()
		e.setState(EngineIdle, "验证失败")
		if isAuthError(err) {
			deleteLocalConfig()
			return fmt.Errorf("认证参数已过期，请重新捕获")
		}
		return fmt.Errorf("验证失败: %w", err)
	}

	setNotifier(BuildNotifierFromConfig())
	go e.runSniper(ctx, client, settings, targets)
	return nil
}

func normalizeSniperTargetsForSettings(targets []SniperTarget, settings Settings) []SniperTarget {
	out := make([]SniperTarget, 0, len(targets))
	loc := settings.Location
	if loc == nil {
		loc = time.Local
	}
	defaultStore := ""
	if len(settings.StoreIDs) > 0 {
		defaultStore = settings.StoreIDs[0]
	}
	allowedStores := map[string]bool{}
	for _, storeID := range settings.StoreIDs {
		allowedStores[storeID] = true
	}
	for _, target := range targets {
		target.Date = strings.TrimSpace(target.Date)
		target.StartAfter = normalizeTimeStr(target.StartAfter)
		target.StartBefore = normalizeTimeStr(target.StartBefore)
		target.StoreID = strings.TrimSpace(target.StoreID)
		if target.StoreID == "" {
			target.StoreID = defaultStore
		}
		if _, err := parseCompactDate(target.Date, loc); err != nil {
			continue
		}
		if parseTimeSeconds(target.StartAfter) < 0 || parseTimeSeconds(target.StartBefore) < 0 {
			continue
		}
		if target.StartAfter >= target.StartBefore {
			continue
		}
		if len(allowedStores) > 0 && !allowedStores[target.StoreID] {
			continue
		}
		out = append(out, target)
	}
	return out
}

func (e *BookingEngine) runSniper(ctx context.Context, client *Client, settings Settings, targets []SniperTarget) {
	sortSniperTargets(targets, settings.Location)
	e.setState(EngineSniping, fmt.Sprintf("狙击计划已启动，共 %d 个目标", len(targets)))
	e.addLog(fmt.Sprintf("开始 Web 狙击计划 — 目标数: %d", len(targets)))

	for _, target := range targets {
		targetID := sniperPlanTargetFromTarget(target, settings.Location).ID
		openAt := sniperOpenTime(target, settings.Location)
		targetLabel := fmt.Sprintf("%s %s-%s @ %s",
			target.Date, formatCompactTime(target.StartAfter), formatCompactTime(target.StartBefore), target.StoreID)
		UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
			t.Status = "pending"
			t.LastError = ""
		})

		if openAt.After(time.Now().In(settings.Location)) {
			wait := time.Until(openAt)
			e.setState(EngineSniping, fmt.Sprintf("等待开放: %s", targetLabel))
			if wait > 5*time.Second {
				select {
				case <-ctx.Done():
					e.setState(EngineIdle, "已停止狙击")
					return
				case <-time.After(wait - 3*time.Second):
				}
			}
		}

		deadline := openAt
		now := time.Now().In(settings.Location)
		if deadline.Before(now) {
			deadline = now
		}
		deadline = deadline.Add(sniperWindow)
		attempts := 0
		UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
			t.Status = "running"
		})

		for time.Now().In(settings.Location).Before(deadline) {
			select {
			case <-ctx.Done():
				e.setState(EngineIdle, "已停止狙击")
				return
			default:
			}

			attempts++
			e.mu.Lock()
			e.state.Attempts++
			e.state.LastCheck = time.Now().Format("15:04:05")
			e.state.Message = fmt.Sprintf("狙击中...%s，第 %d 次", targetLabel, attempts)
			e.mu.Unlock()
			if attempts == 1 || attempts%20 == 0 {
				UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
					t.Status = "running"
					t.Attempts = attempts
					t.LastAttemptAt = time.Now().In(settings.Location).Format(time.RFC3339)
				})
				bus.publish("engine", mustJSON(e.GetState()))
			}

			slots, err := client.GetTimeslots(ctx, target.StoreID)
			if err != nil {
				UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
					t.Status = "running"
					t.Attempts = attempts
					t.LastError = err.Error()
				})
				if isAuthError(err) {
					e.addLogLevel("狙击认证失败，请重新捕获参数", "error")
					sendNotification("寿司郎狙击 - 认证失败", "认证参数已失效")
					deleteLocalConfig()
					e.setState(EngineError, "认证参数已失效，请重新捕获")
					return
				}
				if isHTTPStatus(err, 500) {
					e.addLogLevel("狙击接口 HTTP 500，参数可能已失效", "error")
					sendNotification("寿司郎狙击 - HTTP 500", "参数可能已失效")
					deleteLocalConfig()
					e.setState(EngineError, "参数已失效，请重新捕获")
					return
				}
				time.Sleep(50 * time.Millisecond)
				continue
			}

			for _, slot := range slots {
				if slot.Date != target.Date || slot.Start < target.StartAfter || slot.Start >= target.StartBefore {
					continue
				}
				if slot.Availability != "" && strings.ToUpper(slot.Availability) != "AVAILABLE" {
					continue
				}
				slotLabel := formatSlotWindow(slot.Date, slot.Start, defaultString(slot.End, slot.Start), settings.Location)
				reservation, err := client.CreateReservation(ctx, target.StoreID, slot.Date, slot.Start)
				if err != nil {
					UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
						t.Status = "running"
						t.Attempts = attempts
						t.LastError = err.Error()
						t.LastAttemptAt = time.Now().In(settings.Location).Format(time.RFC3339)
					})
					if isAuthError(err) {
						e.addLogLevel("预约认证失败，终止狙击", "error")
						sendNotification("寿司郎狙击 - 认证失败", "预约认证参数已失效")
						deleteLocalConfig()
						e.setState(EngineError, "预约认证参数已失效")
						return
					}
					if isHTTPStatus(err, 500) {
						e.addLogLevel("预约接口 HTTP 500，参数可能已失效", "error")
						sendNotification("寿司郎狙击 - HTTP 500", "参数可能已失效")
						deleteLocalConfig()
						e.setState(EngineError, "参数已失效，请重新捕获")
						return
					}
					continue
				}

				storeInfo, _ := client.GetStoreInfo(ctx, target.StoreID)
				storeName := storeInfo.Name
				if storeName == "" {
					storeName = target.StoreID
				}
				reservation.MonitoredStoreID = target.StoreID
				onBookingSuccess(reservation, storeName, storeInfo.Address, slotLabel, "狙击")
				UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
					t.Status = "done"
					t.Attempts = attempts
					t.LastError = ""
					t.CompletedAt = time.Now().In(settings.Location).Format(time.RFC3339)
				})
				e.mu.Lock()
				e.state.Reservation = &reservation
				e.mu.Unlock()
				e.setState(EngineSuccess, fmt.Sprintf("狙击成功！%s @ %s", slotLabel, storeName))
				return
			}
			time.Sleep(50 * time.Millisecond)
		}

		UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
			t.Status = "expired"
			t.Attempts = attempts
			t.LastError = "开放窗口内未预约成功"
		})
		e.addLog(fmt.Sprintf("狙击目标超时: %s", targetLabel))
	}
	e.setState(EngineIdle, "狙击计划已结束，未预约成功")
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
				if best == nil || prefs.PreferTargetSlot(*t, *best, settings.Location, settings.StoreIDs) {
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

			if isHTTPStatus(err, 500) {
				e.addLogLevel("预约接口 HTTP 500，参数可能已失效", "error")
				sendNotification("寿司郎 - HTTP 500", "参数可能已失效")
				deleteLocalConfig()
				e.setState(EngineError, "参数已失效，请重新捕获")
				return
			} else if errors.Is(err, errNoReservationAvailable) {
				key := best.StoreID + best.Date + best.Start
				if booked == nil {
					booked = make(map[string]bool)
				}
				booked[key] = true
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
	tokens, err := loadLocalConfig()
	return err == nil && tokens.validateForReservation() == nil
}
