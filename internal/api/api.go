package api

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrNoReservationAvailable = errors.New("no reservation available")
var ErrReservationsEndpointUnavailable = errors.New("reservations endpoint unavailable")
var ErrActiveReservationExists = errors.New("active reservation exists")

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	settings   Settings
	httpClient *http.Client
	mu         sync.Mutex
	storeCache map[string]StoreInfo
}

func NewClient(settings Settings) *Client {
	return &Client{
		settings: settings,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		storeCache: map[string]StoreInfo{},
	}
}

func (c *Client) GetStoreInfo(ctx context.Context, storeID string) (StoreInfo, error) {
	c.mu.Lock()
	if store, ok := c.storeCache[storeID]; ok {
		c.mu.Unlock()
		return store, nil
	}
	c.mu.Unlock()

	query := url.Values{}
	query.Set("storeId", storeID)
	target := c.settings.BaseURL + "/wechat/api/2.0/getStoreById?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, target, c.BaseHeaders(c.settings.QueryAuthorization, ""), nil)
	if err != nil {
		return StoreInfo{}, err
	}

	var store StoreInfo
	if err := json.Unmarshal(body, &store); err != nil {
		return StoreInfo{}, fmt.Errorf("store info response is not a JSON object: %w", err)
	}
	c.mu.Lock()
	c.storeCache[storeID] = store
	c.mu.Unlock()
	return store, nil
}

func (c *Client) GetTimeslots(ctx context.Context, storeID string) ([]Slot, error) {
	query := url.Values{}
	query.Set("tableType", c.settings.TableType)
	query.Set("storeId", storeID)
	query.Set("numpersons", strconv.Itoa(c.settings.NumPersons()))
	target := c.settings.BaseURL + "/wechat/api/2.0/store/timeslots?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, target, c.BaseHeaders(c.settings.QueryAuthorization, ""), nil)
	if err != nil {
		return nil, err
	}

	var slots []Slot
	if err := json.Unmarshal(body, &slots); err != nil {
		return nil, fmt.Errorf("timeslots response is not a JSON array: %w", err)
	}
	return slots, nil
}

func (c *Client) CreateReservation(ctx context.Context, storeID, slotDate, slotTime string) (ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/createReservation"
	payload := map[string]any{
		"storeId":     storeID,
		"adult":       c.settings.Adult,
		"child":       c.settings.Child,
		"tableType":   c.settings.TableType,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
		"date":        slotDate,
		"time":        slotTime,
	}
	body, err := c.doJSON(ctx, http.MethodPost, target, c.BaseHeaders(c.settings.ReservationAuth, "application/json"), payload)
	if err != nil {
		if IsNoReservationText(err.Error()) {
			return ReservationRecord{}, ErrNoReservationAvailable
		}
		if IsActiveReservationText(err.Error()) {
			return ReservationRecord{}, ErrActiveReservationExists
		}
		return ReservationRecord{}, err
	}

	return parseReservationRecord(body, "reservation")
}

// CreateNetTicket 远程取号（日常排队），对应小程序「排队取号」。端点名来自抓包，
// payload 参照 createReservation 去掉日期/时间（取号即"现在"）。属实验性：成功与否、
// 字段细节以接口实际返回为准，错误原文会原样返回便于校正。
func (c *Client) CreateNetTicket(ctx context.Context, storeID string) (ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/createNetTicket"
	// 取号人数/桌型兜底：若预约偏好未配置（adult/child 都为 0、tableType 为空），
	// 用合理默认，避免拿"0 个人"提交导致官方 500/E010。
	adult, child := c.settings.Adult, c.settings.Child
	if adult <= 0 && child <= 0 {
		adult = 2
	}
	tableType := c.settings.TableType
	if strings.TrimSpace(tableType) == "" {
		tableType = "T"
	}
	payload := map[string]any{
		"storeId":     storeID,
		"adult":       adult,
		"child":       child,
		"tableType":   tableType,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	}
	body, err := c.doJSON(ctx, http.MethodPost, target, c.BaseHeaders(c.settings.ReservationAuth, "application/json"), payload)
	if err != nil {
		return ReservationRecord{}, err
	}
	return parseReservationRecord(body, "net ticket")
}

