package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/platform"

import . "github.com/Ryujoxys/sushiro-overdose/internal/proxy"

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
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
	done   chan struct{}
	tokens *CapturedTokens
	proxy  *ProxyServer
	logs   []LogEntry
	// pinnedSlot，非空时 booking 只预约这一个确切时段（直接预约），不按偏好扫描。
	// 仅单次运行有效，finishRun 清空。
	pinnedSlot *TargetSlot
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
	return captureStatusForTokens(e.tokens)
}

func captureStatusForTokens(tokens *CapturedTokens) *CaptureStatusJSON {
	if tokens == nil {
		return nil
	}
	tokens.Lock()
	defer tokens.Unlock()
	return &CaptureStatusJSON{
		XAppCode:        tokens.XAppCode != "",
		QueryAuth:       tokens.QueryAuth != "",
		ReservationAuth: tokens.ReservationAuth != "",
		UserAgent:       tokens.UserAgent != "",
		Referer:         tokens.Referer != "",
		WechatID:        tokens.WechatID != "",
		PhoneNumber:     tokens.PhoneNumber != "",
		StoreIDs:        len(tokens.StoreIDs) > 0,
		Complete:        tokens.IsCompleteUnlocked(),
	}
}

func (e *BookingEngine) setState(status EngineStatus, message string) {
	e.mu.Lock()
	e.state.Status = status
	e.state.Message = message
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
}

func (e *BookingEngine) finishRun(done chan struct{}) {
	e.mu.Lock()
	if e.done == done {
		e.cancel = nil
		e.done = nil
	}
	e.pinnedSlot = nil
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
}

func (e *BookingEngine) abortStart(done chan struct{}, cancel context.CancelFunc) {
	if cancel != nil {
		cancel()
	}
	e.finishRun(done)
	close(done)
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
	LogMessage(time.Now(), msg)
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
	done := make(chan struct{})
	e.done = done
	e.state.Status = EngineCapturing
	e.state.Message = "正在启动参数捕获..."
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
	pauseSamplingForMainFlow()

	go func() {
		defer func() {
			e.finishRun(done)
			close(done)
		}()
		e.runCapture(ctx)
	}()
	return nil
}

