package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type feishuNotifier struct {
	webhook string
}

func (f *feishuNotifier) Name() string { return "feishu" }

func (f *feishuNotifier) Send(ctx context.Context, title, content string) error {
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": title},
			"template": "green",
		},
		"elements": []map[string]any{
			{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}},
			{"tag": "note", "elements": []map[string]any{
				{"tag": "plain_text", "content": time.Now().Format("2006-01-02 15:04:05")},
			}},
		},
	}
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.webhook, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("feishu HTTP %d", resp.StatusCode)
	}
	return nil
}
