package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// notifierClient is shared by all notifiers with a 10s timeout.
var notifierClient = &http.Client{Timeout: 10 * time.Second}

// Notifier sends a notification with a title and markdown content.
type Notifier interface {
	Send(ctx context.Context, title, content string) error
	Name() string
}

// MultiNotifier fans out notifications to multiple channels.
type MultiNotifier struct {
	mu        sync.Mutex
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) Send(ctx context.Context, title, content string) {
	m.mu.Lock()
	snapshot := make([]Notifier, len(m.notifiers))
	copy(snapshot, m.notifiers)
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, n := range snapshot {
		wg.Add(1)
		go func(n Notifier) {
			defer wg.Done()
			if err := n.Send(ctx, title, content); err != nil {
				logMessage(time.Now(), fmt.Sprintf("[%s] 通知发送失败: %s", n.Name(), err))
			}
		}(n)
	}
	wg.Wait()
}

func (m *MultiNotifier) Add(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, n)
}

func (m *MultiNotifier) List() []Notifier {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Notifier, len(m.notifiers))
	copy(out, m.notifiers)
	return out
}

// ---- Notification config ----

type notifyConfig struct {
	Feishu struct {
		Webhook string `json:"webhook"`
	} `json:"feishu"`
	Telegram struct {
		Token  string `json:"token"`
		ChatID string `json:"chat_id"`
	} `json:"telegram"`
	Bark struct {
		URL string `json:"url"`
		Key string `json:"key"`
	} `json:"bark"`
	ServerChan struct {
		Key string `json:"key"`
	} `json:"server_chan"`
}

func notifyConfigPath() string {
	return fmt.Sprintf("%s/notify.json", appDirPath())
}

func loadNotifyConfig() (*notifyConfig, error) {
	data, err := os.ReadFile(notifyConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg notifyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveNotifyConfig(cfg *notifyConfig) error {
	os.MkdirAll(appDirPath(), 0o755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(notifyConfigPath(), data, 0o600)
}

// BuildNotifierFromConfig creates a MultiNotifier from saved config.
func BuildNotifierFromConfig() *MultiNotifier {
	mn := &MultiNotifier{}

	cfg, err := loadNotifyConfig()
	if err != nil {
		// Try loading legacy feishu config
		if webhook := loadFeishuConfig(); webhook != "" {
			mn.Add(&feishuNotifier{webhook: webhook})
		}
		return mn
	}

	if cfg.Feishu.Webhook != "" {
		mn.Add(&feishuNotifier{webhook: cfg.Feishu.Webhook})
	}
	if cfg.Telegram.Token != "" && cfg.Telegram.ChatID != "" {
		mn.Add(&telegramNotifier{token: cfg.Telegram.Token, chatID: cfg.Telegram.ChatID})
	}
	if cfg.Bark.URL != "" && cfg.Bark.Key != "" {
		mn.Add(&barkNotifier{url: cfg.Bark.URL, key: cfg.Bark.Key})
	}
	if cfg.ServerChan.Key != "" {
		mn.Add(&serverChanNotifier{key: cfg.ServerChan.Key})
	}

	return mn
}
