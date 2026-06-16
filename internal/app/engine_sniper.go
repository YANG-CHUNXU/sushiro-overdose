package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type sniperTargetValidationError struct {
	Index  int          `json:"index"`
	Target SniperTarget `json:"target"`
	Reason string       `json:"reason"`
}

// StartSniper 启动 Web 版狙击计划：对一批「尚未开放、未来某时刻放号」的目标时段，
// 等到放号时刻高频轮询抢约。校验链与 StartBooking 对称：互斥 → 加载凭证 → 选门店 →
// 规整/校验目标（normalizeSniperTargetsForSettings 会过滤非法目标）→ 落盘 SniperPlan
// → GetTimeslots 验活 → 起 goroutine 跑 runSniper。
func (e *BookingEngine) StartSniper(targets []SniperTarget) error {
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
	e.state.Status = EngineSniping
	e.state.Message = "正在验证狙击计划..."
	e.state.Attempts = 0
	e.state.Reservation = nil
	e.mu.Unlock()
	bus.publish("engine", mustJSON(e.GetState()))
	pauseSamplingForMainFlow()

	tokens, err := LoadLocalConfig()
	if err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "暂无凭证参数")
		return fmt.Errorf("暂无凭证参数，请先完成参数捕获")
	}
	prefs := LoadPreferences()
	if len(prefs.SelectedStores) > 0 {
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
	targets = normalizeSniperTargetsForSettings(targets, settings)
	if len(targets) == 0 {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "暂无狙击目标")
		return fmt.Errorf("暂无有效狙击目标")
	}
	if err := SaveSniperPlan(NormalizeSniperPlan(targets, settings.Location), settings.Location); err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "保存狙击计划失败")
		return err
	}

	client := NewClient(settings)
	if _, err := client.GetTimeslots(context.Background(), settings.StoreIDs[0]); err != nil {
		e.abortStart(done, cancel)
		e.setState(EngineIdle, "验证失败")
		if isAuthError(err) {
			DeleteLocalConfig()
			noteAuthResult(err)
			return fmt.Errorf("凭证参数已过期，请重新捕获")
		}
		noteAuthResult(err)
		return fmt.Errorf("验证失败: %s", friendlyOfficialAPIError(err))
	}

	setNotifier(BuildNotifierFromConfig())
	go func() {
		defer func() {
			e.finishRun(done)
			close(done)
		}()
		e.runSniper(ctx, client, settings, targets)
	}()
	return nil
}

func normalizeSniperTargetsForSettings(targets []SniperTarget, settings Settings) []SniperTarget {
	valid, _ := validateSniperTargetsForSettings(targets, settings)
	return valid
}

func validateSniperTargetsForSettings(targets []SniperTarget, settings Settings) ([]SniperTarget, []sniperTargetValidationError) {
	out := make([]SniperTarget, 0, len(targets))
	rejected := make([]sniperTargetValidationError, 0)
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
	for i, target := range targets {
		target.Date = strings.ReplaceAll(strings.TrimSpace(target.Date), "-", "")
		target.StartAfter = NormalizeTimeStr(target.StartAfter)
		target.StartBefore = NormalizeTimeStr(target.StartBefore)
		target.StoreID = strings.TrimSpace(target.StoreID)
		if target.StoreID == "" {
			target.StoreID = defaultStore
		}
		if target.StoreID == "" {
			rejected = append(rejected, sniperTargetValidationError{Index: i, Target: target, Reason: "请选择门店"})
			continue
		}
		if _, err := ParseCompactDate(target.Date, loc); err != nil {
			rejected = append(rejected, sniperTargetValidationError{Index: i, Target: target, Reason: "日期无效，请使用 YYYY-MM-DD 或 YYYYMMDD"})
			continue
		}
		if ParseTimeSeconds(target.StartAfter) < 0 || ParseTimeSeconds(target.StartBefore) < 0 {
			rejected = append(rejected, sniperTargetValidationError{Index: i, Target: target, Reason: "时间无效，请使用 HH:MM 或 HHMM"})
			continue
		}
		if target.StartAfter >= target.StartBefore {
			rejected = append(rejected, sniperTargetValidationError{Index: i, Target: target, Reason: "最晚时间必须晚于最早时间"})
			continue
		}
		if len(allowedStores) > 0 && !allowedStores[target.StoreID] {
			rejected = append(rejected, sniperTargetValidationError{Index: i, Target: target, Reason: "门店不在已捕获门店列表中"})
			continue
		}
		out = append(out, target)
	}
	return out, rejected
}