func (e *BookingEngine) runCapture(ctx context.Context) {
	doneActivity := markMainFlowActive("capturing")
	defer doneActivity()

	e.setState(EngineCapturing, "正在准备证书...")

	caCert, caKey, err := LoadOrGenerateCA()
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

	tokens := NewCapturedTokens()
	e.mu.Lock()
	e.tokens = tokens
	e.mu.Unlock()

	proxy, err := StartProxy(caCert, caKey, tokens, e.addLog)
	if err != nil {
		e.setState(EngineError, "启动代理失败: "+err.Error())
		e.addLogLevel("启动代理失败: "+err.Error(), "error")
		return
	}
	e.mu.Lock()
	e.proxy = proxy
	e.mu.Unlock()
	actualPort := proxy.Port()

	if err := SetSystemProxy(actualPort); err != nil {
		proxy.Close()
		e.setState(EngineError, "设置系统代理失败: "+err.Error())
		e.addLogLevel("设置系统代理失败: "+err.Error(), "error")
		return
	}
	markProxyActive(actualPort, os.Getpid())

	e.setState(EngineCapturing, "等待捕获认证参数，请彻底关闭 PC 微信后重新打开，并在寿司郎小程序里点一次排队或预约...")
	proxyHint := fmt.Sprintf("捕获代理已设置 (127.0.0.1:%d)", actualPort)
	if GetActiveWebPort() > 0 && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
		proxyHint += "；已使用 PAC 仅代理寿司郎域名"
	}
	e.addLog(proxyHint + "。请彻底关闭 PC 微信后重新打开，进入寿司郎小程序，选任意门店点一次「排队」或「预约」（不用真的提交）")
	e.addLog("提示：如果 PC 微信小程序弹出“服务器出错/网络错误”，但本工具抓到认证并通过基础接口自检，可以直接忽略小程序弹窗。")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	savedForDiscovery := false

	for {
		select {
		case <-ctx.Done():
			e.cleanupProxy()
			e.setState(EngineIdle, "已停止捕获")
			return
		case <-ticker.C:
			bus.publish("engine", mustJSON(e.GetState()))
			if tokens.IsComplete() {
				prefs := LoadPreferences()
				// 接口发现调试开启时：抓到认证先存好，但保持代理运行，让用户继续在
				// 小程序里点想记录的接口（如「排队取号」），手动「停止」前不拆代理。
				// 否则代理会在认证一抓全就关闭，后续接口根本来不及记录。
				if APIDiscoveryEnabled() {
					if !savedForDiscovery {
						if err := SaveLocalConfig(tokens); err == nil {
							setWebSettings(tokens.ToSettingsWithPrefs(prefs))
						}
						savedForDiscovery = true
						e.addLog("认证已抓到并保存；接口发现调试开启中——请在小程序里点你想记录的接口（如「排队取号」），完成后点「停止」。")
						e.setState(EngineCapturing, "接口发现调试中：认证已抓到，代理保持运行。请在小程序里操作要记录的接口，记录完点「停止」。")
					}
					continue
				}
				// 自检只作诊断，不拦保存：抓到完整认证就落盘并完成捕获，
				// 自检结果仅决定提示语。避免基础接口偶发失败/被拒时把用户卡死。
				e.setState(EngineCapturing, "已抓到认证参数，正在自检基础接口...")
				report := runAuthProbeWithTokens(ctx, "", tokens, prefs)
				if err := SaveLocalConfig(tokens); err != nil {
					e.addLogLevel("保存配置失败: "+err.Error(), "error")
					e.cleanupProxy()
					e.setState(EngineError, "认证参数保存失败: "+err.Error())
					return
				}
				setWebSettings(tokens.ToSettingsWithPrefs(prefs))
				e.cleanupProxy()
				if report.OK {
					e.addLog("认证参数已捕获、基础接口自检通过并保存！")
					e.setState(EngineIdle, "认证参数捕获完成！")
				} else {
					e.addLogLevel("认证参数已捕获并保存；基础接口自检未通过（"+authProbeFailureSummary(report)+"），可直接尝试使用，如不可用再重新捕获。", "warn")
					e.setState(EngineIdle, "认证参数已保存；基础接口自检未通过，但可直接尝试使用。即使小程序显示服务器出错也不影响。")
				}

				tokens.Lock()
				if len(tokens.StoreIDs) > 0 && len(prefs.SelectedStores) == 0 {
					prefs.SelectedStores = tokens.StoreIDs
					SavePreferences(prefs)
				}
				tokens.Unlock()
				return
			}
		}
	}
}

func (e *BookingEngine) cleanupProxy() {
	e.mu.Lock()
	if e.proxy != nil {
		e.proxy.Close()
		e.proxy = nil
	}
	e.mu.Unlock()
	ClearSystemProxy()
	markProxyInactive()
}

// ResetCapture 强制停止当前抓包/运行、关闭代理并清理系统代理残留，把引擎重置回
// idle。抓到认证后会自动断连，状态可能卡在 capturing（尤其接口发现保持连接时），
// 导致「获取认证」被 isRunningLocked 挡住而连不回来；重置后即可手动重新连接抓包。
func (e *BookingEngine) ResetCapture() {
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.cleanupProxy()
	checkStaleProxy()
	e.setState(EngineIdle, "已重置抓包状态，可点「获取认证」手动重新连接")
	e.addLog("已重置抓包状态：代理已断开并清理，点「获取认证」可重新连接抓包。")
}

// StartBooking begins the automated booking loop.
// normalizeBookingTime 把 HHMM / HH:MM:SS 等规范成官方接口要求的 HHMMSS。
func normalizeBookingTime(t string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, t)
	if len(digits) == 4 {
		return digits + "00"
	}
	return digits
}

