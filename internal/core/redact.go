package core

import "regexp"

var (
	phoneRedactor  = regexp.MustCompile(`1[3-9][0-9]{9}`)
	tokenRedactors = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(bearer\s+)?[^\s,;]+`),
		regexp.MustCompile(`(?i)(x-app-code\s*[:=]\s*)[^\s,;]+`),
		regexp.MustCompile(`(?i)((query_authorization|reservation_authorization|wechat_id|phone_number)\s*[:=]\s*)[^\s,;]+`),
	}
)

// SanitizeDiagnosticLine 对一行文本做脱敏：手机号打码、认证类字段值替换为 ***。
func SanitizeDiagnosticLine(s string) string {
	s = phoneRedactor.ReplaceAllStringFunc(s, func(phone string) string {
		return MaskPhone(phone)
	})
	for _, re := range tokenRedactors {
		s = re.ReplaceAllString(s, `${1}***`)
	}
	return s
}
