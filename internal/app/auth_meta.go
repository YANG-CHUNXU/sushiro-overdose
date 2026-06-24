package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// auth_meta.go 记录凭证的「生命周期元数据」——和凭证本体（config.json）分开存，
// 这样既不动 CapturedTokens 的磁盘格式，也不掺进它的锁。
//
// 用途（都是被动记录，绝不后台轮询，避免把手机会话顶掉）：
//   - 凭证年龄 captured_at → UI 常驻「已用 N 天」
//   - 历史寿命中位数 → 在接近历史寿命时提前推「柔性提醒」，而不是等硬失效打断
//   - 上次捕获方式 capture_method → 失效后一键沿用同方式续期
//
// 和 auth_health.go 的关系：auth_health 管「ok/stale」三态机；本文件管「年龄/寿命/方式」。
// markAuthStale 跃迁时回填一段寿命样本；markAuthHealthy 时顺手做一次柔性提醒判断。

const authMetaFile = "auth_meta.json"

const (
	captureMethodPCWechat    = "pc_wechat"
	captureMethodMobileProxy = "mobile_proxy"
	captureMethodImport      = "import"
)

// authLifespanHistoryMax 限制保留的寿命样本数，滚动丢弃最旧的，避免无界增长。
const authLifespanHistoryMax = 12

// authSoftWarnFraction：年龄达到「历史寿命中位数 × 该比例」时推一次柔性提醒。
const authSoftWarnFraction = 0.8

// authMetaMinLifespanSamples：少于这么多样本就不做寿命预测（中位数不可信）。
const authMetaMinLifespanSamples = 2

type authMetaState struct {
	CapturedAt    time.Time `json:"captured_at,omitempty"`
	CaptureMethod string    `json:"capture_method,omitempty"`
	LastStaleAt   time.Time `json:"last_stale_at,omitempty"`
	LifespanHours []float64 `json:"lifespan_hours,omitempty"` // 每段「捕获→失效」的小时数
	SoftWarned    bool      `json:"soft_warned,omitempty"`    // 本捕获周期内是否已推过柔性提醒
}

var (
	authMetaMu    sync.Mutex
	authMetaCache *authMetaState // nil 表示还没加载；加载后即为权威内存副本
)

func authMetaPath() string {
	return filepath.Join(AppDirPath(), authMetaFile)
}

// loadAuthMetaLocked 读盘到缓存（caller 必须持 authMetaMu）。读不到/坏文件 → 空状态。
func loadAuthMetaLocked() *authMetaState {
	if authMetaCache != nil {
		return authMetaCache
	}
	st := &authMetaState{}
	if raw, err := os.ReadFile(authMetaPath()); err == nil {
		_ = json.Unmarshal(raw, st)
	}
	authMetaCache = st
	return st
}

// saveAuthMetaLocked 原子写回（caller 必须持 authMetaMu）。失败只记日志——元数据丢失不致命。
func saveAuthMetaLocked(st *authMetaState) {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(AppDirPath(), 0o755)
	if err := AtomicWriteFile(authMetaPath(), raw, 0o600); err != nil {
		LogMessage(time.Now(), "保存凭证元数据失败: "+err.Error())
	}
}

// recordAuthCaptured：在「真正重新捕获/导入凭证」时调用（不是每次请求成功）。
// 记录捕获时间和方式，并清除本周期的柔性提醒去重位。
func recordAuthCaptured(method string) {
	authMetaMu.Lock()
	defer authMetaMu.Unlock()
	st := loadAuthMetaLocked()
	st.CapturedAt = time.Now()
	if method != "" {
		st.CaptureMethod = method
	}
	st.SoftWarned = false
	saveAuthMetaLocked(st)
}

// recordAuthStaleLifespan：凭证失效跃迁时回填一段「捕获→失效」寿命样本（小时）。
// 仅在有 captured_at 且本次确实是新一段 stale 时调用（由 markAuthStale 把关）。
func recordAuthStaleLifespan() {
	authMetaMu.Lock()
	defer authMetaMu.Unlock()
	st := loadAuthMetaLocked()
	now := time.Now()
	st.LastStaleAt = now
	if !st.CapturedAt.IsZero() {
		hours := now.Sub(st.CapturedAt).Hours()
		if hours > 0 {
			st.LifespanHours = append(st.LifespanHours, hours)
			if len(st.LifespanHours) > authLifespanHistoryMax {
				st.LifespanHours = st.LifespanHours[len(st.LifespanHours)-authLifespanHistoryMax:]
			}
		}
	}
	saveAuthMetaLocked(st)
}

