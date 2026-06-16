package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// serverChanNotifier 走 Server酱（ServerChan），通过微信"方糖"服务号推送。
type serverChanNotifier struct {
	key string // Server酱 SendKey，同时是 URL 路径的一部分
}

func (s *serverChanNotifier) Name() string { return "serverchan" }

// Send 调用 Server酱接口 POST https://sctapi.ftqq.com/{key}.send，
// title 作消息标题、desp 作正文（支持 markdown）。HTTP >=400 视为失败。
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

	resp, err := notifierClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("serverchan HTTP %d", resp.StatusCode)
	}
	return nil
}
