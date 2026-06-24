package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// auth_verify.go —— 凭证「真实取号」验证。
//
// 背景：本机录入的就是「远程取号」要用的那套凭证。光验证查询/时段接口（auth_probe）不等于
// 取号能用——取号走 ReservationAuth、有独立的风控。最贴近真相的验证方式就是用户说的：
// 找一家正在开放线上取号的门店，取号后立即取消。能取到=凭证还生效；取不到（凭证类错误）=该刷新了。
//
// 安全：只会取消「本次验证刚取的号」。若官方返回「你已有排队号」，说明凭证本就有效，
// 此时绝不取消（那是用户的真号）——直接判定有效。服务端临时错误则判「无法确定」，不动凭证健康。

// AuthVerifyResult 是 /api/auth/verify 的返回。
type AuthVerifyResult struct {
	OK      bool   `json:"ok"`              // 是否得到确定结论（valid 才有意义）
	Valid   bool   `json:"valid"`           // 凭证是否仍生效
	Method  string `json:"method"`          // ticket（真实取号）/ existing（已有号）/ none（无可测门店）
	Store   string `json:"store,omitempty"` // 测试用的门店名
	StoreID string `json:"store_id,omitempty"`
	Number  string `json:"number,omitempty"` // 测试取到的号（已立即取消）
	Message string `json:"message"`          // 给用户看的人话
	Detail  string `json:"detail,omitempty"` // 原始错误，排障用
}

func handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	res := runAuthVerify(r.Context())
	writeJSON(w, res)
}

func runAuthVerify(ctx context.Context) AuthVerifyResult {
	client := currentAuthedClient()
	if client == nil {
		return AuthVerifyResult{OK: false, Message: "还没有可用于取号的完整凭证；先获取或重新导入通行证再验证。"}
	}

	// 1) 先看是否已有活跃排队号。有的话凭证显然有效，且绝不能去取号/取消影响用户真号。
	if status, err := client.GetNetTicketStatus(ctx); err == nil && netTicketLooksSuccessful(status) {
		markAuthHealthy()
		return AuthVerifyResult{
			OK: true, Valid: true, Method: "existing",
			Number:  status.Number,
			Message: "凭证有效：检测到你当前已有排队号，未执行取号测试（以免影响你的号）。",
		}
	} else if err != nil && isAuthError(err) {
		markAuthStale("查询排队号状态返回 401/403，凭证已失效")
		return AuthVerifyResult{OK: true, Valid: false, Method: "ticket", Message: "凭证已失效（查询排队号被拒），需要重新获取通行证。", Detail: err.Error()}
	}

	// 2) 找一家正在开放线上取号的门店做真实取号测试（优先用户常用门店，其次全部门店里第一家开放的）。
	storeID, storeName, ok := pickOnlineOpenStore(ctx)
	if !ok {
		return AuthVerifyResult{
			OK: false, Method: "none",
			Message: "现在没有正在开放线上取号的门店，无法做真实取号验证。换到饭点（约 11:00-13:00 / 17:00-20:00）门店开放取号时再试。",
		}
	}

	// 3) 取号 → 立即取消。
	ticket, err := client.CreateNetTicket(ctx, storeID)
	if err != nil {
		// 「你已有排队号」= 凭证有效（成功认证、被官方业务规则拦），不取消任何东西。
		if isTicketAlreadyIssuedError(err) {
			markAuthHealthy()
			return AuthVerifyResult{OK: true, Valid: true, Method: "existing", Message: "凭证有效：官方提示你已有排队号，未重复取号。"}
		}
		// 凭证类错误 → 失效。
		if isAuthError(err) || isCredentialRefreshLikelyError(err) {
			markAuthStale("取号测试返回凭证失败")
			return AuthVerifyResult{OK: true, Valid: false, Method: "ticket", Store: storeName, StoreID: storeID, Message: "凭证已失效：取号被拒，需要重新获取通行证。", Detail: friendlyNetTicketError(err)}
		}
		// 官方服务端临时错误：无法判定，不改凭证健康。
		return AuthVerifyResult{OK: false, Method: "ticket", Store: storeName, StoreID: storeID, Message: "官方服务暂时异常，没能完成取号验证，请稍后重试。", Detail: friendlyNetTicketError(err)}
	}

	// 成功取号 → 立即取消（best-effort，失败也记日志，避免给用户留下真号）。
	if cancelErr := client.CancelNetTicket(ctx); cancelErr != nil {
		LogMessage(time.Now(), "[凭证验证] 取号成功但取消失败，请到小程序手动取消: "+cancelErr.Error())
		markAuthHealthy()
		return AuthVerifyResult{
			OK: true, Valid: true, Method: "ticket", Store: storeName, StoreID: storeID, Number: ticket.Number,
			Message: "凭证有效：取号成功。但自动取消失败，请到寿司郎小程序手动取消这个号（" + DefaultString(ticket.Number, "见我的排队") + "）。",
			Detail:  cancelErr.Error(),
		}
	}
	markAuthHealthy()
	return AuthVerifyResult{
		OK: true, Valid: true, Method: "ticket", Store: storeName, StoreID: storeID, Number: ticket.Number,
		Message: "凭证有效：在「" + storeName + "」成功取号并已立即取消，取号能力正常。",
	}
}

// pickOnlineOpenStore 选一家当前开放线上取号的门店做测试。优先用户已选门店，其次全部门店里第一家开放的。
func pickOnlineOpenStore(ctx context.Context) (storeID, storeName string, ok bool) {
	stores, err := NewQueueLiveClient().CachedAllStores(ctx)
	if err != nil {
		return "", "", false
	}
	prefs := LoadPreferences()
	preferred := map[string]bool{}
	for _, id := range prefs.SelectedStores {
		preferred[id] = true
	}
	var fallbackID, fallbackName string
	for _, s := range stores {
		if !queueLiveStoreOnlineOpen(s) {
			continue
		}
		id := strconv.Itoa(s.ID)
		if preferred[id] {
			return id, s.Name, true
		}
		if fallbackID == "" {
			fallbackID, fallbackName = id, s.Name
		}
	}
	if fallbackID != "" {
		return fallbackID, fallbackName, true
	}
	return "", "", false
}
