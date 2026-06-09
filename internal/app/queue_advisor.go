package app

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// queue_advisor.go 把实时快照 + 本机采样 + 历史趋势合成「排队压力 + 时间答案」。
// 用户侧统一用「排队压力 / 预计等待 / 叫号速度 / 消化趋势」，不引入「最新发出号」之类口径。
// 大部分推算复用 queue_live_panel.go / queue_alert_status.go / queue_trends.go 的现成 helper。

const (
	queueAdvisorWindow15 = 15 * time.Minute
	queueAdvisorWindow30 = 30 * time.Minute
	queueAdvisorWindow60 = 60 * time.Minute

	queuePressureCurveLocalPreferredPoints = 8
)

type QueueTimeRange struct {
	Early string `json:"early,omitempty"`
	Late  string `json:"late,omitempty"`
}

type QueueWaitRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

type QueueAdvisorCurrent struct {
	CalledNo            int    `json:"called_no"`
	WaitingGroups       int    `json:"waiting_groups"`
	OfficialWaitMinutes int    `json:"official_wait_minutes"`
	StoreStatus         string `json:"store_status"`
	NetTicketStatus     string `json:"net_ticket_status"`
	OnlineOpen          bool   `json:"online_open"`
}

type QueueAdvisorPressure struct {
	Level      string `json:"level"`       // low/medium/high/extreme/unknown
	Label      string `json:"label"`       // 低/中/高/极高/数据不足
	Score      int    `json:"score"`       // 0-100
	Trend      string `json:"trend"`       // improving/stable/worsening/stalled/unknown
	TrendLabel string `json:"trend_label"` // 正在变好/基本稳定/正在变差/叫号停滞/数据不足
	Reason     string `json:"reason"`
}

type QueueAdvisorSpeed struct {
	CalledPerMin15 *float64 `json:"called_per_min_15,omitempty"`
	CalledPerMin30 *float64 `json:"called_per_min_30,omitempty"`
	CalledPerMin60 *float64 `json:"called_per_min_60,omitempty"`
}

type QueueAdvisorEta struct {
	TargetNo               int             `json:"target_no"`
	RemainingGroups        int             `json:"remaining_groups"`
	EstimatedCalledAt      string          `json:"estimated_called_at,omitempty"`
	EstimatedCalledAtRange *QueueTimeRange `json:"estimated_called_at_range,omitempty"`
	WaitMinutesRange       *QueueWaitRange `json:"wait_minutes_range,omitempty"`
	ArrivalSuggestion      string          `json:"arrival_suggestion,omitempty"`
	Source                 string          `json:"source,omitempty"`
	SourceLabel            string          `json:"source_label,omitempty"`
	SourceNote             string          `json:"source_note,omitempty"`
	Risk                   string          `json:"risk"` // low/medium/high/unknown
}

