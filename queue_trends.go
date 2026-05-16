package main

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	queueObservationFile = "queue_observations.jsonl"
	queueSessionFile     = "queue_sessions.jsonl"
	queueStatsFile       = "queue_stats.json"
	queueHolidayFile     = "holidays.json"
	queueDataStaleAfter  = 6 * time.Hour
)

type QueueObservation struct {
	Timestamp       string `json:"ts"`
	StoreID         string `json:"store_id"`
	DisplayCalledNo int    `json:"display_called_no"`
	OnlineOpen      bool   `json:"online_open,omitempty"`
}

type QueueSession struct {
	StoreID                string `json:"store_id"`
	TakenAt                string `json:"taken_at"`
	TicketNo               int    `json:"ticket_no,omitempty"`
	DisplayCalledNoAtTake  int    `json:"display_called_no_at_take,omitempty"`
	CheckedInAt            string `json:"checked_in_at,omitempty"`
	CalledForUserAt        string `json:"called_for_user_at,omitempty"`
	CalledNoWhenUserCalled int    `json:"called_no_when_user_called,omitempty"`
	ActualWaitMinutes      int    `json:"actual_wait_minutes,omitempty"`
	PartySize              int    `json:"party_size,omitempty"`
	TableType              string `json:"table_type,omitempty"`
	ExpiredOrMissed        bool   `json:"expired_or_missed,omitempty"`
	Manual                 bool   `json:"manual,omitempty"`
}

