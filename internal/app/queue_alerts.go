package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/notify"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	queueAlertConfigFile = "queue_alerts.json"
	queueAlertStateFile  = "queue_alert_state.json"

	queueAlertWaitBelow   = "wait_below"   // 预估等待低于阈值时提醒（该去取号了）
	queueAlertCalledReach = "called_reach" // 叫号接近我手里的号时提醒

	// wait_below 的迟滞缓冲：等待回升超过阈值+该值才会重新武装，避免临界点反复推送。
	queueAlertWaitHysteresis = 15
)

// QueueAlertRule 是一条叫号提醒规则。
type QueueAlertRule struct {
	StoreID     string `json:"store_id"`
	StoreName   string `json:"store_name,omitempty"`
	Label       string `json:"label,omitempty"`
	Type        string `json:"type"`
	WaitMinutes int    `json:"wait_minutes,omitempty"` // wait_below：预估等待 ≤ 此值时提醒
	TargetNo    int    `json:"target_no,omitempty"`    // called_reach：我手里的号
	LeadGroups  int    `json:"lead_groups,omitempty"`  // called_reach：提前多少号提醒（兼容旧配置）
	NotifyAtNo  int    `json:"notify_at_no,omitempty"` // called_reach：叫到/超过这个号时提醒
	NotifyAtNos []int  `json:"notify_at_nos,omitempty"`
	TravelMin   int    `json:"travel_minutes,omitempty"`
	Template    string `json:"template,omitempty"`
	Enabled     bool   `json:"enabled"`
}

func (r QueueAlertRule) key() string {
	if r.Type == queueAlertCalledReach {
		return fmt.Sprintf("%s|%s|%d|%d|%d", r.StoreID, r.Type, r.WaitMinutes, r.TargetNo, r.calledReachThreshold())
	}
	return fmt.Sprintf("%s|%s|%d|%d", r.StoreID, r.Type, r.WaitMinutes, r.TargetNo)
}

func (r QueueAlertRule) calledReachThreshold() int {
	if r.NotifyAtNo > 0 {
		return r.NotifyAtNo
	}
	if r.TargetNo > 0 {
		return r.TargetNo - r.LeadGroups
	}
	return 0
}

type QueueAlertConfig struct {
	Rules []QueueAlertRule `json:"rules"`
}

// queueAlertRuleState 是单条规则的去重状态。
type queueAlertRuleState struct {
	Armed            bool   `json:"armed"`              // wait_below 是否已武装（等待曾高于阈值）
	FiredAt          string `json:"fired_at"`           // 上次推送时间
	FiredOnce        bool   `json:"fired_once"`         // called_reach 是否已推送过
	LastSeenCalledNo int    `json:"last_seen_called_no"` // called_reach：上一轮观测到的叫号，用于检测回退
}

var queueAlertMu sync.Mutex

func queueAlertConfigPath() string { return filepath.Join(AppDirPath(), queueAlertConfigFile) }
func queueAlertStatePath() string  { return filepath.Join(AppDirPath(), queueAlertStateFile) }

func LoadQueueAlertConfig() QueueAlertConfig {
	data, err := os.ReadFile(queueAlertConfigPath())
	if err != nil {
		return QueueAlertConfig{}
	}
	var cfg QueueAlertConfig
	if json.Unmarshal(data, &cfg) != nil {
		return QueueAlertConfig{}
	}
	return normalizeQueueAlertConfig(cfg)
}

func SaveQueueAlertConfig(cfg QueueAlertConfig) error {
	queueAlertMu.Lock()
	defer queueAlertMu.Unlock()
	return saveQueueAlertConfigLocked(cfg)
}

