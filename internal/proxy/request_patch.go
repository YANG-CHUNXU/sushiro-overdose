package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const mobileWeixinUA = "Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro Build/AP2A.240605.024; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.6422.147 Mobile Safari/537.36 XWEB/1250053 MMWEBSDK/20240501 MicroMessenger/8.0.50.2701(0x2800323D) WeChat/arm64 Weixin NetType/WIFI Language/zh_CN ABI/arm64 MiniProgramEnv/android"

var mobilePlatformFields = map[string]string{
	"platform":         "android",
	"os":               "android",
	"osName":           "android",
	"os_name":          "android",
	"system":           "Android 14",
	"source":           "android",
	"env":              "android",
	"miniProgramEnv":   "android",
	"mini_program_env": "android",
}

func patchSushiroRequestForForward(req *http.Request, body []byte) ([]byte, []string) {
	if req == nil {
		return body, nil
	}
	patches := []string{}
	if !looksMobileWeixinUA(req.Header.Get("User-Agent")) {
		req.Header.Set("User-Agent", mobileWeixinUA)
		patches = append(patches, "ua=mobile-weixin")
	}
	patchedBody, bodyPatches := patchSushiroRequestBody(req.Header.Get("Content-Type"), body)
	if len(bodyPatches) > 0 {
		body = patchedBody
		patches = append(patches, bodyPatches...)
	}
	return body, patches
}

func looksMobileWeixinUA(ua string) bool {
	ua = strings.ToLower(ua)
	return strings.Contains(ua, "micromessenger/") &&
		strings.Contains(ua, "mobile") &&
		!strings.Contains(ua, "windows") &&
		!strings.Contains(ua, "miniprogramenv/windows")
}

func patchSushiroRequestBody(contentType string, body []byte) ([]byte, []string) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || hasRequestSignatureField(trimmed) {
		return body, nil
	}
	if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		return patchFormPlatformBody(trimmed)
	}
	if json.Valid(trimmed) {
		return patchJSONPlatformBody(trimmed)
	}
	return body, nil
}

func hasRequestSignatureField(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, key := range []string{"signature", "sign", "sig", "hmac", "nonce"} {
		if strings.Contains(lower, `"`+key+`"`) || strings.Contains(lower, key+"=") {
			return true
		}
	}
	return false
}

func patchJSONPlatformBody(body []byte) ([]byte, []string) {
	var object map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&object); err != nil {
		return body, nil
	}
	patches := []string{}
	for key, replacement := range mobilePlatformFields {
		current, ok := object[key].(string)
		if !ok || !isWindowsPlatformValue(current) {
			continue
		}
		object[key] = replacement
		patches = append(patches, "body."+key+"=android")
	}
	if len(patches) == 0 {
		return body, nil
	}
	out, err := json.Marshal(object)
	if err != nil {
		return body, nil
	}
	return out, patches
}

func patchFormPlatformBody(body []byte) ([]byte, []string) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return body, nil
	}
	patches := []string{}
	for key, replacement := range mobilePlatformFields {
		current := values.Get(key)
		if !isWindowsPlatformValue(current) {
			continue
		}
		values.Set(key, replacement)
		patches = append(patches, "body."+key+"=android")
	}
	if len(patches) == 0 {
		return body, nil
	}
	return []byte(values.Encode()), patches
}

func isWindowsPlatformValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	return strings.Contains(value, "windows") ||
		value == "win32" ||
		value == "win64" ||
		value == "pc" ||
		value == "desktop" ||
		value == "miniprogramenv/windows"
}
