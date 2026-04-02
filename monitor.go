package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

type Monitor struct {
	settings                 Settings
	client                   *Client
	dryRun                   bool
	lastWeekdayEveningMinute string
}

func NewMonitor(settings Settings, dryRun bool) *Monitor {
	return &Monitor{
		settings: settings,
		client:   NewClient(settings),
		dryRun:   dryRun,
	}
}

func (m *Monitor) RunOnce(ctx context.Context) (bool, error) {
	now := time.Now().In(m.settings.Location)
	state, err := loadState(m.settings.StateFile)
	if err != nil {
		return false, err
	}
	if pruneExpiredReservation(&state, now) {
		if state.IsZero() {
			if err := clearState(m.settings.StateFile); err != nil {
				return false, err
			}
		} else if err := saveState(m.settings.StateFile, state); err != nil {
			return false, err
		}
	}

	existing := activeReservation(state, now)
	summaryDue := shouldSendWeekendSummary(state, now)
	consoleDue := m.lastWeekdayEveningMinute != currentMinuteKey(now)

	var (
		timeslotsByStore map[string][]Slot
		storeInfos       map[string]StoreInfo
	)
	needTimeslots := summaryDue || consoleDue || existing == nil
	needStoreInfos := summaryDue || consoleDue
	if needTimeslots || needStoreInfos {
		timeslotsByStore, storeInfos, err = m.fetchRoundData(ctx, needTimeslots, needStoreInfos)
		if err != nil {
			return false, err
		}
	}

	if summaryDue {
		summaries := make([]StoreSummary, 0, len(m.settings.StoreIDs))
		for _, storeID := range m.settings.StoreIDs {
			summaries = append(summaries, StoreSummary{
				Store: storeInfos[storeID],
				Slots: filterMonthlyWeekendSlots(timeslotsByStore[storeID], now, m.settings.AvailableStatuses, 30),
			})
		}
		if err := m.sendWeekendSummary(ctx, summaries, &state, now); err != nil {
			return false, err
		}
	}

	if consoleDue {
		summaries := make([]StoreSummary, 0, len(m.settings.StoreIDs))
		for _, storeID := range m.settings.StoreIDs {
			summaries = append(summaries, StoreSummary{
				Store: storeInfos[storeID],
				Slots: filterCurrentWeekWeekdayEveningSlots(timeslotsByStore[storeID], now, m.settings.AvailableStatuses),
			})
		}
		logMessage(now, buildWeekdayEveningConsoleReport(summaries, now))
		m.lastWeekdayEveningMinute = currentMinuteKey(now)
	}

	if existing != nil {
		logMessage(now, "An active reservation already exists, skipping new reservation attempts.")
		if !state.NotificationSent {
			if err := m.sendReservationNotification(ctx, *existing, &state, now); err != nil {
				return true, err
			}
		}
		return true, nil
	}

	if timeslotsByStore == nil {
		timeslotsByStore, err = m.fetchTimeslotsByStore(ctx)
		if err != nil {
			return false, err
		}
	}

	candidates := collectCandidateSlotsByStore(timeslotsByStore, m.settings.StoreIDs, now, m.settings.AvailableStatuses, m.settings.Debug)
	if len(candidates) == 0 {
		if m.settings.Debug {
			logMessage(now, "No bookable slots found from now onward across configured stores.")
		} else {
			logMessage(now, "No bookable slots found for this week's weekend across configured stores.")
		}
		return false, nil
	}

	for _, candidate := range candidates {
		storeInfo, ok := storeInfos[candidate.StoreID]
		if !ok {
			storeInfo, err = m.client.GetStoreInfo(ctx, candidate.StoreID)
			if err != nil {
				return false, err
			}
		}

		slotLabel := formatSlotWindow(candidate.Slot.Date, candidate.Slot.Start, defaultString(candidate.Slot.End, candidate.Slot.Start), m.settings.Location)
		if m.dryRun {
			logMessage(now, fmt.Sprintf("Dry run found candidate slot: %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
			return false, nil
		}

		logMessage(now, fmt.Sprintf("Attempting reservation for %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
		reservation, err := m.client.CreateReservation(ctx, candidate.StoreID, candidate.Slot.Date, candidate.Slot.Start)
		if err != nil {
			if errors.Is(err, errNoReservationAvailable) {
				logMessage(now, fmt.Sprintf("Slot disappeared before reservation completed: %s %s", fallbackString(storeInfo.Name, candidate.StoreID), slotLabel))
				continue
			}
			return false, err
		}

		reservation.MonitoredStoreID = candidate.StoreID
		reservation.StoreName = storeInfo.Name
		reservation.StoreAddress = storeInfo.Address
		reservation.SlotLabel = formatSlotWindow(defaultString(reservation.QueueDate, candidate.Slot.Date), defaultString(reservation.Start, candidate.Slot.Start), defaultString(reservation.End, defaultString(candidate.Slot.End, candidate.Slot.Start)), m.settings.Location)

		state.ActiveReservation = &reservation
		state.NotificationSent = false
		state.SavedAt = now.Format(time.RFC3339)
		state.NotifiedAt = ""
		if err := saveState(m.settings.StateFile, state); err != nil {
			return false, err
		}
		if err := m.sendReservationNotification(ctx, reservation, &state, now); err != nil {
			return true, err
		}
		logMessage(now, fmt.Sprintf("Reservation succeeded at %s with number %s", fallbackString(storeInfo.Name, candidate.StoreID), reservation.Number))
		return true, nil
	}

	logMessage(now, "Candidates existed across configured stores, but no reservation could be created.")
	return false, nil
}

func (m *Monitor) fetchRoundData(ctx context.Context, needTimeslots, needStoreInfos bool) (map[string][]Slot, map[string]StoreInfo, error) {
	var (
		wg               sync.WaitGroup
		timeslotsByStore map[string][]Slot
		storeInfos       map[string]StoreInfo
		timeslotsErr     error
		storeInfosErr    error
	)

	if needTimeslots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timeslotsByStore, timeslotsErr = m.fetchTimeslotsByStore(ctx)
		}()
	}
	if needStoreInfos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			storeInfos, storeInfosErr = m.fetchStoreInfos(ctx)
		}()
	}
	wg.Wait()

	if timeslotsErr != nil {
		return nil, nil, timeslotsErr
	}
	if storeInfosErr != nil {
		return nil, nil, storeInfosErr
	}
	return timeslotsByStore, storeInfos, nil
}

