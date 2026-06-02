package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type AuthProbeReport struct {
	OK      bool              `json:"ok"`
	StoreID string            `json:"store_id,omitempty"`
	Store   string            `json:"store,omitempty"`
	Missing []string          `json:"missing,omitempty"`
	Results []AuthProbeResult `json:"results"`
	Advice  []string          `json:"advice,omitempty"`
}

type AuthProbeResult struct {
	Name      string `json:"name"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	OK        bool   `json:"ok"`
	Skipped   bool   `json:"skipped,omitempty"`
	Status    int    `json:"status,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

func handleAuthProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET or POST")
		return
	}
	report := RunAuthProbe(r.Context(), strings.TrimSpace(r.URL.Query().Get("store")))
	status := http.StatusOK
	if !report.OK {
		status = http.StatusBadGateway
	}
	writeJSONStatus(w, status, report)
}

func RunAuthProbe(ctx context.Context, requestedStore string) AuthProbeReport {
	report := AuthProbeReport{}
	tokens, err := loadLocalConfig()
	if err != nil {
		report.Missing = []string{"config.json"}
		report.Advice = []string{"本地还没有认证参数；先从可用设备获取认证，或导入已有 config.json。"}
		report.Results = append(report.Results, AuthProbeResult{Name: "读取本地认证", OK: false, Detail: err.Error()})
		return report
	}

	prefs := LoadPreferences()
	settings := tokens.toSettingsWithPrefs(prefs)
	storeID := chooseProbeStoreID(requestedStore, settings.StoreIDs, tokens.StoreIDs)
	report.StoreID = storeID

	if missing := tokens.missingFields(false); len(missing) > 0 {
		report.Missing = append(report.Missing, missing...)
		report.Results = append(report.Results, AuthProbeResult{Name: "查询认证参数", OK: false, Detail: "缺少: " + strings.Join(missing, ", ")})
		report.Advice = append(report.Advice, "查询认证不完整，基础门店/时段接口无法验证。")
		return report
	}
	if storeID == "" {
		report.Missing = append(report.Missing, "门店")
		report.Results = append(report.Results, AuthProbeResult{Name: "选择探测门店", OK: false, Detail: "没有可用门店 ID"})
		report.Advice = append(report.Advice, "认证配置里没有门店 ID，先在小程序里打开目标门店，或手动补充 store_ids。")
		return report
	}

	httpClient := directProbeHTTPClient()
	storeResult, storeName := probeGetStoreInfo(ctx, httpClient, settings, storeID)
	report.Results = append(report.Results, storeResult)
	report.Store = storeName
	report.Results = append(report.Results, probeTimeslots(ctx, httpClient, settings, storeID))

	if missing := tokens.missingFields(true); len(missing) > 0 {
		report.Results = append(report.Results, AuthProbeResult{
			Name:    "预约认证接口",
			Skipped: true,
			Detail:  "缺少: " + strings.Join(missing, ", "),
		})
	} else {
		report.Results = append(report.Results, probeReservations(ctx, httpClient, settings))
	}

	report.OK = true
	for _, result := range report.Results {
		if !result.OK && !result.Skipped {
			report.OK = false
			break
		}
	}
	report.Advice = authProbeAdvice(report)
	return report
}

func chooseProbeStoreID(requested string, preferred []string, captured []string) string {
	if requested != "" {
		return requested
	}
	for _, values := range [][]string{preferred, captured} {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func directProbeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				NextProtos: []string{"h2", "http/1.1"},
			},
			ForceAttemptHTTP2: true,
		},
	}
}

func probeGetStoreInfo(ctx context.Context, client *http.Client, settings Settings, storeID string) (AuthProbeResult, string) {
	query := url.Values{}
	query.Set("storeId", storeID)
	path := "/wechat/api/2.0/getStoreById?" + query.Encode()
	result, body := probeOfficialAPI(ctx, client, http.MethodGet, settings.BaseURL+path, path, NewClient(settings).baseHeaders(settings.QueryAuthorization, ""), nil)
	result.Name = "基础接口：门店信息"
	if result.OK {
		var store StoreInfo
		if err := json.Unmarshal(body, &store); err == nil {
			name := strings.TrimSpace(store.Name)
			result.Detail = fmt.Sprintf("门店 %s %s", storeID, defaultString(name, "返回正常"))
			return result, name
		}
	}
	return result, ""
}

