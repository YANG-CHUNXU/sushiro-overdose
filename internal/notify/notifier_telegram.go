package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// telegramNotifier 走 Telegram Bot API。
type telegramNotifier struct {
	token  string // Bot Token（由 @BotFather 颁发）
	chatID string // 目标会话 ID（个人/群组）
}

func (t *telegramNotifier) Name() string { return "telegram" }

// telegramSessions 是「会话 → 消息 ID」的进程级映射，供 SendSession 原地更新。
// 做成包级而非挂在 notifier 实例上，是因为 sendQueueAlert 每次都 BuildNotifierFromConfig() 新建实例——
// 状态必须跨实例存活才能持续更新同一条消息。key = token|chatID|sessionKey。
var telegramSessions = struct {
	mu sync.Mutex
	m  map[string]int
}{m: map[string]int{}}

func (t *telegramNotifier) sessionStoreKey(sessionKey string) string {
	return t.token + "|" + t.chatID + "|" + sessionKey
}

// formatText 标题加粗 + 正文，Markdown 解析。
func (t *telegramNotifier) formatText(title, content string) string {
	return fmt.Sprintf("*%s*\n\n%s", title, content)
}

// Send 调用 sendMessage 推一条新消息。
func (t *telegramNotifier) Send(ctx context.Context, title, content string) error {
	_, err := t.sendMessage(ctx, t.formatText(title, content))
	return err
}

// SendSession 同一 sessionKey 复用同一条消息：已有 message_id 则 editMessageText 原地更新，
// 没有或编辑失败（消息过旧/被删）则发新消息并记下 id。「内容未变化」视为成功（消息已是最新）。
func (t *telegramNotifier) SendSession(ctx context.Context, sessionKey, title, content string) error {
	text := t.formatText(title, content)
	storeKey := t.sessionStoreKey(sessionKey)

	telegramSessions.mu.Lock()
	msgID, has := telegramSessions.m[storeKey]
	telegramSessions.mu.Unlock()

	if has {
		err := t.editMessage(ctx, msgID, text)
		if err == nil || errIsTelegramNotModified(err) {
			return nil
		}
		// 编辑失败（消息太旧/已删）→ 退化为发新消息。
	}

	newID, err := t.sendMessage(ctx, text)
	if err != nil {
		return err
	}
	telegramSessions.mu.Lock()
	telegramSessions.m[storeKey] = newID
	telegramSessions.mu.Unlock()
	return nil
}

// sendMessage POST sendMessage，返回新消息的 message_id。
func (t *telegramNotifier) sendMessage(ctx context.Context, text string) (int, error) {
	payload := map[string]any{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	body, err := t.doAPI(ctx, "sendMessage", payload)
	if err != nil {
		return 0, err
	}
	return parseTelegramMessageID(body), nil
}

// editMessage POST editMessageText 原地更新。HTTP>=400 返回错误（含 body 供「未修改」判定）。
func (t *telegramNotifier) editMessage(ctx context.Context, messageID int, text string) error {
	payload := map[string]any{
		"chat_id":    t.chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	_, err := t.doAPI(ctx, "editMessageText", payload)
	return err
}

// doAPI 发一次 Bot API 调用，HTTP>=400 时把响应体并入错误，便于上层判定「未修改」。
func (t *telegramNotifier) doAPI(ctx context.Context, method string, payload map[string]any) ([]byte, error) {
	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := notifierClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("telegram %s HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

// parseTelegramMessageID 从 {"ok":true,"result":{"message_id":123,...}} 取 message_id。
func parseTelegramMessageID(body []byte) int {
	var parsed struct {
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		return parsed.Result.MessageID
	}
	return 0
}

// errIsTelegramNotModified：editMessageText 在内容相同时返回「message is not modified」——
// 这等价于「已经是最新」，不该退化成发新消息。
func errIsTelegramNotModified(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not modified")
}
