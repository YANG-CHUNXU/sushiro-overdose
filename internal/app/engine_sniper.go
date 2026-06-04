package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type sniperTargetValidationError struct {
	Index  int          `json:"index"`
	Target SniperTarget `json:"target"`
	Reason string       `json:"reason"`
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
		e.setState(EngineIdle, "暂无认证参数")
		return fmt.Errorf("暂无认证参数，请先完成参数捕获")
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
		temporarySkips := map[string]time.Time{}
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
					DeleteLocalConfig()
					e.setState(EngineError, "认证参数已失效，请重新捕获")
					return
				}
				if isOfficialServerHTTPError(err) {
					UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
						t.Status = "running"
						t.Attempts = attempts
						t.LastError = friendlyOfficialAPIError(err)
					})
					time.Sleep(200 * time.Millisecond)
					continue
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
						e.addLogLevel("预约认证失败，终止狙击", "error")
						sendNotification("寿司郎狙击 - 认证失败", "预约认证参数已失效")
						DeleteLocalConfig()
						e.setState(EngineError, "预约认证参数已失效")
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
				UpdateSniperPlanTarget(targetID, settings.Location, func(t *SniperPlanTarget) {
					t.Status = "done"
					t.Attempts = attempts
					t.LastError = ""
					t.CompletedAt = time.Now().In(settings.Location).Format(time.RFC3339)
				})
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
	}
	e.setState(EngineIdle, "狙击计划已结束，未预约成功")
}