func saveQueueAlertConfigLocked(cfg QueueAlertConfig) error {
	os.MkdirAll(AppDirPath(), 0o755)
	cfg = normalizeQueueAlertConfig(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(queueAlertConfigPath(), data, 0o600); err != nil {
		return err
	}
	pruneQueueAlertStateLocked(cfg)
	return nil
}

func pruneQueueAlertStateLocked(cfg QueueAlertConfig) {
	state := loadQueueAlertState()
	if len(state) == 0 {
		return
	}
	allowed := map[string]bool{}
	for _, rule := range cfg.Rules {
		allowed[rule.key()] = true
	}
	changed := false
	for key := range state {
		if !allowed[key] {
			delete(state, key)
			changed = true
		}
	}
	if changed {
		saveQueueAlertState(state)
	}
}

func normalizeQueueAlertConfig(cfg QueueAlertConfig) QueueAlertConfig {
	out := QueueAlertConfig{Rules: make([]QueueAlertRule, 0, len(cfg.Rules))}
	seen := map[string]int{}
	for _, rule := range cfg.Rules {
		rules := normalizeQueueAlertRule(rule)
		for _, normalized := range rules {
			key := normalized.key()
			if key == "" {
				continue
			}
			if idx, ok := seen[key]; ok {
				out.Rules[idx] = normalized
				continue
			}
			seen[key] = len(out.Rules)
			out.Rules = append(out.Rules, normalized)
		}
	}
	return out
}

func normalizeQueueAlertRule(rule QueueAlertRule) []QueueAlertRule {
	rule.StoreID = strings.TrimSpace(rule.StoreID)
	rule.StoreName = strings.TrimSpace(rule.StoreName)
	rule.Label = strings.TrimSpace(rule.Label)
	rule.Type = strings.TrimSpace(rule.Type)
	rule.Template = strings.TrimSpace(rule.Template)
	if rule.TravelMin < 0 {
		rule.TravelMin = 0
	}
	if rule.StoreID == "" || rule.Type == "" {
		return nil
	}
	switch rule.Type {
	case queueAlertCalledReach:
		if rule.TargetNo <= 0 {
			return nil
		}
		nos := positiveUniqueInts(rule.NotifyAtNos)
		if rule.NotifyAtNo > 0 {
			nos = appendUniqueInt(nos, rule.NotifyAtNo)
		}
		if len(nos) == 0 {
			threshold := rule.calledReachThreshold()
			if threshold > 0 {
				nos = append(nos, threshold)
			}
		}
		out := make([]QueueAlertRule, 0, len(nos))
		for _, no := range nos {
			normalized := rule
			normalized.NotifyAtNos = nil
			normalized.NotifyAtNo = no
			normalized.LeadGroups = max(0, normalized.TargetNo-no)
			out = append(out, normalized)
		}
		return out
	case queueAlertWaitBelow:
		if rule.WaitMinutes <= 0 {
			return nil
		}
		rule.NotifyAtNos = nil
		rule.NotifyAtNo = 0
		return []QueueAlertRule{rule}
	default:
		return nil
	}
}

func positiveUniqueInts(values []int) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 {
			out = appendUniqueInt(out, value)
		}
	}
	return out
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func loadQueueAlertState() map[string]queueAlertRuleState {
	out := map[string]queueAlertRuleState{}
	data, err := os.ReadFile(queueAlertStatePath())
	if err != nil {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

// saveQueueAlertState 落盘提醒已触发状态。返回 error：写盘失败（磁盘满/权限/目录被清）时
// 调用方据此决定是否回滚内存 state——否则 FiredOnce 没落盘但通知已发，下一轮采样重读会
// 把同一条规则再触发一次（重复推送）。改用原子写避免并发读读到半截 JSON。
func saveQueueAlertState(state map[string]queueAlertRuleState) error {
	os.MkdirAll(AppDirPath(), 0o755)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(queueAlertStatePath(), data, 0o600)
}

// evaluateQueueAlerts 在每轮采样写入一条排队观测后调用，评估该门店的提醒规则并推送。
// obs 是 getStoreById 解析出的观测（含当前叫号/预估等待/在等组数）。
//
// 顺序约定：必须先把 state 成功落盘（FiredOnce/Armed 写入磁盘），再发送通知。
// 否则写盘失败但通知已发，下一轮采样重读无 FiredOnce 的 state 会把同一条规则再触发一次（重复推送）。
// 不再「物理删除已触发规则」：用户原始的 notify_at_nos:[1000,1025,1050] 数组配置会被破坏，
// 去重完全交给 state 的 FiredOnce 字段。
func evaluateQueueAlerts(ctx context.Context, obs QueueObservation, storeName string) {
	storeID := strings.TrimSpace(obs.StoreID)

	notifications := func() []queueAlertNotification {
		queueAlertMu.Lock()
		defer queueAlertMu.Unlock()
		cfg := LoadQueueAlertConfig()
		if len(cfg.Rules) == 0 {
			return nil
		}
		state := loadQueueAlertState()
		changed := false
		out := []queueAlertNotification{}

		for _, rule := range cfg.Rules {
			if !rule.Enabled || rule.StoreID != storeID {
				continue
			}
			title, body, fire := queueAlertEvaluateRule(rule, obs, state)
			if !fire {
				continue
			}
			changed = true
			name := strings.TrimSpace(rule.StoreName)
			if name == "" {
				name = strings.TrimSpace(storeName)
			}
			if name == "" {
				name = storeID
			}
			out = append(out, queueAlertNotification{Title: title, Content: name + "：" + body})
		}
		if changed {
			// 先落盘 state：失败则丢弃本次通知（不更新内存 state、不发通知），
			// 保证「已发通知」与「FiredOnce 已持久化」一致，杜绝写盘失败导致的重复推送。
			if err := saveQueueAlertState(state); err != nil {
				LogMessage(time.Now(), "[排队提醒] 状态落盘失败，本次提醒未发送以防重复: "+err.Error())
				return nil
			}
		}
		return out
	}()

	for _, item := range notifications {
		sendQueueAlert(ctx, item.Title, item.Content)
	}
}

type queueAlertNotification struct {
	Title   string
	Content string
}

// queueAlertEvaluateRule 评估单条规则，更新 state，返回是否需要推送及内容。
func queueAlertEvaluateRule(rule QueueAlertRule, obs QueueObservation, state map[string]queueAlertRuleState) (string, string, bool) {
	key := rule.key()
	st := state[key]

	switch rule.Type {
	case queueAlertWaitBelow:
		wait := obs.WaitMinutes
		if wait > rule.WaitMinutes+queueAlertWaitHysteresis {
			// 等待较高，武装规则，等它落下来再提醒。
			if !st.Armed {
				st.Armed = true
				state[key] = st
			}
			return "", "", false
		}
		if wait <= rule.WaitMinutes && st.Armed {
			st.Armed = false
			st.FiredAt = time.Now().Format(time.RFC3339)
			state[key] = st
			return "🍣 可以去取号了",
				fmt.Sprintf("预计等待已降到约 %d 分钟（阈值 %d），在等 %d 桌。", wait, rule.WaitMinutes, obs.GroupQueuesCount),
				true
		}
		return "", "", false

	case queueAlertCalledReach:
		calledNo := obs.DisplayCalledNo
		if st.FiredOnce || calledNo <= 0 {
			return "", "", false
		}
		// M4 回退防护：寿司郎叫号系统偶发重置/跨日导致 calledNo 回退。记录上一轮观测值，
		// 若本次明显小于上一次（回退），跳过本轮评估，避免误触发。只前进、不后退。
		if st.LastSeenCalledNo > 0 && calledNo < st.LastSeenCalledNo-50 {
			return "", "", false
		}
		st.LastSeenCalledNo = calledNo
		state[key] = st

		threshold := rule.calledReachThreshold()
		if threshold > 0 && calledNo >= threshold {
			st.FiredOnce = true
			st.FiredAt = time.Now().Format(time.RFC3339)
			state[key] = st
			label := queueAlertLabel(rule)
			prefix := ""
			if label != "" {
				prefix = "【" + label + "】"
			}
			travel := ""
			if rule.TravelMin > 0 {
				travel = fmt.Sprintf(" 你填写路程约 %d 分钟，建议现在出发或尽快回店。", rule.TravelMin)
			}
			// M5 过号文案：当前叫号已超过用户手里的号（过号），不再说「还差 0 桌」误导。
			if calledNo > rule.TargetNo {
				return "⚠️ 可能已经过号",
					fmt.Sprintf("%s当前叫号 %d，已超过你的号码 %d。可能已过号，请尽快回店确认。%s", prefix, calledNo, rule.TargetNo, travel),
					true
			}
			return "🔔 快叫到你了",
				fmt.Sprintf("%s当前叫号 %d，已达到提醒点 %d；号码 %d，还差 %d 桌。%s", prefix, calledNo, threshold, rule.TargetNo, max(0, rule.TargetNo-calledNo), travel),
				true
		}
		return "", "", false
	}
	return "", "", false
}

func queueAlertLabel(rule QueueAlertRule) string {
	label := strings.TrimSpace(rule.Label)
	if label != "" {
		return label
	}
	if rule.TargetNo > 0 {
		return fmt.Sprintf("%d号", rule.TargetNo)
	}
	return ""
}

func sendQueueAlert(ctx context.Context, title, content string) {
	LogMessage(time.Now(), fmt.Sprintf("[排队提醒] %s — %s", title, content))
	BuildNotifierFromConfig().Send(ctx, title, content)
}