// runSniper 是 Web 版狙击主循环。按开放时间升序逐个目标处理：
//
//	等放号 → 开放窗口内（openAt..openAt+sniperWindow=3min）高频轮询抢约 → 成功/超时收尾。
//
// 相比 runBooking（100ms 节流）这里更快：开放窗口内每轮 50ms（≈20次/秒），因为放号瞬间
// 名额秒空，必须尽快 GetTimeslots+CreateReservation。每轮状态写回 SniperPlan（UpdateSniperPlanTarget）
// 供 UI 实时看进度；为减少写盘，只在第 1 次和每 20 次落盘一次 attempts。
// 凭证失败（isAuthError）一票否决：立即删配置、终止整个计划（不重试）。
func (e *BookingEngine) runSniper(ctx context.Context, client *Client, settings Settings, targets []SniperTarget) {
	doneActivity := markMainFlowActive("sniping")
	defer doneActivity()

	sortSniperTargets(targets, settings.Location)
	e.setState(EngineSniping, fmt.Sprintf("狙击计划已启动，共 %d 个目标", len(targets)))
	e.addLog(fmt.Sprintf("开始 Web 狙击计划 — 目标数: %d", len(targets)))

	for _, target := range targets {
		targetID := sniperPlanTargetFromTarget(target, settings.Location).ID
		openAt := sniperOpenTime(target, settings.Location)
		targetLabel := fmt.Sprintf("%s %s-%s @ %s",
			target.Date, FormatCompactTime(target.StartAfter), FormatCompactTime(target.StartBefore), target.StoreID)
		UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
			t.Status = "pending"
			t.LastError = ""
		})

		// 还没到放号时刻：睡到放号前 3s 再开始密集轮询。提前 3s 是留余量——本地时钟和服务端
		// 可能有几秒偏差，提前醒来比错过放号稳妥；但离放号>5s 才睡（否则直接进轮询，select 兜底）。
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

		// 开放抢约窗口：[openAt, openAt+sniperWindow(3min)]。若 openAt 已过（目标已开放），从当下算起，
		// 仍给满 3min 窗口，让事后补救也有机会。
		deadline := openAt
		now := time.Now().In(settings.Location)
		if deadline.Before(now) {
			deadline = now
		}
		deadline = deadline.Add(sniperWindow)
		attempts := 0
		// 5xx 命中时段冷却 30s（与 booking 同机制），避免对返回 500 的时段无脑重试。
		temporarySkips := map[string]time.Time{}
		UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
			t.Status = "running"
		})

		// 开放窗口内的高频轮询：每轮 GetTimeslots 找匹配时段 → 命中即 CreateReservation。
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
			// 落盘节流：第 1 次和每 20 次才写 attempts 到 plan 文件，避免 50ms 一轮 ×
			// 每轮写盘把 IO 打爆；UI 进度靠 state.Message 实时刷新即可。
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
					noteAuthResult(err) // 凭证失败则标记 stale
					e.addLogLevel("狙击凭证失败，请重新捕获参数", "error")
					sendNotification("寿司郎狙击 - 凭证失败", "凭证参数已失效")
					DeleteLocalConfig()
					e.setState(EngineError, "凭证参数已失效，请重新捕获")
					return
				}
				if isCredentialRefreshLikelyError(err) {
					// 软过期也喂给凭证健康监测，与预约失败分支保持对称。
					noteAuthResult(err)
				}
				if isOfficialServerHTTPError(err) {
					// 5xx 用稍长的 200ms 退避，给服务端恢复时间；其他网络错误仍走 50ms 高频。
					UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
						t.Status = "running"
						t.Attempts = attempts
						t.LastError = friendlyOfficialAPIError(err)
					})
					time.Sleep(200 * time.Millisecond)
					continue
				}
				// 放号窗口内常规轮询节流 50ms（≈20次/秒），兼顾抓放号瞬间与接口压力。
				time.Sleep(50 * time.Millisecond)
				continue
			}

			markAuthHealthy() // GetTimeslots 成功 → 凭证有效
			// 匹配目标：同一日期 + 起始时间落在 [StartAfter, StartBefore) 半开区间，
			// 且状态为空或 AVAILABLE。注意 Start 用半开（>=StartAfter 且 <StartBefore），
			// 与 parseSniperArgs 校验时 StartAfter<StartBefore 配合，避免边界时段漏选/重选。
			for _, slot := range slots {
				if slot.Date != target.Date || slot.Start < target.StartAfter || slot.Start >= target.StartBefore {
					continue
				}
				// Availability 为空（接口未返回该字段）也视作可约，避免服务端字段缺失误伤。
				if slot.Availability != "" && strings.ToUpper(slot.Availability) != "AVAILABLE" {
					continue
				}
				key := bookingSlotKey(target.StoreID, slot.Date, slot.Start)
				if isTemporaryBookingSkipped(temporarySkips, key, time.Now().In(settings.Location)) {
					continue
				}
				slotLabel := FormatSlotWindow(slot.Date, slot.Start, DefaultString(slot.End, slot.Start), settings.Location)
				reservation, err := client.CreateReservation(ctx, target.StoreID, slot.Date, slot.Start)
				if err != nil {
					UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
						t.Status = "running"
						t.Attempts = attempts
						t.LastError = err.Error()
						t.LastAttemptAt = time.Now().In(settings.Location).Format(time.RFC3339)
					})
					if isAuthError(err) {
						noteAuthResult(err) // 凭证失败则标记 stale
						e.addLogLevel("预约凭证失败，终止狙击", "error")
						sendNotification("寿司郎狙击 - 凭证失败", "预约凭证参数已失效")
						DeleteLocalConfig()
						e.setState(EngineError, "预约凭证参数已失效")
						return
					}
					if errors.Is(err, ErrActiveReservationExists) {
						msg := friendlyOfficialAPIError(err)
						UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
							t.Status = "error"
							t.Attempts = attempts
							t.LastError = msg
							t.LastAttemptAt = time.Now().In(settings.Location).Format(time.RFC3339)
						})
						e.addLogLevel(slotLabel+" — "+msg, "error")
						e.setState(EngineError, msg)
						return
					}
					if isOfficialServerHTTPError(err) {
						markTemporaryBookingSkip(temporarySkips, key, time.Now().In(settings.Location))
						UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
							t.Status = "running"
							t.Attempts = attempts
							t.LastError = friendlyOfficialAPIError(err)
							t.LastAttemptAt = time.Now().In(settings.Location).Format(time.RFC3339)
						})
						e.addLog(bookingServerErrorLog(slotLabel, err))
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
				markAuthHealthy() // 狙击预约成功 → 凭证有效
				UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
					t.Status = "done"
					t.Attempts = attempts
					t.LastError = ""
					t.CompletedAt = time.Now().In(settings.Location).Format(time.RFC3339)
				})
				// 一个目标抢到即终止整轮计划：把其余 pending/open/running 目标标 stopped，
				// 因为预约系统同一账号通常只允许一个有效预约，继续抢别的没意义。
				StopRemainingSniperPlanTargetsAfterSuccess(targetID, settings.Location)
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
		sendNotification("寿司郎狙击 - 未抢到",
			fmt.Sprintf("%s 开放窗口内未预约成功。可换个时段或门店再试。", targetLabel))
	}
	e.setState(EngineIdle, "狙击计划已结束，未预约成功")
	sendNotification("寿司郎狙击 - 已结束", "狙击计划已结束，本轮未预约成功。打开应用查看详情。")
}
