package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// recordingTransport 记录所有请求的 endpoint，并对 sendMessage 返回一个 message_id。
type recordingTransport struct {
	mu        sync.Mutex
	endpoints []string
	notMod    bool // editMessageText 是否返回「not modified」
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	path := req.URL.Path
	ep := path[strings.LastIndex(path, "/")+1:]
	rt.endpoints = append(rt.endpoints, ep)
	if req.Body != nil {
		_, _ = io.ReadAll(req.Body)
	}
	if ep == "sendMessage" {
		return resp(200, `{"ok":true,"result":{"message_id":555}}`), nil
	}
	// editMessageText
	if rt.notMod {
		return resp(400, `{"ok":false,"description":"Bad Request: message is not modified"}`), nil
	}
	return resp(200, `{"ok":true,"result":{"message_id":555}}`), nil
}

func resp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func withRecording(t *testing.T, rt *recordingTransport) {
	t.Helper()
	old := notifierClient.Transport
	notifierClient.Transport = rt
	t.Cleanup(func() { notifierClient.Transport = old })
}

func clearTelegramSessions() {
	telegramSessions.mu.Lock()
	telegramSessions.m = map[string]int{}
	telegramSessions.mu.Unlock()
}

func TestTelegramSessionEditsInPlace(t *testing.T) {
	clearTelegramSessions()
	rt := &recordingTransport{}
	withRecording(t, rt)

	n := &telegramNotifier{token: "tok", chatID: "chat"}
	key := "called|STORE|1078"
	if err := n.SendSession(context.Background(), key, "🔔", "还差 12 桌"); err != nil {
		t.Fatalf("first SendSession err: %v", err)
	}
	if err := n.SendSession(context.Background(), key, "🔔", "还差 5 桌"); err != nil {
		t.Fatalf("second SendSession err: %v", err)
	}
	if err := n.SendSession(context.Background(), key, "🔔", "还差 1 桌"); err != nil {
		t.Fatalf("third SendSession err: %v", err)
	}

	// 期望：第一次 sendMessage，后两次 editMessageText（原地更新）。
	want := []string{"sendMessage", "editMessageText", "editMessageText"}
	if len(rt.endpoints) != len(want) {
		t.Fatalf("endpoints=%v want %v", rt.endpoints, want)
	}
	for i := range want {
		if rt.endpoints[i] != want[i] {
			t.Fatalf("endpoint[%d]=%s want %s (all=%v)", i, rt.endpoints[i], want[i], rt.endpoints)
		}
	}
}

func TestTelegramSessionNotModifiedIsSuccess(t *testing.T) {
	clearTelegramSessions()
	rt := &recordingTransport{notMod: true}
	withRecording(t, rt)

	n := &telegramNotifier{token: "tok", chatID: "chat"}
	key := "called|S|1"
	_ = n.SendSession(context.Background(), key, "t", "same")
	// 第二次 edit 返回 not modified，应被吞掉为成功、且不退化为再发新消息。
	if err := n.SendSession(context.Background(), key, "t", "same"); err != nil {
		t.Fatalf("not-modified 应视为成功: %v", err)
	}
	want := []string{"sendMessage", "editMessageText"}
	if strings.Join(rt.endpoints, ",") != strings.Join(want, ",") {
		t.Fatalf("endpoints=%v want %v（不应退化为再次 sendMessage）", rt.endpoints, want)
	}
}

func TestMultiNotifierSendSessionFallsBack(t *testing.T) {
	clearTelegramSessions()
	rt := &recordingTransport{}
	withRecording(t, rt)
	// 飞书不实现 SessionNotifier → 应回退普通 Send（走 feishu webhook，不属于 telegram endpoint）。
	mn := NewMultiNotifier(&feishuNotifier{webhook: "https://example.com/feishu"}, &telegramNotifier{token: "tok", chatID: "c"})
	mn.SendSession(context.Background(), "k", "标题", "正文")
	// telegram 应走 sendMessage（首条会话）。
	found := false
	for _, e := range rt.endpoints {
		if e == "sendMessage" {
			found = true
		}
	}
	if !found {
		t.Fatalf("telegram 应通过 sendMessage 发送，endpoints=%v", rt.endpoints)
	}
}