// StartBookingSlot 直接预约一个确切时段（来自「预约未来」里已放出、可约的卡片）。
// 与 StartBooking 共用校验/运行链路，只是钉死单个目标、不按偏好扫描，且为一次性。
func (e *BookingEngine) StartBookingSlot(storeID, date, start, end string) error {
	storeID = strings.TrimSpace(storeID)
	date = strings.TrimSpace(date)
	start = normalizeBookingTime(start)
	end = normalizeBookingTime(end)
	if storeID == "" || date == "" || start == "" {
		return fmt.Errorf("时段信息不完整，无法直接预约")
	}
	e.mu.Lock()
	if e.isRunningLocked() {
		e.mu.Unlock()
		return fmt.Errorf("引擎正在运行中")
	}
	e.pinnedSlot = &TargetSlot{StoreID: storeID, Date: date, Start: start, End: end}
	e.mu.Unlock()
	if err := e.StartBooking(); err != nil {
		e.mu.Lock()
		e.pinnedSlot = nil
		e.mu.Unlock()
		return err
	}
	return nil
}

func (e *BookingEngine) StartBooking() error {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.isRunningLocked() {
		e.mu.Unlock()
		cancel()
		return fmt.Errorf("引擎正在运行中")
	}
	e.cancel = cancel
	done := make(chan struct{})
	e.done = done
	e.state.Status = EngineBooking
	e.state.Message = "正在验证认证参数..."
	e.state.Attempts = 0
	e.state.Reservation = nil
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
	pauseSamplingForMainFlow()

	tokens, err := LoadLocalConfig()
	if err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "暂无认证参数")
		return fmt.Errorf("暂无认证参数，请先完成参数捕获")
	}

	prefs := LoadPreferences()
	e.mu.RLock()
	pinned := e.pinnedSlot
	e.mu.RUnlock()
	if pinned != nil {
		// 直接预约：钉死该时段所属门店，即使它不在偏好门店里。
		tokens.Lock()
		tokens.StoreIDs = []string{pinned.StoreID}
		tokens.Unlock()
	} else if len(prefs.SelectedStores) > 0 {
		tokens.Lock()
		tokens.StoreIDs = prefs.SelectedStores
		tokens.Unlock()
	}
	if err := tokens.ValidateForReservation(); err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "预约参数不完整")
		return err
	}

	settings := tokens.ToSettingsWithPrefs(prefs)
	client := NewClient(settings)

	// Quick verify
	if _, err := client.GetTimeslots(context.Background(), settings.StoreIDs[0]); err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "验证失败")
		if isAuthError(err) {
			DeleteLocalConfig()
			return fmt.Errorf("认证参数已过期，请重新捕获")
		}
		return fmt.Errorf("验证失败: %w", err)
	}

	setNotifier(BuildNotifierFromConfig())

	go func() {
		defer func() {
			e.finishRun(done)
			close(done)
		}()
		e.runBooking(ctx, client, settings, prefs)
	}()
	return nil
}

func (e *BookingEngine) isRunningLocked() bool {
	return e.state.Status == EngineCapturing ||
		e.state.Status == EngineBooking ||
		e.state.Status == EngineSniping
}