func (m *Monitor) fetchTimeslotsByStore(ctx context.Context) (map[string][]Slot, error) {
	type result struct {
		storeID string
		slots   []Slot
		err     error
	}
	results := make(chan result, len(m.settings.StoreIDs))
	for _, storeID := range m.settings.StoreIDs {
		storeID := storeID
		go func() {
			slots, err := m.client.GetTimeslots(ctx, storeID)
			results <- result{storeID: storeID, slots: slots, err: err}
		}()
	}

	timeslotsByStore := make(map[string][]Slot, len(m.settings.StoreIDs))
	for range m.settings.StoreIDs {
		result := <-results
		if result.err != nil {
			return nil, fmt.Errorf("fetch timeslots for store %s: %w", result.storeID, result.err)
		}
		timeslotsByStore[result.storeID] = result.slots
	}
	return timeslotsByStore, nil
}

func (m *Monitor) fetchStoreInfos(ctx context.Context) (map[string]StoreInfo, error) {
	type result struct {
		storeID string
		store   StoreInfo
		err     error
	}
	results := make(chan result, len(m.settings.StoreIDs))
	for _, storeID := range m.settings.StoreIDs {
		storeID := storeID
		go func() {
			store, err := m.client.GetStoreInfo(ctx, storeID)
			results <- result{storeID: storeID, store: store, err: err}
		}()
	}

	storeInfos := make(map[string]StoreInfo, len(m.settings.StoreIDs))
	for range m.settings.StoreIDs {
		result := <-results
		if result.err != nil {
			return nil, fmt.Errorf("fetch store info for store %s: %w", result.storeID, result.err)
		}
		storeInfos[result.storeID] = result.store
	}
	return storeInfos, nil
}

func (m *Monitor) sendReservationNotification(ctx context.Context, reservation ReservationRecord, state *State, now time.Time) error {
	storeName := fallbackString(reservation.StoreName, "未知门店")
	slotLabel := fallbackString(reservation.SlotLabel, "未提供")
	reservationNumber := fallbackString(reservation.Number, "未提供")
	logMessage(now, "=== 预约成功 ===")
	logMessage(now, fmt.Sprintf("  门店：%s", storeName))
	logMessage(now, fmt.Sprintf("  时段：%s", slotLabel))
	logMessage(now, fmt.Sprintf("  号码：%s", reservationNumber))
	if reservation.StoreAddress != "" {
		logMessage(now, fmt.Sprintf("  地址：%s", reservation.StoreAddress))
	}

	title := fmt.Sprintf("寿司郎预约成功 - %s", storeName)
	message := fmt.Sprintf("号码: %s | 时段: %s", reservationNumber, slotLabel)
	_ = exec.Command("osascript", "-e",
		fmt.Sprintf(`display notification "%s" with title "%s"`, message, title),
	).Run()

	if m.settings.FeishuWebhook != "" {
		card := buildReservationSuccessCard(reservation, now)
		if err := m.client.SendFeishuCard(ctx, card); err != nil {
			logMessage(now, fmt.Sprintf("Feishu notification failed: %v", err))
		}
	}

	state.NotificationSent = true
	state.NotifiedAt = now.Format(time.RFC3339)
	return saveState(m.settings.StateFile, *state)
}

func (m *Monitor) sendWeekendSummary(ctx context.Context, summaries []StoreSummary, state *State, now time.Time) error {
	logMessage(now, "=== 未来30天周末可预约时段 ===")
	for _, summary := range summaries {
		storeName := fallbackString(summary.Store.Name, "未知门店")
		grouped := groupSlotsByDate(summary.Slots, now.Location())
		if len(grouped) == 0 {
			logMessage(now, fmt.Sprintf("  %s：暂无", storeName))
			continue
		}
		dates := make([]string, 0, len(grouped))
		for key := range grouped {
			dates = append(dates, key)
		}
		sort.Strings(dates)
		for _, dateKey := range dates {
			logMessage(now, fmt.Sprintf("  %s %s：%s", storeName, dateKey, strings.Join(grouped[dateKey], "、")))
		}
	}

	if m.settings.FeishuWebhook != "" {
		card := buildMultiStoreWeekendSummaryCard(summaries, now)
		if err := m.client.SendFeishuCard(ctx, card); err != nil {
			logMessage(now, fmt.Sprintf("Feishu notification failed: %v", err))
		}
	}

	state.LastWeekendSummaryHour = currentSummaryHourKey(now)
	state.LastWeekendSummaryAt = now.Format(time.RFC3339)
	return saveState(m.settings.StateFile, *state)
}

func runLoop(ctx context.Context, monitor *Monitor, interval time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := monitor.RunOnce(ctx); err != nil {
			fmt.Println(err)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
