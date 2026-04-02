package main

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
		if strings.Contains(err.Error(), "E044") || strings.Contains(err.Error(), "no_more_reservations") {
			return ReservationRecord{}, errNoReservationAvailable
		}
		return ReservationRecord{}, err
	}

	var reservation ReservationRecord
	if err := json.Unmarshal(body, &reservation); err != nil {
		return ReservationRecord{}, fmt.Errorf("reservation response is not a JSON object: %w", err)
	}
	return reservation, nil
}

func (c *Client) SendFeishuCard(ctx context.Context, card map[string]any) error {
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	body, err := c.doJSON(ctx, http.MethodPost, c.settings.FeishuWebhook, map[string]string{"Content-Type": "application/json"}, payload)
	if err != nil {
		return err
	}

	var response map[string]any
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("invalid Feishu response: %w", err)
	}
	if statusCode, ok := response["StatusCode"]; ok {
		switch value := statusCode.(type) {
		case float64:
			if value != 0 {
				return fmt.Errorf("Feishu bot error: %s", stringifyJSON(response))
			}
		case string:
			if value != "" && value != "0" {
				return fmt.Errorf("Feishu bot error: %s", stringifyJSON(response))
			}
		}
	}
	return nil
}

func (c *Client) baseHeaders(authorization, contentType string) map[string]string {
	headers := map[string]string{
		"Authorization": ensureBearer(authorization),
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
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, normalizeErrorBody(responseBody))
	}
	if len(responseBody) == 0 {
		return nil, nil
	}
	if !json.Valid(responseBody) {
		return nil, fmt.Errorf("response is not JSON: %s", string(responseBody))
	}
	return responseBody, nil
}

func normalizeErrorBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		return stringifyJSON(payload)
	}
	return string(body)
}

func stringifyJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}
