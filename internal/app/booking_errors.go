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

func friendlyOfficialAPIError(err error) string {
	if err == nil {
		return ""
	}
	if isKnownOfficialServerError(err) {
		return "官方接口返回 E010/error.server，通常是该门店/时段当前不可提交或官方临时异常；已保留认证，可稍后重试"
	}
	if isOfficialServerHTTPError(err) {
		return "官方接口返回 HTTP 500，已保留认证；如果小程序也失败，通常是官方临时异常"
	}
	return err.Error()
}

func bookingServerErrorLog(slotLabel string, err error) string {
	return fmt.Sprintf("%s — %s，跳过当前时段 %d 秒后再试", slotLabel, friendlyOfficialAPIError(err), int(reservationServerErrorCooldown.Seconds()))
}
