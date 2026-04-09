package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type telegramNotifier struct {
	token  string
	chatID string
}

func (t *telegramNotifier) Name() string { return "telegram" }

func (t *telegramNotifier) Send(ctx context.Context, title, content string) error {
	text := fmt.Sprintf("*%s*\n\n%s", title, content)
	payload := map[string]any{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
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
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}