func (c *Client) GetNetTicketStatus(ctx context.Context) (ReservationRecord, error) {
	query := url.Values{}
	query.Set("wechatId", c.settings.WechatID)
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticket/status?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, target, c.BaseHeaders(c.settings.ReservationAuth, ""), nil)
	if err != nil {
		return ReservationRecord{}, err
	}
	return parseReservationRecord(body, "net ticket status")
}

func (c *Client) GetReservations(ctx context.Context) ([]ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/getReservations"
	body, err := c.doJSON(ctx, http.MethodPost, target, c.BaseHeaders(c.settings.ReservationAuth, "application/json"), map[string]any{
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	})
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, ErrReservationsEndpointUnavailable
		}
		return nil, err
	}
	var reservations []ReservationRecord
	if err := json.Unmarshal(body, &reservations); err != nil {
		var wrapper struct {
			Data []ReservationRecord `json:"data"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 == nil {
			markReservationRecordsKind(wrapper.Data, "reservation")
			return wrapper.Data, nil
		}
		return nil, fmt.Errorf("reservations response parse error: %w", err)
	}
	markReservationRecordsKind(reservations, "reservation")
	return reservations, nil
}

func (c *Client) CancelReservation(ctx context.Context, ticketID int64) error {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/cancelReservation"
	_, err := c.doJSON(ctx, http.MethodPost, target, c.BaseHeaders(c.settings.ReservationAuth, "application/json"), map[string]any{
		"ticketId":    ticketID,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	})
	return err
}

// CancelNetTicket 取消当前排队号（日常排队），对应小程序「取消取号」。端点与
// 参数来自抓包：POST cancelNetTicket，body 只带 wechatId（取消当前活跃号，不按 id）。
func (c *Client) CancelNetTicket(ctx context.Context) error {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/cancelNetTicket"
	_, err := c.doJSON(ctx, http.MethodPost, target, c.BaseHeaders(c.settings.ReservationAuth, "application/json"), map[string]any{
		"wechatId": c.settings.WechatID,
	})
	return err
}

func (c *Client) BaseHeaders(authorization, contentType string) map[string]string {
	headers := map[string]string{
		"Authorization": EnsureBearer(authorization),
		"X-App-Code":    c.settings.XAppCode,
		"X-App-Client":  c.settings.XAppClient,
		"User-Agent":    c.settings.UserAgent,
		"Referer":       c.settings.Referer,
		// 微信小程序 WebView 在每个 api_auth 请求都会带这个头，补齐以更贴近官方请求。
		"Xweb_xhr": "1",
	}
	if strings.TrimSpace(contentType) != "" {
		headers["Content-Type"] = contentType
	}
	return headers
}

func (c *Client) doJSON(ctx context.Context, method, target string, headers map[string]string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: NormalizeErrorBody(responseBody)}
	}
	if len(responseBody) == 0 {
		return nil, nil
	}
	if !json.Valid(responseBody) {
		return nil, fmt.Errorf("response is not JSON: %s", string(responseBody))
	}
	return responseBody, nil
}

func reservationLooksSuccessful(r ReservationRecord) bool {
	return r.TicketID != 0 || strings.TrimSpace(r.Number) != ""
}

func parseReservationRecord(body []byte, label string) (ReservationRecord, error) {
	var record ReservationRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return ReservationRecord{}, fmt.Errorf("%s response is not a JSON object: %w", label, err)
	}
	if reservationLooksSuccessful(record) {
		record = markReservationRecordKind(record, recordKindForLabel(label))
		return record, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, item := range []struct {
			key  string
			kind string
		}{
			{"netTicket", "net_ticket"},
			{"reservationTicket", "reservation"},
			{"reservation", "reservation"},
			{"data", recordKindForLabel(label)},
			{"ticket", recordKindForLabel(label)},
			{"currentTicket", recordKindForLabel(label)},
			{"current", recordKindForLabel(label)},
		} {
			key := item.key
			raw, ok := payload[key]
			if !ok || len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
				continue
			}
			if nested, ok := parseReservationCandidate(raw); ok {
				nested = markReservationRecordKind(nested, item.kind)
				return nested, nil
			}
		}
	}

	if err := ReservationBusinessError(body); err != nil {
		return ReservationRecord{}, err
	}
	return ReservationRecord{}, fmt.Errorf("%s response missing ticket id/number: %s", label, NormalizeErrorBody(body))
}

func parseReservationCandidate(raw json.RawMessage) (ReservationRecord, bool) {
	var record ReservationRecord
	if err := json.Unmarshal(raw, &record); err == nil && reservationLooksSuccessful(record) {
		return record, true
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return ReservationRecord{}, false
	}
	for _, detailKey := range []string{"TICKET_DETAIL", "ticketDetail", "ticket_detail"} {
		detailRaw, ok := wrapper[detailKey]
		if !ok || len(detailRaw) == 0 || bytes.Equal(detailRaw, []byte("null")) {
			continue
		}
		var detail ReservationRecord
		if err := json.Unmarshal(detailRaw, &detail); err != nil || !reservationLooksSuccessful(detail) {
			continue
		}
		for _, storeKey := range []string{"STORE_INFO", "storeInfo", "store_info"} {
			storeRaw, ok := wrapper[storeKey]
			if !ok || len(storeRaw) == 0 || bytes.Equal(storeRaw, []byte("null")) {
				continue
			}
			var store StoreInfo
			if err := json.Unmarshal(storeRaw, &store); err == nil {
				detail.StoreName = store.Name
				detail.StoreAddress = store.Address
				if detail.StoreID == "" && store.ID != 0 {
					detail.StoreID = strconv.Itoa(store.ID)
				}
			}
			break
		}
		return detail, true
	}
	return ReservationRecord{}, false
}

func recordKindForLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if strings.Contains(label, "net ticket") {
		return "net_ticket"
	}
	if strings.Contains(label, "reservation") {
		return "reservation"
	}
	return ""
}

func markReservationRecordKind(record ReservationRecord, kind string) ReservationRecord {
	if strings.TrimSpace(record.Kind) == "" {
		record.Kind = kind
	}
	return record
}

func markReservationRecordsKind(records []ReservationRecord, kind string) {
	for i := range records {
		records[i] = markReservationRecordKind(records[i], kind)
	}
}

func ReservationBusinessError(body []byte) error {
	text := NormalizeErrorBody(body)
	if IsNoReservationText(text) {
		return ErrNoReservationAvailable
	}
	if IsActiveReservationText(text) {
		return ErrActiveReservationExists
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	for _, key := range []string{"code", "errorCode", "error_code", "errCode", "message", "msg", "error"} {
		if v, ok := payload[key]; ok {
			text := fmt.Sprintf("%v", v)
			if IsNoReservationText(text) {
				return ErrNoReservationAvailable
			}
			if IsActiveReservationText(text) {
				return ErrActiveReservationExists
			}
		}
	}
	return nil
}

func IsNoReservationText(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "e044") ||
		strings.Contains(text, "no_more_reservations") ||
		strings.Contains(text, "no reservation available") ||
		strings.Contains(text, "名额已满") ||
		strings.Contains(text, "已满")
}

func IsActiveReservationText(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "e052") ||
		strings.Contains(text, "already") && strings.Contains(text, "reservation") ||
		strings.Contains(text, "one reservation at a time") ||
		strings.Contains(text, "すでにご予約") ||
		strings.Contains(text, "予約をいただいております") ||
		strings.Contains(text, "已有预约") ||
		strings.Contains(text, "已经有预约") ||
		strings.Contains(text, "一次只能预约")
}

func IsHTTPStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

// SetHTTPClient 替换内部 HTTP 客户端（主要用于测试注入）。
func (c *Client) SetHTTPClient(h *http.Client) { c.httpClient = h }
