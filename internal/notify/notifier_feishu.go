package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// feishuNotifier 走飞书自定义机器人 webhook。
type feishuNotifier struct {
	webhook string // 飞书机器人的完整 webhook 地址（含 token）
}

func (f *feishuNotifier) Name() string { return "feishu" }

// Send 向飞书 webhook POST 一张 interactive 卡片消息：标题放 header（绿色模板），
// 正文走 lark_md 渲染 markdown，并附当前时间作为 note。HTTP >=400 视为失败。
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

	resp, err := notifierClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("feishu HTTP %d", resp.StatusCode)
	}
	return nil
}

// NewFeishuNotifier 构造飞书通知器（供外部包用）。
func NewFeishuNotifier(webhook string) Notifier { return &feishuNotifier{webhook: webhook} }
