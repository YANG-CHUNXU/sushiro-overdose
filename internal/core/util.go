package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultString 返回 value，若为空白则返回 fallback。
func DefaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// NormalizeErrorBody 把接口错误响应体规整成可读字符串（保留原始技术细节）。
func NormalizeErrorBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		return StringifyJSON(payload)
	}
	return string(body)
}

// MaskPhone 脱敏手机号用于展示/日志。
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func StringifyJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}