type QueueTrendQuery struct {
	StoreIDs      []string `json:"store_ids"`
	DateType      string   `json:"date_type"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Start         string   `json:"start"`
	End           string   `json:"end"`
	BucketMinutes int      `json:"bucket_minutes"`
}

type QueueTrendResponse struct {
	GeneratedAt string              `json:"generated_at"`
	Filters     QueueTrendQuery     `json:"filters"`
	Summary     QueueTrendSummary   `json:"summary"`
	Series      []QueueTrendPoint   `json:"series"`
	Stores      []QueueTrendStore   `json:"stores"`
	Sampling    QueueSamplingStatus `json:"sampling"`
	Scope       QueueTrendScope     `json:"scope"`
	Warnings    []string            `json:"warnings,omitempty"`
}

type QueueTrendSummary struct {
	ObservationRecords int    `json:"observation_records"`
	SessionRecords     int    `json:"session_records"`
	ActualSamples      int    `json:"actual_samples"`
	GlobalSamples      int    `json:"global_samples"`
	ActualPassedTotal  int    `json:"actual_passed_total"`
	GlobalPassedTotal  int    `json:"global_passed_total"`
	LastObservationAt  string `json:"last_observation_at,omitempty"`
	LastSessionAt      string `json:"last_session_at,omitempty"`
}

type QueueTrendPoint struct {
	StoreID           string   `json:"store_id"`
	StoreName         string   `json:"store_name"`
	DateType          string   `json:"date_type"`
	DateTypeName      string   `json:"date_type_name"`
	Bucket            string   `json:"bucket"`
	ActualPassed      int      `json:"actual_passed"`
	GlobalPassed      int      `json:"global_passed"`
	ActualSamples     int      `json:"actual_samples"`
	GlobalSamples     int      `json:"global_samples"`
	SessionSamples    int      `json:"session_samples"`
	WaitP50Minutes    *float64 `json:"wait_p50_minutes,omitempty"`
	WaitP80Minutes    *float64 `json:"wait_p80_minutes,omitempty"`
	MissedRate        float64  `json:"missed_rate"`
	Confidence        string   `json:"confidence"`
	LastObservationAt string   `json:"last_observation_at,omitempty"`
}

type QueueTrendStore struct {
	StoreID   string `json:"store_id"`
	StoreName string `json:"store_name"`
}

type QueueTrendScope struct {
	Mode       string `json:"mode"`
	StoreCount int    `json:"store_count"`
	Message    string `json:"message"`
}

type QueueSamplingStatus struct {
	Enabled            bool            `json:"enabled"`
	Running            bool            `json:"running"`
	DaemonRunning      bool            `json:"daemon_running"`
	AppAutoStart       bool            `json:"app_auto_start"`
	SystemAutoStart    AutoStartStatus `json:"system_auto_start"`
	AuthOK             bool            `json:"auth_ok"`
	NeedsAuth          bool            `json:"needs_auth"`
	NeedsBackground    bool            `json:"needs_background"`
	NeedsDataRefresh   bool            `json:"needs_data_refresh"`
	PermissionStatus   string          `json:"permission_status"`
	Message            string          `json:"message"`
	LastRunAt          string          `json:"last_run_at,omitempty"`
	LastError          string          `json:"last_error,omitempty"`
	LastDataAt         string          `json:"last_data_at,omitempty"`
	DataStaleHours     int             `json:"data_stale_hours,omitempty"`
	SamplingLogPath    string          `json:"sampling_log_path,omitempty"`
	SamplingConfigPath string          `json:"sampling_config_path,omitempty"`
}

type QueueLocalStat struct {
	StoreID                 string   `json:"store_id"`
	Weekday                 int      `json:"weekday"`
	TimeBucket              string   `json:"time_bucket"`
	TableType               string   `json:"table_type"`
	PartySizeBucket         string   `json:"party_size_bucket"`
	Samples                 int      `json:"samples"`
	WaitP50Minutes          *float64 `json:"wait_p50_minutes,omitempty"`
	WaitP80Minutes          *float64 `json:"wait_p80_minutes,omitempty"`
	CheckinToCallP50Minutes *float64 `json:"checkin_to_call_p50_minutes,omitempty"`
	MissedRate              float64  `json:"missed_rate"`
}

type queueTrendAccumulator struct {
	point          QueueTrendPoint
	waits          []float64
	missed         int
	sessionTotal   int
	lastObservedAt time.Time
}

type queueLocalBucket struct {
	storeID         string
	weekday         int
	timeBucket      string
	tableType       string
	partySizeBucket string
	waits           []float64
	checkinWaits    []float64
	missed          int
	total           int
}

type queueHolidayConfig struct {
	Holidays []string `json:"holidays"`
	Workdays []string `json:"workdays"`
}

func queueObservationPath() string {
	return filepath.Join(appDirPath(), queueObservationFile)
}

func queueSessionPath() string {
	return filepath.Join(appDirPath(), queueSessionFile)
}

func queueStatsPath() string {
	return filepath.Join(appDirPath(), queueStatsFile)
}

func queueHolidayPath() string {
	return filepath.Join(appDirPath(), queueHolidayFile)
}

func BuildQueueTrends(query QueueTrendQuery, now time.Time) QueueTrendResponse {
	if now.IsZero() {
		now = time.Now()
	}
	query = normalizeQueueTrendQuery(query, now)
	sessions := loadQueueSessions()
	observations := loadQueueObservations()
	holidays, workdays, holidayConfigured := loadQueueHolidayDates()
	storeNames := queueTrendStoreNames(query.StoreIDs, sessions, observations)
	storeFilter := stringSet(query.StoreIDs)
	series := map[string]*queueTrendAccumulator{}
	summary := QueueTrendSummary{
		ObservationRecords: len(observations),
		SessionRecords:     len(sessions),
	}

	for _, session := range sessions {
		takenAt, ok := parseRFC3339Local(session.TakenAt)
		if !ok || !queueTrendMatches(query, takenAt, storeFilter, session.StoreID, holidays, workdays) {
			continue
		}
		updateLatest(&summary.LastSessionAt, takenAt)
		dateType := queueTrendDateType(takenAt, holidays, workdays)
		acc := queueTrendAcc(series, session.StoreID, storeNames[session.StoreID], dateType, queueTrendBucket(takenAt, query.BucketMinutes))
		acc.sessionTotal++
		acc.point.SessionSamples++
		if session.ExpiredOrMissed {
			acc.missed++
		}
		if wait, ok := queueSessionWaitMinutes(session); ok {
			acc.waits = append(acc.waits, wait)
		}
		if passed, ok := queueActualPassed(session); ok {
			acc.point.ActualPassed += passed
			acc.point.ActualSamples++
			summary.ActualPassedTotal += passed
			summary.ActualSamples++
		}
	}

	observationsByStore := map[string][]QueueObservation{}
	for _, observation := range observations {
		observationsByStore[strings.TrimSpace(observation.StoreID)] = append(observationsByStore[strings.TrimSpace(observation.StoreID)], observation)
		if at, ok := parseRFC3339Local(observation.Timestamp); ok {
			updateLatest(&summary.LastObservationAt, at)
		}
	}
	for storeID, storeObservations := range observationsByStore {
		sort.Slice(storeObservations, func(i, j int) bool {
			left, lok := parseRFC3339Local(storeObservations[i].Timestamp)
			right, rok := parseRFC3339Local(storeObservations[j].Timestamp)
			if !lok || !rok {
				return storeObservations[i].Timestamp < storeObservations[j].Timestamp
			}
			return left.Before(right)
		})
		for i := 1; i < len(storeObservations); i++ {
			prev := storeObservations[i-1]
			curr := storeObservations[i]
			prevAt, prevOK := parseRFC3339Local(prev.Timestamp)
			currAt, currOK := parseRFC3339Local(curr.Timestamp)
			if !prevOK || !currOK || !sameLocalDate(prevAt, currAt) {
				continue
			}
			if !queueTrendMatches(query, currAt, storeFilter, storeID, holidays, workdays) {
				continue
			}
			diff := curr.DisplayCalledNo - prev.DisplayCalledNo
			if diff <= 0 || diff > 500 {
				continue
			}
			dateType := queueTrendDateType(currAt, holidays, workdays)
			acc := queueTrendAcc(series, storeID, storeNames[storeID], dateType, queueTrendBucket(currAt, query.BucketMinutes))
			acc.point.GlobalPassed += diff
			acc.point.GlobalSamples++
			if currAt.After(acc.lastObservedAt) {
				acc.lastObservedAt = currAt
			}
			summary.GlobalPassedTotal += diff
			summary.GlobalSamples++
		}
	}

	points := finalizeQueueTrendPoints(series)
	warnings := queueTrendWarnings(query, holidayConfigured, summary)
	stores := queueTrendStores(storeNames, query.StoreIDs, points)
	scope := QueueTrendScope{
		Mode:       "local",
		StoreCount: len(stores),
		Message:    "当前只分析本机已捕获、已选择或本地文件里出现过的门店。没有稳定门店 ID 列表时，不能保证一个人自动覆盖全国或某个城市的全部门店。",
	}
	return QueueTrendResponse{
		GeneratedAt: now.Format(time.RFC3339),
		Filters:     query,
		Summary:     summary,
		Series:      points,
		Stores:      stores,
		Sampling:    buildQueueSamplingStatus(now, summary),
		Scope:       scope,
		Warnings:    warnings,
	}
}

func BuildQueueLocalStats(sessions []QueueSession, minSamples int) ([]QueueLocalStat, int) {
	if minSamples <= 0 {
		minSamples = 1
	}
	buckets := map[string]*queueLocalBucket{}
	usable := 0
	for _, session := range sessions {
		bucket, ok := queueSessionBucket(session)
		if !ok {
			continue
		}
		usable++
		key := strings.Join([]string{bucket.storeID, strconv.Itoa(bucket.weekday), bucket.timeBucket, bucket.tableType, bucket.partySizeBucket}, "|")
		acc := buckets[key]
		if acc == nil {
			acc = &bucket
			buckets[key] = acc
		}
		acc.total++
		if session.ExpiredOrMissed {
			acc.missed++
		}
		if wait, ok := queueSessionWaitMinutes(session); ok {
			acc.waits = append(acc.waits, wait)
		}
		if wait, ok := queueSessionCheckinToCallMinutes(session); ok {
			acc.checkinWaits = append(acc.checkinWaits, wait)
		}
	}

	stats := make([]QueueLocalStat, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.total < minSamples {
			continue
		}
		stat := QueueLocalStat{
			StoreID:         bucket.storeID,
			Weekday:         bucket.weekday,
			TimeBucket:      bucket.timeBucket,
			TableType:       bucket.tableType,
			PartySizeBucket: bucket.partySizeBucket,
			Samples:         bucket.total,
			MissedRate:      float64(bucket.missed) / float64(bucket.total),
		}
		stat.WaitP50Minutes = floatPtr(queueQuantile(bucket.waits, 0.50))
		stat.WaitP80Minutes = floatPtr(queueQuantile(bucket.waits, 0.80))
		stat.CheckinToCallP50Minutes = floatPtr(queueQuantile(bucket.checkinWaits, 0.50))
		stats = append(stats, stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		a, b := stats[i], stats[j]
		if a.StoreID != b.StoreID {
			return a.StoreID < b.StoreID
		}
		if a.Weekday != b.Weekday {
			return a.Weekday < b.Weekday
		}
		if a.TimeBucket != b.TimeBucket {
			return a.TimeBucket < b.TimeBucket
		}
		if a.TableType != b.TableType {
			return a.TableType < b.TableType
		}
		return a.PartySizeBucket < b.PartySizeBucket
	})
	return stats, usable
}

func normalizeQueueTrendQuery(query QueueTrendQuery, now time.Time) QueueTrendQuery {
	query.StoreIDs = uniqueNonEmptyStrings(query.StoreIDs)
	query.DateType = strings.ToLower(strings.TrimSpace(query.DateType))
	if query.DateType == "" {
		query.DateType = "all"
	}
	switch query.DateType {
	case "all", "weekday", "weekend", "holiday":
	default:
		query.DateType = "all"
	}
	if query.BucketMinutes != 60 {
		query.BucketMinutes = 30
	}
	from, ok := parseTrendDateParam(query.From, now.Location())
	if !ok {
		from = beginningOfDay(now).AddDate(0, 0, -14)
	}
	to, ok := parseTrendDateParam(query.To, now.Location())
	if !ok {
		to = beginningOfDay(now)
	}
	if to.Before(from) {
		from, to = to, from
	}
	query.From = from.Format("2006-01-02")
	query.To = to.Format("2006-01-02")
	if parseTimeSeconds(compactTrendTime(query.Start)) < 0 {
		query.Start = "10:00"
	} else {
		query.Start = displayTrendTime(query.Start)
	}
	if parseTimeSeconds(compactTrendTime(query.End)) < 0 {
		query.End = "22:00"
	} else {
		query.End = displayTrendTime(query.End)
	}
	return query
}

func queueTrendMatches(query QueueTrendQuery, at time.Time, storeFilter map[string]bool, storeID string, holidays, workdays map[string]bool) bool {
	storeID = strings.TrimSpace(storeID)
	if storeID == "" {
		return false
	}
	if len(storeFilter) > 0 && !storeFilter[storeID] {
		return false
	}
	from, _ := parseTrendDateParam(query.From, at.Location())
	to, _ := parseTrendDateParam(query.To, at.Location())
	day := beginningOfDay(at)
	if day.Before(from) || day.After(to) {
		return false
	}
	dateType := queueTrendDateType(at, holidays, workdays)
	if query.DateType != "all" && query.DateType != dateType {
		return false
	}
	return queueTrendTimeInRange(at, query.Start, query.End)
}

func queueTrendTimeInRange(at time.Time, startRaw, endRaw string) bool {
	start := parseTimeSeconds(compactTrendTime(startRaw))
	end := parseTimeSeconds(compactTrendTime(endRaw))
	if start < 0 || end < 0 || start == end {
		return true
	}
	current := at.Hour()*3600 + at.Minute()*60 + at.Second()
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func queueTrendAcc(series map[string]*queueTrendAccumulator, storeID, storeName, dateType, bucket string) *queueTrendAccumulator {
	key := strings.Join([]string{storeID, dateType, bucket}, "|")
	acc := series[key]
	if acc == nil {
		if strings.TrimSpace(storeName) == "" {
			storeName = storeID
		}
		acc = &queueTrendAccumulator{
			point: QueueTrendPoint{
				StoreID:      storeID,
				StoreName:    storeName,
				DateType:     dateType,
				DateTypeName: queueTrendDateTypeName(dateType),
				Bucket:       bucket,
			},
		}
		series[key] = acc
	}
	return acc
}

func finalizeQueueTrendPoints(series map[string]*queueTrendAccumulator) []QueueTrendPoint {
	points := make([]QueueTrendPoint, 0, len(series))
	for _, acc := range series {
		point := acc.point
		point.WaitP50Minutes = floatPtr(queueQuantile(acc.waits, 0.50))
		point.WaitP80Minutes = floatPtr(queueQuantile(acc.waits, 0.80))
		if acc.sessionTotal > 0 {
			point.MissedRate = float64(acc.missed) / float64(acc.sessionTotal)
		}
		point.Confidence = queueTrendConfidence(point)
		if !acc.lastObservedAt.IsZero() {
			point.LastObservationAt = acc.lastObservedAt.Format(time.RFC3339)
		}
		points = append(points, point)
	}
	sort.Slice(points, func(i, j int) bool {
		a, b := points[i], points[j]
		if a.Bucket != b.Bucket {
			return a.Bucket < b.Bucket
		}
		if a.DateType != b.DateType {
			return queueTrendDateTypeRank(a.DateType) < queueTrendDateTypeRank(b.DateType)
		}
		return a.StoreID < b.StoreID
	})
	return points
}

func queueTrendConfidence(point QueueTrendPoint) string {
	samples := point.ActualSamples + point.GlobalSamples
	switch {
	case samples >= 12:
		return "high"
	case samples >= 4:
		return "medium"
	case samples > 0:
		return "low"
	default:
		return "none"
	}
}

func queueTrendWarnings(query QueueTrendQuery, holidayConfigured bool, summary QueueTrendSummary) []string {
	warnings := []string{}
	if !holidayConfigured {
		warnings = append(warnings, "未配置本地节假日表，当前只能按自然工作日/周末归类；节假日筛选会没有独立数据。")
	}
	if query.DateType == "holiday" && !holidayConfigured {
		warnings = append(warnings, "要启用节假日趋势，可在 ~/.sushiro/holidays.json 写入 holidays/workdays 日期列表。")
	}
	if summary.ObservationRecords == 0 && summary.SessionRecords == 0 {
		warnings = append(warnings, "暂无本地排队数据。需要保持采样运行，或在实际取号后记录真实叫号。")
	}
	if summary.GlobalSamples == 0 {
		warnings = append(warnings, "暂无连续公开叫号快照，因此全局过号数暂时为空。")
	}
	if summary.ActualSamples == 0 {
		warnings = append(warnings, "暂无确认叫到自己的取号记录，因此实际过号数暂时为空。")
	}
	return warnings
}

func buildQueueSamplingStatus(now time.Time, summary QueueTrendSummary) QueueSamplingStatus {
	cfg := LoadSamplingConfig()
	state := sampler.GetState()
	systemAuto := SamplingAutoStartStatus()
	status := QueueSamplingStatus{
		Enabled:            cfg.Enabled,
		Running:            state.Running,
		DaemonRunning:      isSamplingDaemonRunning(),
		AppAutoStart:       cfg.AutoStart,
		SystemAutoStart:    systemAuto,
		LastRunAt:          state.LastRunAt,
		LastError:          state.LastError,
		SamplingLogPath:    samplingLogPath(),
		SamplingConfigPath: samplingConfigPath(),
	}
	if holder, ok := processLockHolder(samplingLockFileName); ok && holder > 0 {
		status.DaemonRunning = true
	}
	tokens, err := loadLocalConfig()
	if err == nil {
		err = tokens.validateForQuery()
	}
	status.AuthOK = err == nil
	status.NeedsAuth = err != nil

	lastDataAt := latestRFC3339(summary.LastObservationAt, summary.LastSessionAt)
	if !lastDataAt.IsZero() {
		status.LastDataAt = lastDataAt.Format(time.RFC3339)
		age := now.Sub(lastDataAt)
		if age > queueDataStaleAfter {
			status.NeedsDataRefresh = true
			status.DataStaleHours = int(math.Round(age.Hours()))
		}
	} else {
		status.NeedsDataRefresh = true
	}
	status.NeedsBackground = !status.Running && !status.DaemonRunning && !systemAuto.Enabled

	switch {
	case status.NeedsAuth:
		status.PermissionStatus = "needs_auth"
		status.Message = "认证参数不可用或已过期，请重新获取认证后再采样。"
	case status.NeedsBackground:
		status.PermissionStatus = "needs_background"
		status.Message = "未启用常驻采样。排队趋势依赖本机持续记录，建议启用系统开机自启动。"
	case status.NeedsDataRefresh:
		status.PermissionStatus = "needs_update"
		if status.LastDataAt == "" {
			status.Message = "还没有本地排队数据，请启动采样或完成一次真实取号记录。"
		} else {
			status.Message = "本地排队数据较旧，建议重新采样。"
		}
	default:
		status.PermissionStatus = "ok"
		status.Message = "本地采样状态正常。"
	}
	if state.LastError != "" && strings.Contains(strings.ToLower(state.LastError), "认证") {
		status.PermissionStatus = "needs_auth"
		status.NeedsAuth = true
		status.Message = "最近采样提示认证异常，请重新获取认证。"
	}
	return status
}

func queueActualPassed(session QueueSession) (int, bool) {
	if session.CalledNoWhenUserCalled <= 0 || session.DisplayCalledNoAtTake <= 0 {
		return 0, false
	}
	passed := session.CalledNoWhenUserCalled - session.DisplayCalledNoAtTake
	if passed < 0 || passed > 1000 {
		return 0, false
	}
	return passed, true
}

func queueSessionBucket(session QueueSession) (queueLocalBucket, bool) {
	storeID := strings.TrimSpace(session.StoreID)
	if storeID == "" {
		return queueLocalBucket{}, false
	}
	takenAt, ok := parseRFC3339Local(session.TakenAt)
	if !ok {
		return queueLocalBucket{}, false
	}
	if _, ok := queueSessionWaitMinutes(session); !ok && !session.ExpiredOrMissed {
		return queueLocalBucket{}, false
	}
	return queueLocalBucket{
		storeID:         storeID,
		weekday:         isoWeekday(takenAt.Weekday()),
		timeBucket:      halfHourBucket(takenAt),
		tableType:       normalizeQueueTableType(session.TableType),
		partySizeBucket: partySizeBucket(session.PartySize),
	}, true
}

func queueSessionWaitMinutes(session QueueSession) (float64, bool) {
	if session.ActualWaitMinutes > 0 {
		return float64(session.ActualWaitMinutes), true
	}
	takenAt, ok := parseRFC3339Local(session.TakenAt)
	if !ok {
		return 0, false
	}
	calledAt, ok := parseRFC3339Local(session.CalledForUserAt)
	if !ok || calledAt.Before(takenAt) {
		return 0, false
	}
	return calledAt.Sub(takenAt).Minutes(), true
}

func queueSessionCheckinToCallMinutes(session QueueSession) (float64, bool) {
	checkedInAt, ok := parseRFC3339Local(session.CheckedInAt)
	if !ok {
		return 0, false
	}
	calledAt, ok := parseRFC3339Local(session.CalledForUserAt)
	if !ok || calledAt.Before(checkedInAt) {
		return 0, false
	}
	return calledAt.Sub(checkedInAt).Minutes(), true
}

func loadQueueSessions() []QueueSession {
	f, err := os.Open(queueSessionPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := []QueueSession{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var session QueueSession
		if json.Unmarshal([]byte(line), &session) == nil {
			out = append(out, session)
		}
	}
	return out
}

func loadQueueObservations() []QueueObservation {
	f, err := os.Open(queueObservationPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := []QueueObservation{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var observation QueueObservation
		if json.Unmarshal([]byte(line), &observation) == nil {
			out = append(out, observation)
		}
	}
	return out
}

func loadQueueHolidayDates() (map[string]bool, map[string]bool, bool) {
	data, err := os.ReadFile(queueHolidayPath())
	if err != nil {
		return map[string]bool{}, map[string]bool{}, false
	}
	var cfg queueHolidayConfig
	if json.Unmarshal(data, &cfg) != nil {
		return map[string]bool{}, map[string]bool{}, false
	}
	holidays := normalizedDateSet(cfg.Holidays)
	workdays := normalizedDateSet(cfg.Workdays)
	return holidays, workdays, len(holidays) > 0 || len(workdays) > 0
}

func normalizedDateSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		if day, ok := parseTrendDateParam(value, time.Local); ok {
			out[day.Format("2006-01-02")] = true
		}
	}
	return out
}

func queueTrendStoreNames(queryStores []string, sessions []QueueSession, observations []QueueObservation) map[string]string {
	names := map[string]string{}
	reg := GetStoreRegistry()
	add := func(storeID string) {
		storeID = strings.TrimSpace(storeID)
		if storeID == "" {
			return
		}
		if _, ok := names[storeID]; !ok {
			names[storeID] = reg.DisplayName(storeID, storeID)
		}
	}
	for _, storeID := range queryStores {
		add(storeID)
	}
	for _, session := range sessions {
		add(session.StoreID)
	}
	for _, observation := range observations {
		add(observation.StoreID)
	}
	return names
}

func queueTrendStores(names map[string]string, queryStores []string, points []QueueTrendPoint) []QueueTrendStore {
	seen := map[string]bool{}
	ids := []string{}
	for _, storeID := range queryStores {
		if storeID != "" && !seen[storeID] {
			seen[storeID] = true
			ids = append(ids, storeID)
		}
	}
	for _, point := range points {
		if point.StoreID != "" && !seen[point.StoreID] {
			seen[point.StoreID] = true
			ids = append(ids, point.StoreID)
		}
	}
	sort.Strings(ids)
	out := make([]QueueTrendStore, 0, len(ids))
	for _, storeID := range ids {
		name := names[storeID]
		if name == "" {
			name = storeID
		}
		out = append(out, QueueTrendStore{StoreID: storeID, StoreName: name})
	}
	return out
}

func queueTrendDateType(at time.Time, holidays, workdays map[string]bool) string {
	key := at.Format("2006-01-02")
	if workdays[key] {
		return "weekday"
	}
	if holidays[key] {
		return "holiday"
	}
	if at.Weekday() == time.Saturday || at.Weekday() == time.Sunday {
		return "weekend"
	}
	return "weekday"
}

func queueTrendDateTypeName(dateType string) string {
	switch dateType {
	case "weekday":
		return "工作日"
	case "weekend":
		return "周末"
	case "holiday":
		return "节假日"
	default:
		return "全部"
	}
}

func queueTrendDateTypeRank(dateType string) int {
	switch dateType {
	case "weekday":
		return 1
	case "weekend":
		return 2
	case "holiday":
		return 3
	default:
		return 9
	}
}

func queueTrendBucket(at time.Time, minutes int) string {
	if minutes == 60 {
		return at.Format("15:00")
	}
	return halfHourBucket(at)
}

func halfHourBucket(t time.Time) string {
	minute := 0
	if t.Minute() >= 30 {
		minute = 30
	}
	return t.Format("15:") + twoDigits(minute)
}

func partySizeBucket(size int) string {
	switch {
	case size <= 0:
		return "unknown"
	case size <= 2:
		return "1-2"
	case size <= 4:
		return "3-4"
	default:
		return "5+"
	}
}

func normalizeQueueTableType(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	switch v {
	case "T", "TABLE":
		return "T"
	case "C", "COUNTER":
		return "C"
	default:
		return "unknown"
	}
}

func queueQuantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	out := append([]float64(nil), values...)
	sort.Float64s(out)
	if len(out) == 1 {
		return out[0]
	}
	pos := q * float64(len(out)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return out[lo]
	}
	weight := pos - float64(lo)
	return out[lo]*(1-weight) + out[hi]*weight
}

func floatPtr(v float64) *float64 {
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

func parseRFC3339Local(raw string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func parseTrendDateParam(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if loc == nil {
		loc = time.Local
	}
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if day, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return beginningOfDay(day), true
		}
	}
	return time.Time{}, false
}

func compactTrendTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) == 5 && raw[2] == ':' {
		return raw[:2] + raw[3:] + "00"
	}
	if len(raw) == 4 {
		return raw + "00"
	}
	return strings.ReplaceAll(raw, ":", "")
}

func displayTrendTime(raw string) string {
	raw = compactTrendTime(raw)
	if len(raw) >= 4 {
		return raw[:2] + ":" + raw[2:4]
	}
	return raw
}

func sameLocalDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func updateLatest(target *string, t time.Time) {
	if t.IsZero() {
		return
	}
	current, ok := parseRFC3339Local(*target)
	if !ok || t.After(current) {
		*target = t.Format(time.RFC3339)
	}
}

func latestRFC3339(values ...string) time.Time {
	var latest time.Time
	for _, value := range values {
		if t, ok := parseRFC3339Local(value); ok && t.After(latest) {
			latest = t
		}
	}
	return latest
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
