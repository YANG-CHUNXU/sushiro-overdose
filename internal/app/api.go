package app

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

var errNoReservationAvailable = errors.New("no reservation available")

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
	body, err := c.doJSON(ctx, http.MethodGet, target, c.baseHeaders(c.settings.QueryAuthorization, ""), nil)
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
	body, err := c.doJSON(ctx, http.MethodGet, target, c.baseHeaders(c.settings.QueryAuthorization, ""), nil)
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
	body, err := c.doJSON(ctx, http.MethodPost, target, c.baseHeaders(c.settings.ReservationAuth, "application/json"), payload)
	if err != nil {
		if isNoReservationText(err.Error()) {
			return ReservationRecord{}, errNoReservationAvailable
		}
		return ReservationRecord{}, err
	}

	var reservation ReservationRecord
	if err := json.Unmarshal(body, &reservation); err != nil {
		return ReservationRecord{}, fmt.Errorf("reservation response is not a JSON object: %w", err)
	}
	if !reservationLooksSuccessful(reservation) {
		var wrapper struct {
			Data ReservationRecord `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err == nil && reservationLooksSuccessful(wrapper.Data) {
			return wrapper.Data, nil
		}
		if err := reservationBusinessError(body); err != nil {
			return ReservationRecord{}, err
		}
		return ReservationRecord{}, fmt.Errorf("reservation response missing ticket id/number: %s", NormalizeErrorBody(body))
	}
	return reservation, nil
}

// CreateNetTicket 远程取号（日常排队），对应小程序「排队取号」。端点名来自抓包，
// payload 参照 createReservation 去掉日期/时间（取号即"现在"）。属实验性：成功与否、
// 字段细节以接口实际返回为准，错误原文会原样返回便于校正。
func (c *Client) CreateNetTicket(ctx context.Context, storeID string) (ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/createNetTicket"
	payload := map[string]any{
		"storeId":     storeID,
		"adult":       c.settings.Adult,
		"child":       c.settings.Child,
		"tableType":   c.settings.TableType,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	}
	body, err := c.doJSON(ctx, http.MethodPost, target, c.baseHeaders(c.settings.ReservationAuth, "application/json"), payload)
	if err != nil {
		return ReservationRecord{}, err
	}
	var ticket ReservationRecord
	if err := json.Unmarshal(body, &ticket); err != nil {
		return ReservationRecord{}, fmt.Errorf("net ticket response is not a JSON object: %w", err)
	}
	if !reservationLooksSuccessful(ticket) {
		var wrapper struct {
			Data ReservationRecord `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err == nil && reservationLooksSuccessful(wrapper.Data) {
			return wrapper.Data, nil
		}
		if err := reservationBusinessError(body); err != nil {
			return ReservationRecord{}, err
		}
		return ReservationRecord{}, fmt.Errorf("net ticket response missing ticket id/number: %s", NormalizeErrorBody(body))
	}
	return ticket, nil
}

func (c *Client) GetReservations(ctx context.Context) ([]ReservationRecord, error) {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/getReservations"
	body, err := c.doJSON(ctx, http.MethodPost, target, c.baseHeaders(c.settings.ReservationAuth, "application/json"), map[string]any{
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	})
	if err != nil {
		return nil, err
	}
	var reservations []ReservationRecord
	if err := json.Unmarshal(body, &reservations); err != nil {
		var wrapper struct {
			Data []ReservationRecord `json:"data"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 == nil && len(wrapper.Data) > 0 {
			return wrapper.Data, nil
		}
		return nil, fmt.Errorf("reservations response parse error: %w", err)
	}
	return reservations, nil
}

func (c *Client) CancelReservation(ctx context.Context, ticketID int64) error {
	target := c.settings.BaseURL + "/wechat/api_auth/2.0/ticketing/cancelReservation"
	_, err := c.doJSON(ctx, http.MethodPost, target, c.baseHeaders(c.settings.ReservationAuth, "application/json"), map[string]any{
		"ticketId":    ticketID,
		"wechatId":    c.settings.WechatID,
		"phoneNumber": c.settings.PhoneNumber,
	})
	return err
}

func (c *Client) baseHeaders(authorization, contentType string) map[string]string {
	headers := map[string]string{
		"Authorization": EnsureBearer(authorization),
		"X-App-Code":    c.settings.XAppCode,
		"X-App-Client":  c.settings.XAppClient,
		"User-Agent":    c.settings.UserAgent,
		"Referer":       c.settings.Referer,
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

func reservationBusinessError(body []byte) error {
	text := NormalizeErrorBody(body)
	if isNoReservationText(text) {
		return errNoReservationAvailable
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	for _, key := range []string{"code", "errorCode", "error_code", "errCode", "message", "msg", "error"} {
		if v, ok := payload[key]; ok {
			if isNoReservationText(fmt.Sprintf("%v", v)) {
				return errNoReservationAvailable
			}
		}
	}
	return nil
}

func isNoReservationText(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "e044") ||
		strings.Contains(text, "no_more_reservations") ||
		strings.Contains(text, "no reservation available") ||
		strings.Contains(text, "名额已满") ||
		strings.Contains(text, "已满")
}

func isHTTPStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}