// resetAuthMeta：登出/断开凭证时清空元数据（不保留寿命历史——已是别的账号语境）。
func resetAuthMeta() {
	authMetaMu.Lock()
	defer authMetaMu.Unlock()
	authMetaCache = &authMetaState{}
	saveAuthMetaLocked(authMetaCache)
}

// medianLifespanHoursLocked 返回历史寿命中位数（小时）；样本不足返回 0（调用方按「未知」处理）。
func medianLifespanHoursLocked(st *authMetaState) float64 {
	n := len(st.LifespanHours)
	if n < authMetaMinLifespanSamples {
		return 0
	}
	cp := append([]float64(nil), st.LifespanHours...)
	sort.Float64s(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

// maybeSoftWarnAuthAge：被动柔性提醒。在凭证请求成功（markAuthHealthy）时顺手判断——
// 若当前年龄已达历史寿命中位数的 authSoftWarnFraction，且本周期没提醒过，就推一次「建议择机重抓」。
// 这是「提前」而非「事后」：和 markAuthStale 的硬失效通知互补。
func maybeSoftWarnAuthAge() {
	authMetaMu.Lock()
	st := loadAuthMetaLocked()
	if st.CapturedAt.IsZero() || st.SoftWarned {
		authMetaMu.Unlock()
		return
	}
	median := medianLifespanHoursLocked(st)
	if median <= 0 {
		authMetaMu.Unlock()
		return
	}
	ageHours := time.Since(st.CapturedAt).Hours()
	if ageHours < median*authSoftWarnFraction {
		authMetaMu.Unlock()
		return
	}
	st.SoftWarned = true
	saveAuthMetaLocked(st)
	authMetaMu.Unlock()

	body := fmt.Sprintf("当前凭证已用约 %s，接近你以往的平均有效期（约 %s）。可以挑个空档重新获取一次，免得抢预约/取号时正好失效。",
		humanizeDuration(ageHours), humanizeDuration(median))
	body += recaptureDeepLinkSuffix()
	sendNotification("寿司郎 - 凭证快到期了", body)
}

// AuthMetaJSON 暴露给 /api/status，驱动前端通行证卡的「年龄 / 寿命 / 方式 / 续期方式」。
type AuthMetaJSON struct {
	CapturedAt         string  `json:"captured_at,omitempty"`
	AgeHours           float64 `json:"age_hours,omitempty"`
	AgeLabel           string  `json:"age_label,omitempty"`
	CaptureMethod      string  `json:"capture_method,omitempty"`
	CaptureMethodLabel string  `json:"capture_method_label,omitempty"`
	MedianLifespanDays float64 `json:"median_lifespan_days,omitempty"`
	SoftWarn           bool    `json:"soft_warn,omitempty"` // 年龄已接近历史寿命，UI 给个黄色提示
}

func getAuthMeta() AuthMetaJSON {
	authMetaMu.Lock()
	defer authMetaMu.Unlock()
	st := loadAuthMetaLocked()
	out := AuthMetaJSON{
		CaptureMethod:      st.CaptureMethod,
		CaptureMethodLabel: captureMethodLabel(st.CaptureMethod),
	}
	if st.CapturedAt.IsZero() {
		return out
	}
	out.CapturedAt = st.CapturedAt.Format(time.RFC3339)
	age := time.Since(st.CapturedAt).Hours()
	out.AgeHours = age
	out.AgeLabel = humanizeDuration(age)
	median := medianLifespanHoursLocked(st)
	if median > 0 {
		out.MedianLifespanDays = median / 24
		out.SoftWarn = age >= median*authSoftWarnFraction
	}
	return out
}

func captureMethodLabel(method string) string {
	switch method {
	case captureMethodPCWechat:
		return "PC 微信自动捕获"
	case captureMethodMobileProxy:
		return "手机抓包导入"
	case captureMethodImport:
		return "手动导入"
	default:
		return ""
	}
}

// humanizeDuration 把小时数说成「N 天 / N 小时」的人话。
func humanizeDuration(hours float64) string {
	if hours < 1 {
		return "不到 1 小时"
	}
	if hours < 24 {
		return fmt.Sprintf("%.0f 小时", hours)
	}
	days := hours / 24
	if days < 10 {
		return fmt.Sprintf("%.1f 天", days)
	}
	return fmt.Sprintf("%.0f 天", days)
}

// recaptureDeepLinkSuffix 给通知正文追加一个可点的续期深链（端口已知时）。
// 点开直接落到 Web UI 并自动拉起通行证向导（前端读 ?recapture=1）。
func recaptureDeepLinkSuffix() string {
	if port := GetActiveWebPort(); port > 0 {
		return fmt.Sprintf("\n\n一键续期：http://127.0.0.1:%d/?recapture=1", port)
	}
	return ""
}