type QueueAdvisor struct {
	StoreID        string               `json:"store_id"`
	StoreName      string               `json:"store_name"`
	GeneratedAt    string               `json:"generated_at"`
	Current        QueueAdvisorCurrent  `json:"current"`
	Pressure       QueueAdvisorPressure `json:"pressure"`
	Speed          QueueAdvisorSpeed    `json:"speed"`
	Eta            *QueueAdvisorEta     `json:"eta,omitempty"`
	SamplingPoints int                  `json:"sampling_points"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// ---------- 排队压力模型（纯函数，便于测试） ----------

// queuePressureLevel 用当前等待桌数与官方等待分钟综合判档；两者都缺为 unknown。
func queuePressureLevel(waitGroups, waitMinutes int) string {
	if waitGroups <= 0 && waitMinutes <= 0 {
		return "unknown"
	}
	switch {
	case (waitGroups > 0 && waitGroups <= 20) || (waitMinutes > 0 && waitMinutes <= 20):
		return "low"
	case (waitGroups > 0 && waitGroups <= 60) || (waitMinutes > 0 && waitMinutes <= 60):
		return "medium"
	case (waitGroups > 0 && waitGroups <= 120) || (waitMinutes > 0 && waitMinutes <= 120):
		return "high"
	default:
		return "extreme"
	}
}

func queuePressureLabel(level string) string {
	switch level {
	case "low":
		return "低"
	case "medium":
		return "中"
	case "high":
		return "高"
	case "extreme":
		return "极高"
	default:
		return "数据不足"
	}
}

// queuePressureScore 把等待桌数/分钟单调映射到 0-100，用于面积/柱高度。
func queuePressureScore(waitGroups, waitMinutes int) int {
	if waitGroups <= 0 && waitMinutes <= 0 {
		return 0
	}
	byGroups := float64(waitGroups) / 160.0 * 100.0
	byMinutes := float64(waitMinutes) / 160.0 * 100.0
	score := math.Max(byGroups, byMinutes)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return int(math.Round(score))
}

// queuePressureTrend 用近窗等待桌数变化 + 叫号速度判断消化趋势。
func queuePressureTrend(recent []QueueObservation, rate float64, stalled bool) (string, string) {
	if stalled {
		return "stalled", "叫号停滞"
	}
	if len(recent) < 2 {
		return "unknown", "数据不足"
	}
	first := recent[0].GroupQueuesCount
	last := recent[len(recent)-1].GroupQueuesCount
	if first <= 0 && last <= 0 {
		return "unknown", "数据不足"
	}
	delta := last - first
	switch {
	case delta <= -8:
		return "improving", "正在变好"
	case delta >= 8:
		return "worsening", "正在变差"
	case rate <= 0:
		return "unknown", "数据不足"
	default:
		return "stable", "基本稳定"
	}
}

func queuePressureReason(level, trend string, waitGroups int, rate float64, called15 *int) string {
	parts := []string{}
	if waitGroups > 0 {
		parts = append(parts, fmt.Sprintf("当前在等约 %d 桌", waitGroups))
	}
	if called15 != nil {
		parts = append(parts, fmt.Sprintf("近 15 分钟叫了 %d 桌", *called15))
	} else if rate > 0 {
		parts = append(parts, fmt.Sprintf("近期约 %.1f 桌/分钟", rate))
	}
	switch trend {
	case "improving":
		parts = append(parts, "队伍在加速消化")
	case "worsening":
		parts = append(parts, "积压还在变多")
	case "stalled":
		parts = append(parts, "叫号暂时没有推进")
	}
	if len(parts) == 0 {
		return "实时数据不足，先按官方等待参考"
	}
	return strings.Join(parts, "，") + "。"
}

// ---------- 叫号速度（多窗口） ----------

// calledRateOverWindow 复用 recentStoreObservations + calledRatePerMinute 算某窗口的叫号速度。
func calledRateOverWindow(all []QueueObservation, storeID string, now time.Time, window time.Duration) *float64 {
	recent := recentStoreObservations(all, storeID, now, window)
	if len(recent) < 2 {
		return nil
	}
	rate, ok := calledRatePerMinute(recent)
	if !ok || rate <= 0 {
		return nil
	}
	r := math.Round(rate*10) / 10
	return &r
}

// ---------- 预计等待区间 ----------

// estimateWaitRange 预计等待分钟区间：官方 → 近窗速度 → 历史 P50/P80 → 数据不足。
func estimateWaitRange(waitGroups, officialWait int, rate float64, hist *QueueWaitRange) (*QueueWaitRange, string) {
	if rate > 0 && waitGroups > 0 {
		base := float64(waitGroups) / rate
		low := int(math.Floor(base * 0.85))
		high := int(math.Ceil(base * 1.2))
		if officialWait > 0 {
			// 官方等待作为下界兜底，避免本机速度过于乐观。
			if officialWait < low {
				low = officialWait
			}
		}
		if high < low {
			high = low
		}
		return &QueueWaitRange{Low: max(0, low), High: max(0, high)}, "recent_speed"
	}
	if officialWait > 0 {
		low := int(math.Floor(float64(officialWait) * 0.9))
		high := int(math.Ceil(float64(officialWait) * 1.15))
		return &QueueWaitRange{Low: low, High: high}, "official"
	}
	if hist != nil && (hist.Low > 0 || hist.High > 0) {
		return &QueueWaitRange{Low: hist.Low, High: hist.High}, "history"
	}
	return nil, "unknown"
}

// ---------- advisor 主入口 ----------

func buildQueueAdvisor(ctx context.Context, storeID string, targetNo, travelMinutes int, now time.Time) (QueueAdvisor, error) {
	if now.IsZero() {
		now = time.Now()
	}
	store, err := NewQueueLiveClient().GetStore(ctx, storeID)
	if err != nil {
		return QueueAdvisor{}, err
	}
	snapshot := queueObservationFromLiveStore(store, now)

	all := loadQueueObservations()
	history := recentStoreObservations(all, snapshot.StoreID, now, queuePanelRateWindow)
	if snapshot.DisplayCalledNo > 0 {
		history = append(history, snapshot)
	}
	recent15 := recentStoreObservations(all, snapshot.StoreID, now, queueAdvisorWindow15)
	if snapshot.DisplayCalledNo > 0 {
		recent15 = append(recent15, snapshot)
	}

	warnings := queueAlertStoreWarnings(all, snapshot.StoreID, now)
	stalled := false
	for _, w := range warnings {
		if strings.Contains(w, "没有推进") {
			stalled = true
		}
	}

	rate60 := calledRateOverWindow(all, snapshot.StoreID, now, queueAdvisorWindow60)
	rate := 0.0
	if r, ok := calledRatePerMinute(history); ok && r > 0 {
		rate = r
	}

	var called15 *int
	if c, ok := calledAdvanceWithin(history, now, queueAdvisorWindow15); ok {
		called15 = &c
	}

	level := queuePressureLevel(snapshot.GroupQueuesCount, store.Wait)
	trend, trendLabel := queuePressureTrend(recent15, rate, stalled)

	advisor := QueueAdvisor{
		StoreID:     snapshot.StoreID,
		StoreName:   store.Name,
		GeneratedAt: now.Format(time.RFC3339),
		Current: QueueAdvisorCurrent{
			CalledNo:            snapshot.DisplayCalledNo,
			WaitingGroups:       snapshot.GroupQueuesCount,
			OfficialWaitMinutes: store.Wait,
			StoreStatus:         store.StoreStatus,
			NetTicketStatus:     store.NetTicketStatus,
			OnlineOpen:          snapshot.OnlineOpen,
		},
		Pressure: QueueAdvisorPressure{
			Level:      level,
			Label:      queuePressureLabel(level),
			Score:      queuePressureScore(snapshot.GroupQueuesCount, store.Wait),
			Trend:      trend,
			TrendLabel: trendLabel,
			Reason:     queuePressureReason(level, trend, snapshot.GroupQueuesCount, rate, called15),
		},
		Speed: QueueAdvisorSpeed{
			CalledPerMin15: calledRateOverWindow(all, snapshot.StoreID, now, queueAdvisorWindow15),
			CalledPerMin30: calledRateOverWindow(all, snapshot.StoreID, now, queueAdvisorWindow30),
			CalledPerMin60: rate60,
		},
		SamplingPoints: len(history),
		Warnings:       warnings,
	}

	if targetNo > 0 {
		advisor.Eta = buildQueueAdvisorEta(targetNo, travelMinutes, snapshot, store, rate, now, storeID)
	}
	return advisor, nil
}

func buildQueueAdvisorEta(targetNo, travelMinutes int, snapshot QueueObservation, store QueueLiveStore, rate float64, now time.Time, storeID string) *QueueAdvisorEta {
	var hist *QueueWaitRange
	if m, _ := historicalWaitByBucket(storeID, now); m != nil {
		if r, ok := m[queueTrendBucket(now, 30)]; ok {
			hist = &r
		}
	}
	return computeQueueEta(targetNo, snapshot.DisplayCalledNo, travelMinutes, store.Wait, rate, hist, now)
}

// computeQueueEta 是 ETA 估算的纯函数核心（不读磁盘），便于单测。
func computeQueueEta(targetNo, calledNo, travelMinutes, officialWait int, rate float64, hist *QueueWaitRange, now time.Time) *QueueAdvisorEta {
	remaining := max(0, targetNo-calledNo)
	eta := &QueueAdvisorEta{
		TargetNo:        targetNo,
		RemainingGroups: remaining,
		Risk:            "unknown",
	}
	waitRange, source := estimateWaitRange(remaining, officialWait, rate, hist)
	if remaining <= 0 {
		eta.EstimatedCalledAt = now.Format(time.RFC3339)
		eta.WaitMinutesRange = &QueueWaitRange{Low: 0, High: 0}
		eta.Risk = "low"
		eta.ArrivalSuggestion = "已轮到或即将轮到你，请尽快到店。"
		return eta
	}
	eta.Source = source
	eta.SourceLabel = queueEtaSourceLabel(source)
	eta.SourceNote = queueEtaSourceNote(source)
	if source == "official" {
		eta.Risk = "high"
		eta.ArrivalSuggestion = "当前缺少叫号速度，官方等待只能代表门店排队压力，暂时不能可靠判断你的号码几点叫到。"
		return eta
	}
	if waitRange == nil {
		eta.ArrivalSuggestion = "实时和历史数据都不足，暂时无法预估叫到时间。"
		return eta
	}
	eta.WaitMinutesRange = waitRange
	early := now.Add(time.Duration(waitRange.Low) * time.Minute)
	late := now.Add(time.Duration(waitRange.High) * time.Minute)
	mid := now.Add(time.Duration((waitRange.Low+waitRange.High)/2) * time.Minute)
	eta.EstimatedCalledAt = mid.Format(time.RFC3339)
	eta.EstimatedCalledAtRange = &QueueTimeRange{Early: early.Format(time.RFC3339), Late: late.Format(time.RFC3339)}
	// 建议出发：按偏早叫到时间倒减路程。
	depart := early.Add(-time.Duration(max(0, travelMinutes)) * time.Minute)
	if depart.Before(now) {
		eta.ArrivalSuggestion = "建议现在就出发。"
	} else {
		eta.ArrivalSuggestion = fmt.Sprintf("建议 %s 前后出发。", depart.Format("15:04"))
	}
	eta.Risk = queueEtaRisk(source, officialWait, remaining, rate)
	return eta
}

func queueEtaSourceLabel(source string) string {
	switch source {
	case "recent_speed":
		return "近实时叫号速度"
	case "history":
		return "同门店历史"
	case "official":
		return "官方等待参考"
	default:
		return "数据不足"
	}
}

func queueEtaSourceNote(source string) string {
	switch source {
	case "recent_speed":
		return "按本机近期采样的叫号推进速度估算，适合判断你手里号码的大概叫到时间。"
	case "history":
		return "按同门店同日型历史等待估算，适合作参考，建议配合实时叫号一起看。"
	case "official":
		return "官方等待不等同于到你号码的等待时间，只能说明当前门店压力。"
	default:
		return "缺少叫号速度和历史样本。"
	}
}

func queueEtaRisk(source string, officialWait, remaining int, rate float64) string {
	switch source {
	case "recent_speed":
		if remaining > 200 {
			return "high"
		}
		if remaining > 80 {
			return "medium"
		}
		return "low"
	case "official":
		if officialWait >= 120 {
			return "high"
		}
		if officialWait >= 60 {
			return "medium"
		}
		return "low"
	case "history":
		return "medium"
	default:
		return "unknown"
	}
}

// ---------- 压力曲线 ----------

type QueuePressureCurvePoint struct {
	Time                string   `json:"time"`
	CalledNo            int      `json:"called_no"`
	WaitingGroups       int      `json:"waiting_groups"`
	OfficialWaitMinutes int      `json:"official_wait_minutes"`
	PressureLevel       string   `json:"pressure_level"`
	PressureScore       int      `json:"pressure_score"`
	CalledSpeed15       *float64 `json:"called_speed_15,omitempty"`
	Source              string   `json:"source,omitempty"`
	SampleCount         int      `json:"sample_count,omitempty"`
	Confidence          string   `json:"confidence,omitempty"`
}

type QueuePressureCurve struct {
	StoreID      string                    `json:"store_id"`
	Date         string                    `json:"date"`
	DateType     string                    `json:"date_type,omitempty"`
	DateTypeName string                    `json:"date_type_name,omitempty"`
	GeneratedAt  string                    `json:"generated_at"`
	Source       string                    `json:"source,omitempty"`
	LocalPoints  int                       `json:"local_points"`
	RemotePoints int                       `json:"remote_points"`
	Baseline     QueueBaselineRemoteStatus `json:"baseline"`
	Points       []QueuePressureCurvePoint `json:"points"`
	Message      string                    `json:"message,omitempty"`
}

func buildQueuePressureCurve(ctx context.Context, storeID, date string, now time.Time) QueuePressureCurve {
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}
	date, day := queuePressureCurveDate(date, now)
	holidays, workdays, _ := loadQueueHolidayDates()
	dateType := queueTrendDateType(day, holidays, workdays)
	out := QueuePressureCurve{
		StoreID:      storeID,
		Date:         date,
		DateType:     dateType,
		DateTypeName: queueTrendDateTypeName(dateType),
		GeneratedAt:  now.Format(time.RFC3339),
	}

	localPoints := buildLocalQueuePressureCurvePoints(storeID, date)
	out.LocalPoints = len(localPoints)

	baseline, baselineStatus, baselineErr := loadRemoteQueuePressureBaseline(ctx, storeID, now)
	out.Baseline = baselineStatus
	remotePoints := buildRemoteQueuePressureCurvePoints(storeID, date, dateType, baseline)
	out.RemotePoints = len(remotePoints)

	switch {
	case len(localPoints) >= queuePressureCurveLocalPreferredPoints:
		out.Points = localPoints
		out.Source = "local"
		if len(remotePoints) > 0 {
			out.Message = "当前曲线使用本机实际采样；线上 Turso 基准已连接，本机样本不足时会自动兜底。"
		}
	case len(localPoints) > 0 && len(remotePoints) > 0:
		out.Points = mergeQueuePressureCurvePoints(remotePoints, localPoints)
		out.Source = "mixed"
		out.Message = fmt.Sprintf("本机今天只有 %d 个采样点，已用线上 Turso 基准补全排队压力；带“本机采样”的点按实际数据覆盖，远端基准不等同实时叫号。", len(localPoints))
	case len(remotePoints) > 0:
		out.Points = remotePoints
		out.Source = "remote_baseline"
		out.Message = "本机今天还没有足够采样，当前使用线上 Turso 基准的排队压力；实时叫号判断仍以上方官方当前状态为准。"
	case len(localPoints) > 0:
		out.Points = localPoints
		out.Source = "local"
		if baselineErr != nil {
			out.Message = "线上 Turso 基准暂时不可用，当前只显示本机采样曲线。"
		}
	default:
		out.Source = "none"
		out.Message = "还没有这家店今天的本机采样曲线。开启本机数据收集后会逐步补齐。"
		if baselineErr != nil {
			out.Message += " 线上 Turso 基准暂时不可用。"
		} else if baselineStatus.Configured && !baselineStatus.Used {
			out.Message += " 线上 Turso 基准未返回可用数据。"
		} else if !baselineStatus.Configured {
			out.Message += " 未配置线上 Turso 基准。"
		}
	}
	return out
}

func queuePressureCurveDate(raw string, now time.Time) (string, time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	if day, ok := parseTrendDateParam(raw, loc); ok {
		return day.Format("2006-01-02"), day
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return day.Format("2006-01-02"), day
}

func buildLocalQueuePressureCurvePoints(storeID, date string) []QueuePressureCurvePoint {
	all := loadQueueObservations()
	dayObservations := make([]QueueObservation, 0, len(all))
	for _, o := range all {
		if o.StoreID != storeID {
			continue
		}
		at, ok := parseRFC3339Local(queueObservationCollectedAt(o))
		if !ok || at.Format("2006-01-02") != date {
			continue
		}
		dayObservations = append(dayObservations, o)
	}
	if len(dayObservations) == 0 {
		return nil
	}
	sort.SliceStable(dayObservations, func(i, j int) bool {
		li, _ := parseRFC3339Local(queueObservationCollectedAt(dayObservations[i]))
		lj, _ := parseRFC3339Local(queueObservationCollectedAt(dayObservations[j]))
		return li.Before(lj)
	})
	points := make([]QueuePressureCurvePoint, 0, len(dayObservations))
	for _, o := range dayObservations {
		at, _ := parseRFC3339Local(queueObservationCollectedAt(o))
		level := queuePressureLevel(o.GroupQueuesCount, o.WaitMinutes)
		points = append(points, QueuePressureCurvePoint{
			Time:                at.Format("15:04"),
			CalledNo:            o.DisplayCalledNo,
			WaitingGroups:       o.GroupQueuesCount,
			OfficialWaitMinutes: o.WaitMinutes,
			PressureLevel:       level,
			PressureScore:       queuePressureScore(o.GroupQueuesCount, o.WaitMinutes),
			CalledSpeed15:       calledRateOverWindow(dayObservations, storeID, at, queueAdvisorWindow15),
			Source:              "local",
			SampleCount:         1,
			Confidence:          "live",
		})
	}
	return points
}

type queuePressureCurveRemoteAcc struct {
	samples   int
	calledSum float64
	calledN   int
	groupsSum float64
	groupsN   int
	waitSum   float64
	waitN     int
}

func buildRemoteQueuePressureCurvePoints(storeID, date, dateType string, baseline QueueBaselineExport) []QueuePressureCurvePoint {
	storeID = strings.TrimSpace(storeID)
	if storeID == "" || len(baseline.Rollups) == 0 && len(baseline.Latest) == 0 {
		return nil
	}
	byBucket := map[string]*queuePressureCurveRemoteAcc{}
	for _, rollup := range baseline.Rollups {
		if strconv.Itoa(rollup.StoreID) != storeID {
			continue
		}
		if !queueDashboardRollupDateTypeAllowed(rollup.DateType, dateType, false) || !queueDashboardCalledBucketInRange(rollup.TimeBucket) {
			continue
		}
		if rollup.CalledNoTypical == nil && rollup.QueueGroupsTypical == nil && rollup.WaitTypicalMinutes == nil {
			continue
		}
		acc := byBucket[rollup.TimeBucket]
		if acc == nil {
			acc = &queuePressureCurveRemoteAcc{}
			byBucket[rollup.TimeBucket] = acc
		}
		acc.samples += maxInt(rollup.SampleCount, 1)
		if rollup.CalledNoTypical != nil {
			weight := maxInt(rollup.CalledSampleCount, 1)
			acc.calledSum += *rollup.CalledNoTypical * float64(weight)
			acc.calledN += weight
		}
		if rollup.QueueGroupsTypical != nil {
			weight := maxInt(rollup.SampleCount, 1)
			acc.groupsSum += *rollup.QueueGroupsTypical * float64(weight)
			acc.groupsN += weight
		}
		if rollup.WaitTypicalMinutes != nil {
			weight := maxInt(rollup.SampleCount, 1)
			acc.waitSum += *rollup.WaitTypicalMinutes * float64(weight)
			acc.waitN += weight
		}
	}

	buckets := make([]string, 0, len(byBucket))
	for bucket := range byBucket {
		buckets = append(buckets, bucket)
	}
	sort.Strings(buckets)
	points := make([]QueuePressureCurvePoint, 0, len(buckets)+1)
	for _, bucket := range buckets {
		acc := byBucket[bucket]
		point := QueuePressureCurvePoint{
			Time:        bucket,
			Source:      "remote_baseline",
			SampleCount: acc.samples,
			Confidence:  queueDashboardConfidence(acc.samples),
		}
		if acc.calledN > 0 {
			point.CalledNo = int(math.Round(acc.calledSum / float64(acc.calledN)))
		}
		if acc.groupsN > 0 {
			point.WaitingGroups = int(math.Round(acc.groupsSum / float64(acc.groupsN)))
		}
		if acc.waitN > 0 {
			point.OfficialWaitMinutes = int(math.Round(acc.waitSum / float64(acc.waitN)))
		}
		point.PressureLevel = queuePressureLevel(point.WaitingGroups, point.OfficialWaitMinutes)
		point.PressureScore = queuePressureScore(point.WaitingGroups, point.OfficialWaitMinutes)
		points = append(points, point)
	}

	for _, latest := range baseline.Latest {
		if strconv.Itoa(latest.StoreID) != storeID {
			continue
		}
		at, ok := parseRFC3339Local(latest.CollectedAt)
		if !ok || at.Format("2006-01-02") != date || !queueDashboardCalledTimeInRange(at) {
			continue
		}
		level := queuePressureLevel(latest.GroupQueuesCount, latest.WaitMinutes)
		points = append(points, QueuePressureCurvePoint{
			Time:                at.Format("15:04"),
			CalledNo:            latest.DisplayCalledNo,
			WaitingGroups:       latest.GroupQueuesCount,
			OfficialWaitMinutes: latest.WaitMinutes,
			PressureLevel:       level,
			PressureScore:       queuePressureScore(latest.GroupQueuesCount, latest.WaitMinutes),
			Source:              "remote_latest",
			SampleCount:         1,
			Confidence:          "live",
		})
	}
	return sortAndDedupQueuePressureCurvePoints(points)
}

func mergeQueuePressureCurvePoints(remotePoints, localPoints []QueuePressureCurvePoint) []QueuePressureCurvePoint {
	points := make([]QueuePressureCurvePoint, 0, len(remotePoints)+len(localPoints))
	points = append(points, remotePoints...)
	points = append(points, localPoints...)
	return sortAndDedupQueuePressureCurvePoints(points)
}

func sortAndDedupQueuePressureCurvePoints(points []QueuePressureCurvePoint) []QueuePressureCurvePoint {
	sort.SliceStable(points, func(i, j int) bool {
		mi := queueDashboardBucketMinute(points[i].Time)
		mj := queueDashboardBucketMinute(points[j].Time)
		if mi != mj {
			return mi < mj
		}
		return queuePressureCurveSourceRank(points[i].Source) > queuePressureCurveSourceRank(points[j].Source)
	})
	out := make([]QueuePressureCurvePoint, 0, len(points))
	seen := map[string]bool{}
	for _, point := range points {
		minute := queueDashboardBucketMinute(point.Time)
		if minute < 0 {
			continue
		}
		key := point.Time
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, point)
	}
	return out
}

func queuePressureCurveSourceRank(source string) int {
	switch source {
	case "local":
		return 3
	case "remote_latest":
		return 2
	case "remote_baseline":
		return 1
	default:
		return 0
	}
}

// ---------- 时间互推：取号 <-> 就餐 ----------

// historicalWaitByBucket 取某店今日日期类型的历史等待 P50/P80（按半小时 bucket）。
func historicalWaitByBucket(storeID string, now time.Time) (map[string]QueueWaitRange, string) {
	dt := queueAdvisorDateTypeForToday(now)
	resp := BuildQueueTrends(QueueTrendQuery{StoreIDs: []string{storeID}, DateType: dt, BucketMinutes: 30}, now)
	out := map[string]QueueWaitRange{}
	for _, p := range resp.Series {
		if p.StoreID != storeID || p.WaitP50Minutes == nil {
			continue
		}
		low := int(math.Round(*p.WaitP50Minutes))
		high := low
		if p.WaitP80Minutes != nil {
			high = int(math.Round(*p.WaitP80Minutes))
		}
		if high < low {
			high = low
		}
		out[p.Bucket] = QueueWaitRange{Low: low, High: high}
	}
	return out, queueTrendDateTypeName(dt)
}

func queueAdvisorDateTypeForToday(now time.Time) string {
	holidays, workdays, _ := loadQueueHolidayDates()
	return queueTrendDateType(now, holidays, workdays)
}

type QueuePickupPlan struct {
	StoreID          string          `json:"store_id"`
	Pickup           string          `json:"pickup"`
	WaitMinutesRange *QueueWaitRange `json:"wait_minutes_range,omitempty"`
	MealRange        *QueueTimeRange `json:"meal_range,omitempty"`
	Risk             string          `json:"risk"`
	Basis            string          `json:"basis"`
	Message          string          `json:"message,omitempty"`
}

type QueueMealPlan struct {
	StoreID              string          `json:"store_id"`
	TargetMeal           string          `json:"target_meal"`
	RecommendPickupRange *QueueTimeRange `json:"recommend_pickup_range,omitempty"`
	StablePickup         string          `json:"stable_pickup,omitempty"`
	LatestPickup         string          `json:"latest_pickup,omitempty"`
	WaitMinutesRange     *QueueWaitRange `json:"wait_minutes_range,omitempty"`
	Risk                 string          `json:"risk"`
	Basis                string          `json:"basis"`
	Message              string          `json:"message,omitempty"`
}

// parseHHMM 把 "1210"/"12:10" 解析为今天对应的本地时间。
func parseHHMM(raw string, now time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ":", ""))
	if len(raw) != 4 {
		return time.Time{}, false
	}
	hh, ok1 := atoiStrict(raw[:2])
	mm, ok2 := atoiStrict(raw[2:])
	if !ok1 || !ok2 || hh > 23 || mm > 59 {
		return time.Time{}, false
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location()), true
}

func atoiStrict(s string) (int, bool) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func buildQueuePickupPlan(storeID, pickupRaw string, now time.Time) QueuePickupPlan {
	if now.IsZero() {
		now = time.Now()
	}
	out := QueuePickupPlan{StoreID: storeID, Pickup: compactTrendTime(pickupRaw)}
	pickup, ok := parseHHMM(pickupRaw, now)
	if !ok {
		out.Message = "请提供有效的取号时间，例如 1210 或 12:10。"
		return out
	}
	out.Pickup = pickup.Format("15:04")
	hist, basis := historicalWaitByBucket(storeID, now)
	wr, found := hist[queueTrendBucket(pickup, 30)]
	if !found {
		out.Message = "这家店该时段的历史样本不足，先开启本机数据收集积累几次。"
		out.Basis = basis
		return out
	}
	out.WaitMinutesRange = &wr
	out.MealRange = &QueueTimeRange{
		Early: pickup.Add(time.Duration(wr.Low) * time.Minute).Format("15:04"),
		Late:  pickup.Add(time.Duration(wr.High) * time.Minute).Format("15:04"),
	}
	out.Basis = "同门店同日型历史等待；今日实时压力仅修正风险"
	out.Risk = queuePlanRisk(storeID, now, wr)
	return out
}

func buildQueueMealPlan(storeID, mealRaw string, travelMinutes int, now time.Time) QueueMealPlan {
	if now.IsZero() {
		now = time.Now()
	}
	out := QueueMealPlan{StoreID: storeID, TargetMeal: compactTrendTime(mealRaw)}
	meal, ok := parseHHMM(mealRaw, now)
	if !ok {
		out.Message = "请提供有效的目标就餐时间，例如 1300 或 13:00。"
		return out
	}
	out.TargetMeal = meal.Format("15:04")
	target := meal.Add(-time.Duration(max(0, travelMinutes)) * time.Minute)
	hist, _ := historicalWaitByBucket(storeID, now)
	if len(hist) == 0 {
		out.Message = "这家店历史样本不足，先开启本机数据收集积累几次。"
		return out
	}
	// 枚举候选取号时间（每 10 分钟），预测就餐时间，挑最接近目标的窗口。
	var best, latest time.Time
	var bestWR QueueWaitRange
	bestGap := math.MaxFloat64
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	for t := dayStart; !t.After(meal); t = t.Add(10 * time.Minute) {
		wr, found := hist[queueTrendBucket(t, 30)]
		if !found {
			continue
		}
		predEarly := t.Add(time.Duration(wr.Low) * time.Minute)
		predLate := t.Add(time.Duration(wr.High) * time.Minute)
		// 推荐取号窗口：偏稳就餐（P80）不晚于目标。
		if !predLate.After(target) {
			latest = t
		}
		gap := math.Abs(predEarly.Sub(target).Minutes())
		if gap < bestGap {
			bestGap = gap
			best = t
			bestWR = wr
		}
	}
	if best.IsZero() {
		out.Message = "按历史样本，今天恐怕赶不上这个就餐时间，建议提早取号或换时段。"
		return out
	}
	out.StablePickup = best.Format("15:04")
	out.WaitMinutesRange = &bestWR
	if !latest.IsZero() {
		out.LatestPickup = latest.Format("15:04")
		out.RecommendPickupRange = &QueueTimeRange{Early: best.Format("15:04"), Late: latest.Format("15:04")}
	} else {
		out.RecommendPickupRange = &QueueTimeRange{Early: best.Format("15:04"), Late: best.Format("15:04")}
	}
	out.Basis = "同门店同日型历史等待；今日实时压力仅修正风险"
	out.Risk = queuePlanRisk(storeID, now, bestWR)
	return out
}

// queuePlanRisk 用今日实时排队压力修正历史规划的风险。
func queuePlanRisk(storeID string, now time.Time, wr QueueWaitRange) string {
	all := loadQueueObservations()
	latest := latestQueueObservationsByStore(all)[storeID]
	level := queuePressureLevel(latest.GroupQueuesCount, latest.WaitMinutes)
	switch level {
	case "extreme":
		return "high"
	case "high":
		if wr.High >= 90 {
			return "high"
		}
		return "medium"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		if wr.High >= 90 {
			return "medium"
		}
		return "low"
	}
}
