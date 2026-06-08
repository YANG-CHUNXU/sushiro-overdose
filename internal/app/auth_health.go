package app

import (
	"sync"
	"time"
)

// authHealth 跟踪"本机捕获的凭证令牌是否还有效"。背景：工具复用的是手机/PC 微信抓包
// 得到的同一套令牌，寿司郎一个账号同一时间只认一个活跃会话——在手机上用过小程序后，
// 电脑这边的令牌就会被顶失效。这里用被动方式检测（只看用户已触发的凭证请求结果，
// 不做任何后台轮询，避免再把手机会话顶掉），并在 ok→stale 跃迁时提醒一次。
// 详见 specs/006-auth-staleness-reminder。

const (
	authHealthUnknown = "unknown" // 尚无判断（含从未捕获）
	authHealthOK      = "ok"      // 最近一次凭证请求成功
	authHealthStale   = "stale"   // 最近一次凭证请求被官方判为凭证失败
)

type authHealthTracker struct {
	mu        sync.RWMutex
	status    string
	reason    string
	checkedAt time.Time
	notified  bool // 当前 stale 周期内是否已推过通知，避免刷屏
}

var authHealth = &authHealthTracker{status: authHealthUnknown}

// AuthHealthJSON 是 /api/status 暴露的凭证健康。
type AuthHealthJSON struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

func getAuthHealth() AuthHealthJSON {
	authHealth.mu.RLock()
	defer authHealth.mu.RUnlock()
	out := AuthHealthJSON{Status: authHealth.status, Reason: authHealth.reason}
	if !authHealth.checkedAt.IsZero() {
		out.CheckedAt = authHealth.checkedAt.Format(time.RFC3339)
	}
	return out
}

// markAuthHealthy：一次凭证请求成功，或刚重新捕获/导入凭证后调用。清除 stale 与通知去重。
func markAuthHealthy() {
	authHealth.mu.Lock()
	authHealth.status = authHealthOK
	authHealth.reason = ""
	authHealth.checkedAt = time.Now()
	authHealth.notified = false
	authHealth.mu.Unlock()
}

func resetAuthHealth() {
	authHealth.mu.Lock()
	authHealth.status = authHealthUnknown
	authHealth.reason = ""
	authHealth.checkedAt = time.Time{}
	authHealth.notified = false
	authHealth.mu.Unlock()
}

// markAuthStale：官方判定凭证失败时调用。仅在 ok/unknown→stale 跃迁时推一次通知。
func markAuthStale(reason string) {
	authHealth.mu.Lock()
	wasStale := authHealth.status == authHealthStale
	authHealth.status = authHealthStale
	if reason != "" {
		authHealth.reason = reason
	}
	authHealth.checkedAt = time.Now()
	shouldNotify := !wasStale && !authHealth.notified
	authHealth.notified = true
	authHealth.mu.Unlock()

	if shouldNotify {
		body := "寿司郎凭证会过期；在手机上用过寿司郎小程序后，电脑这边的凭证也会失效（同一账号只认一个会话）。请在工具里重置认证并重新获取凭证。"
		if reason != "" {
			body = reason + "。" + body
		}
		sendNotification("寿司郎 - 凭证可能已过期",
			body)
	}
}

// noteAuthResult：被动检测入口。把一次"需要凭证的请求"的结果喂进来——
// err 为凭证失败或高概率需刷新凭证的官方错误 → stale；err 为 nil（成功）→ healthy；其它错误不改变凭证健康。
func noteAuthResult(err error) {
	if err == nil {
		markAuthHealthy()
		return
	}
	if isAuthError(err) {
		markAuthStale("官方接口返回凭证失败（401/403）")
		return
	}
	if isCredentialRefreshLikelyError(err) {
		markAuthStale("官方接口返回 E010/error.server，凭证可能需要刷新")
	}
}