func (e *BookingEngine) runBooking(ctx context.Context, client *Client, settings Settings, prefs UserPreferences) {
	doneActivity := markMainFlowActive("booking")
	defer doneActivity()

	e.setState(EngineBooking, "正在抢号...")
	e.addLog(fmt.Sprintf("开始抢号 — 门店: %v, 成人: %d, 儿童: %d", settings.StoreIDs, settings.Adult, settings.Child))

	healthStop := startHealthCheck(ctx, client, settings.StoreIDs)
	defer close(healthStop)

	// 直接预约：钉死单个时段，一次性尝试（成功即停，不可约/业务错误即停），不进入偏好扫描循环。
	e.mu.RLock()
	pinned := e.pinnedSlot
	e.mu.RUnlock()
	if pinned != nil {
		best := pinned
		slotLabel := FormatSlotWindow(best.Date, best.Start, best.End, settings.Location)
		e.addLog(fmt.Sprintf("直接预约: %s — 尝试预约...", slotLabel))
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			select {
			case <-ctx.Done():
				e.setState(EngineIdle, "已停止")
				return
			default:
			}
			reservation, err := client.CreateReservation(ctx, best.StoreID, best.Date, best.Start)
			if err == nil {
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
			lastErr = err
			if isAuthError(err) {
				e.addLogLevel("认证失败", "error")
				sendNotification("寿司郎 - 认证失败", "请重新捕获参数")
				DeleteLocalConfig()
				e.setState(EngineError, "认证参数已失效")
				return
			}
			if errors.Is(err, ErrActiveReservationExists) {
				msg := friendlyOfficialAPIError(err)
				e.addLogLevel(slotLabel+" — "+msg, "error")
				e.setState(EngineError, msg)
				return
			}
			if errors.Is(err, ErrNoReservationAvailable) {
				e.addLogLevel(slotLabel+" — 这个时段已经约满或被抢走了", "error")
				e.setState(EngineError, "这个时段已经约满或被抢走了；可以挑别的时段，或对未放出的时段加入狙击。")
				return
			}
			if isOfficialServerHTTPError(err) {
				e.addLog(bookingServerErrorLog(slotLabel, err))
				time.Sleep(400 * time.Millisecond)
				continue
			}
			e.addLogLevel(fmt.Sprintf("%s — 预约失败: %s", slotLabel, err), "error")
			e.setState(EngineError, "预约失败："+err.Error())
			return
		}
		e.addLogLevel(fmt.Sprintf("%s — 预约失败: %v", slotLabel, lastErr), "error")
		e.setState(EngineError, "这个时段暂时预约不上，请稍后重试或挑别的时段。")
		return
	}

	var booked map[string]bool
	temporarySkips := map[string]time.Time{}
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
						DeleteLocalConfig()
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
				key := bookingSlotKey(storeID, slots[i].Date, slots[i].Start)
				if booked != nil && booked[key] {
					continue
				}
				if isTemporaryBookingSkipped(temporarySkips, key, now) {
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

		slotLabel := FormatSlotWindow(best.Date, best.Start, best.End, settings.Location)
		e.addLog(fmt.Sprintf("发现目标: %s — 尝试预约...", slotLabel))

		reservation, err := client.CreateReservation(ctx, best.StoreID, best.Date, best.Start)
		if err != nil {
			if isAuthError(err) {
				authErrors++
				if authErrors >= 3 {
					e.addLogLevel("认证失败", "error")
					sendNotification("寿司郎 - 认证失败", "请重新捕获参数")
					DeleteLocalConfig()
					e.setState(EngineError, "认证参数已失效")
					return
				}
			}

			if errors.Is(err, ErrActiveReservationExists) {
				msg := friendlyOfficialAPIError(err)
				e.addLogLevel(slotLabel+" — "+msg, "error")
				e.setState(EngineError, msg)
				return
			}

			if isOfficialServerHTTPError(err) {
				key := bookingSlotKey(best.StoreID, best.Date, best.Start)
				markTemporaryBookingSkip(temporarySkips, key, now)
				e.addLog(bookingServerErrorLog(slotLabel, err))
			} else if errors.Is(err, ErrNoReservationAvailable) {
				key := bookingSlotKey(best.StoreID, best.Date, best.Start)
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
	cancel := e.cancel
	done := e.done
	if e.cancel != nil {
		e.cancel()
	}
	if e.proxy != nil {
		e.proxy.Close()
		e.proxy = nil
		ClearSystemProxy()
		markProxyInactive()
	}
	e.mu.Unlock()

	if cancel != nil && done != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}

	e.mu.Lock()
	if e.done == done {
		e.cancel = nil
		e.done = nil
	}
	e.state.Status = EngineIdle
	e.state.Message = "已停止"
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
}

// HasValidConfig checks if we have saved authentication tokens.
func HasValidConfig() bool {
	tokens, err := LoadLocalConfig()
	return err == nil && tokens.ValidateForReservation() == nil
}