func probeTimeslots(ctx context.Context, client *http.Client, settings Settings, storeID string) AuthProbeResult {
	query := url.Values{}
	query.Set("tableType", settings.TableType)
	query.Set("storeId", storeID)
	query.Set("numpersons", fmt.Sprintf("%d", settings.NumPersons()))
	path := "/wechat/api/2.0/store/timeslots?" + query.Encode()
	result, body := probeOfficialAPI(ctx, client, http.MethodGet, settings.BaseURL+path, path, NewClient(settings).baseHeaders(settings.QueryAuthorization, ""), nil)
	result.Name = "基础接口：时段列表"
	if result.OK {
		var slots []Slot
		if err := json.Unmarshal(body, &slots); err == nil {
			result.Detail = fmt.Sprintf("返回 %d 个时段", len(slots))
		}
	}
	return result
}

func probeReservations(ctx context.Context, client *http.Client, settings Settings) AuthProbeResult {
	path := "/wechat/api_auth/2.0/ticketing/getReservations"
	payload := map[string]any{
		"wechatId":    settings.WechatID,
		"phoneNumber": settings.PhoneNumber,
	}
	result, body := probeOfficialAPI(ctx, client, http.MethodPost, settings.BaseURL+path, path, NewClient(settings).baseHeaders(settings.ReservationAuth, "application/json"), payload)
	result.Name = "认证接口：当前预约"
	result = normalizeReservationsProbeResult(result)
	if result.Skipped {
		return result
	}
	if result.OK {
		var reservations []ReservationRecord
		if err := json.Unmarshal(body, &reservations); err == nil {
			result.Detail = fmt.Sprintf("返回 %d 条预约", len(reservations))
		} else {
			result.Detail = "返回 JSON 正常"
		}
	}
	return result
}

func normalizeReservationsProbeResult(result AuthProbeResult) AuthProbeResult {
	if result.Status == http.StatusNotFound && strings.Contains(result.Path, "/ticketing/getReservations") {
		result.Skipped = true
		result.OK = false
		result.Detail = "当前预约查询接口不可用或已变更；门店/时段基础接口已单独验证，抢号提交不依赖此自检项。"
	}
	return result
}

func probeOfficialAPI(ctx context.Context, client *http.Client, method, target, path string, headers map[string]string, payload any) (AuthProbeResult, []byte) {
	result := AuthProbeResult{Method: method, Path: path}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			result.Detail = "构造请求失败: " + err.Error()
			return result, nil
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		result.Detail = "构造请求失败: " + err.Error()
		return result, nil
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	start := time.Now()
	resp, err := client.Do(req)
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Detail = sanitizeDiagnosticLine(err.Error())
		return result, nil
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		result.Detail = "读取响应失败: " + readErr.Error()
		return result, nil
	}
	if resp.StatusCode >= 400 {
		result.Detail = normalizeErrorBody(respBody)
		return result, respBody
	}
	if len(respBody) > 0 && !json.Valid(respBody) {
		result.Detail = "响应不是 JSON: " + sanitizeDiagnosticLine(string(respBody))
		return result, respBody
	}
	result.OK = true
	result.Detail = "HTTP " + resp.Status
	return result, respBody
}

func authProbeAdvice(report AuthProbeReport) []string {
	if report.OK {
		return []string{"基础接口可用，认证参数本身有效；Windows 问题集中在 PC 微信代理/MITM 捕获阶段。"}
	}
	out := []string{}
	for _, result := range report.Results {
		if result.OK || result.Skipped {
			continue
		}
		if result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
			out = append(out, "官方接口返回认证失败，优先重新获取认证参数。")
			continue
		}
		if result.Status >= 500 {
			out = append(out, "官方接口返回服务器错误，保留本结果继续排查请求头/接口兼容性。")
			continue
		}
		if result.Status == 0 && strings.TrimSpace(result.Detail) != "" {
			out = append(out, "本机直连官方接口失败，先排查网络、DNS 或系统代理残留。")
			continue
		}
	}
	if len(out) == 0 {
		out = append(out, "基础接口未全部通过，复制本结果继续排查。")
	}
	return uniqueNonEmptyStrings(out)
}

func cmdAuthProbe() {
	report := RunAuthProbe(context.Background(), "")
	printAuthProbeReport(report)
	if !report.OK {
		os.Exit(1)
	}
}

func printAuthProbeReport(report AuthProbeReport) {
	status := "FAILED"
	if report.OK {
		status = "OK"
	}
	fmt.Printf("认证基础接口自检: %s\n", status)
	if report.StoreID != "" {
		fmt.Printf("门店: %s %s\n", report.StoreID, report.Store)
	}
	for _, result := range report.Results {
		state := "FAIL"
		if result.OK {
			state = "OK"
		} else if result.Skipped {
			state = "SKIP"
		}
		fmt.Printf("  - %s [%s]", result.Name, state)
		if result.Status != 0 {
			fmt.Printf(" HTTP %d", result.Status)
		}
		if result.Detail != "" {
			fmt.Printf(" %s", result.Detail)
		}
		fmt.Println()
	}
	for _, advice := range report.Advice {
		fmt.Println("建议:", advice)
	}
}
