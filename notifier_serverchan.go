package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type serverChanNotifier struct {
	key string
}

func (s *serverChanNotifier) Name() string { return "serverchan" }

func (s *serverChanNotifier) Send(ctx context.Context, title, content string) error {
	payload := map[string]any{
		"title": title,
		"desp":  content,
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", s.key)
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
		return fmt.Errorf("serverchan HTTP %d", resp.StatusCode)
	}
	return nil
}
