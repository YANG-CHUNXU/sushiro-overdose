package core

import "regexp"

var (
	// phoneRedactor 匹配中国大陆手机号（1 开头、第二位 3-9、后 9 位数字）。
	// 用正则全量扫描文本，避免漏掉「日志里随手打出来的手机号」。
	phoneRedactor = regexp.MustCompile(`1[3-9][0-9]{9}`)
	// tokenRedactors 覆盖需要脱敏的凭证类字段，按 key=value / key: value 两种分隔形态匹配：
	//  1. authorization（可选 bearer 前缀）：抓 token 值。
	//  2. x-app-code：小程序应用标识，等同于接入凭证。
	//  3. query_authorization / reservation_authorization：与 Settings 里的两套令牌同义，必须脱敏。
	//     wechat_id / phone_number：用户身份数据（微信 openid / 手机号），属于 PII。
	// 这些 key 的值会被替换成 ***（保留 key 本身，便于看「这里曾经有个值」）。
	// 列表是枚举式的白名单：只覆盖已知敏感 key，避免误伤业务无关字段。
	tokenRedactors = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(bearer\s+)?[^\s,;]+`),
		regexp.MustCompile(`(?i)(x-app-code\s*[:=]\s*)[^\s,;]+`),
		regexp.MustCompile(`(?i)((query_authorization|reservation_authorization|wechat_id|phone_number)\s*[:=]\s*)[^\s,;]+`),
	}
)

// SanitizeDiagnosticLine 对一行文本做脱敏：手机号用 MaskPhone 打码（保留前 3 后 4），
// 凭证类字段值替换为 ***。用于诊断日志/错误信息在落盘或外发（如飞书）前清洗，防止泄露真实凭证和 PII。
// 注意：它只能按已知模式做正则替换，不是全量 JSON 解析脱敏——调用方仍应避免把整条原始响应外发。
func SanitizeDiagnosticLine(s string) string {
	s = phoneRedactor.ReplaceAllStringFunc(s, func(phone string) string {
		return MaskPhone(phone)
	})
	for _, re := range tokenRedactors {
		// ${1} 保留分组 1（即 key 和分隔符），把后面的值换成 ***。
		s = re.ReplaceAllString(s, `${1}***`)
	}
	return s
}
