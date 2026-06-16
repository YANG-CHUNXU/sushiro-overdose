package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// telegramNotifier 走 Telegram Bot API。
type telegramNotifier struct {
	token  string // Bot Token（由 @BotFather 颁发）
	chatID string // 目标会话 ID（个人/群组）
}

func (t *telegramNotifier) Name() string { return "telegram" }

// Send 调用 Telegram Bot sendMessage 接口 POST https://api.telegram.org/bot{token}/sendMessage，
// 标题加粗、正文跟在后面，用 Markdown 解析。HTTP >=400 视为失败。
// 注意：正文里若含 Markdown 特殊字符可能触发解析错误，此处未做转义。
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

	resp, err := notifierClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}
