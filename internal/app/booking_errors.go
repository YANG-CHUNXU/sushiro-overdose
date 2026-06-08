package app

import api "github.com/Ryujoxys/sushiro-overdose/internal/api"

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const reservationServerErrorCooldown = 30 * time.Second

func bookingSlotKey(storeID, date, start string) string {
	return storeID + "|" + date + "|" + start
}

func isTemporaryBookingSkipped(skips map[string]time.Time, key string, now time.Time) bool {
	if skips == nil {
		return false
	}
	until, ok := skips[key]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(skips, key)
	return false
}

func markTemporaryBookingSkip(skips map[string]time.Time, key string, now time.Time) {
	if skips != nil {
		skips[key] = now.Add(reservationServerErrorCooldown)
	}
}

func isOfficialServerHTTPError(err error) bool {
	return api.IsHTTPStatus(err, http.StatusInternalServerError)
}

func isKnownOfficialServerError(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, `"code":"e010"`) ||
		strings.Contains(body, `"code": "e010"`) ||
		strings.Contains(body, "error.server")
}

func isTicketAlreadyIssuedText(text string) bool {
	body := strings.ToLower(text)
	return strings.Contains(body, `"code":"e034"`) ||
		strings.Contains(body, `"code": "e034"`) ||
		strings.Contains(body, "too_many_tickets") ||
		strings.Contains(body, "already issued")
}

func isTicketAlreadyIssuedError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusConflict && isTicketAlreadyIssuedText(apiErr.Body)
	}
	return isTicketAlreadyIssuedText(err.Error())
}

func friendlyNetTicketError(err error) string {
	if isTicketAlreadyIssuedError(err) {
		return "官方提示这台终端已经发过排队号，但本地没有拿到号码；不要重复取号，请打开 PC 微信寿司郎小程序的排队/我的预约页查看，或开启接口调试恢复号码"
	}
	return friendlyOfficialAPIError(err)
}

func friendlyOfficialAPIError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, api.ErrActiveReservationExists) {
		return "官方仍认为当前账号已有预约；如果你刚在手机上取消，请等小程序状态同步后再抢，或重新打开寿司郎小程序确认“我的预约”已清空"
	}
	if isKnownOfficialServerError(err) {
		return "官方接口返回 E010/error.server，通常是该门店/时段当前不可提交或官方临时异常；已保留凭证，可稍后重试"
	}
	if isOfficialServerHTTPError(err) {
		return "官方接口返回 HTTP 500，已保留凭证；如果小程序也失败，通常是官方临时异常"
	}
	return err.Error()
}

func bookingServerErrorLog(slotLabel string, err error) string {
	return fmt.Sprintf("%s — %s，跳过当前时段 %d 秒后再试", slotLabel, friendlyOfficialAPIError(err), int(reservationServerErrorCooldown.Seconds()))
}
