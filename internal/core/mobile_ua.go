package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const mobileUAConfigFile = "mobile_ua.json"

const defaultMobileWeixinUA = "Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro Build/AP2A.240605.024; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.6422.147 Mobile Safari/537.36 XWEB/1250053 MMWEBSDK/20240501 MicroMessenger/8.0.50.2701(0x2800323D) WeChat/arm64 Weixin NetType/WIFI Language/zh_CN ABI/arm64 MiniProgramEnv/android"

var miniProgramEnvPattern = regexp.MustCompile(`(?i)MiniProgramEnv/[A-Za-z0-9_/-]+`)

type MobileUAConfig struct {
	UserAgent           string    `json:"user_agent"`
	NormalizedUserAgent string    `json:"normalized_user_agent"`
	CapturedAt          time.Time `json:"captured_at"`
	Source              string    `json:"source,omitempty"`
	RemoteAddr          string    `json:"remote_addr,omitempty"`
}

func MobileUAPath() string {
	return filepath.Join(AppDirPath(), mobileUAConfigFile)
}

func DefaultMobileWeixinUA() string {
	return defaultMobileWeixinUA
}

func LoadMobileUA() (MobileUAConfig, error) {
	raw, err := os.ReadFile(MobileUAPath())
	if err != nil {
		return MobileUAConfig{}, err
	}
	var cfg MobileUAConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return MobileUAConfig{}, err
	}
	cfg.UserAgent = strings.TrimSpace(cfg.UserAgent)
	cfg.NormalizedUserAgent = NormalizeMobileWeixinUA(cfg.NormalizedUserAgent)
	if cfg.NormalizedUserAgent == "" {
		cfg.NormalizedUserAgent = NormalizeMobileWeixinUA(cfg.UserAgent)
	}
	return cfg, nil
}

func SaveMobileUA(rawUA, source, remoteAddr string) (MobileUAConfig, error) {
	cfg := MobileUAConfig{
		UserAgent:           strings.TrimSpace(rawUA),
		NormalizedUserAgent: NormalizeMobileWeixinUA(rawUA),
		CapturedAt:          time.Now(),
		Source:              strings.TrimSpace(source),
		RemoteAddr:          strings.TrimSpace(remoteAddr),
	}
	if cfg.NormalizedUserAgent == "" {
		cfg.NormalizedUserAgent = defaultMobileWeixinUA
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return MobileUAConfig{}, err
	}
	if err := os.MkdirAll(AppDirPath(), 0o755); err != nil {
		return MobileUAConfig{}, err
	}
	if err := os.WriteFile(MobileUAPath(), raw, 0o600); err != nil {
		return MobileUAConfig{}, err
	}
	return cfg, nil
}

func EffectiveUserAgent(capturedUA string) string {
	if cfg, err := LoadMobileUA(); err == nil && cfg.NormalizedUserAgent != "" {
		return cfg.NormalizedUserAgent
	}
	return NormalizeMobileWeixinUA(capturedUA)
}

func NormalizeMobileWeixinUA(ua string) string {
	ua = strings.Join(strings.Fields(strings.TrimSpace(ua)), " ")
	lower := strings.ToLower(ua)
	if ua == "" || strings.Contains(lower, "windows") || strings.Contains(lower, "miniprogramenv/windows") {
		return defaultMobileWeixinUA
	}
	if !strings.Contains(lower, "micromessenger/") {
		return defaultMobileWeixinUA
	}
	env := "android"
	if strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad") || strings.Contains(lower, "cpu iphone os") {
		env = "ios"
	}
	if miniProgramEnvPattern.MatchString(ua) {
		return miniProgramEnvPattern.ReplaceAllString(ua, "MiniProgramEnv/"+env)
	}
	return strings.TrimSpace(ua + " MiniProgramEnv/" + env)
}

func LooksMobileWeixinUA(ua string) bool {
	lower := strings.ToLower(strings.TrimSpace(ua))
	return strings.Contains(lower, "micromessenger/") &&
		!strings.Contains(lower, "windows") &&
		!strings.Contains(lower, "miniprogramenv/windows") &&
		(strings.Contains(lower, "miniprogramenv/android") ||
			strings.Contains(lower, "miniprogramenv/ios") ||
			strings.Contains(lower, "mobile"))
}
