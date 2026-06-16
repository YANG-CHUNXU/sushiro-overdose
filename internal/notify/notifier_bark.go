package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// barkNotifier 走 Bark（iOS 推送 App）的自建/官方服务。
type barkNotifier struct {
	url string // Bark 服务地址，如 https://api.day.app
	key string // 设备 key，拼在路径里标识推送目标
}

func (b *barkNotifier) Name() string { return "bark" }

// Send 调用 Bark 的路径式推送接口 GET {url}/{key}/{title}/{content}，
// title/content 均做 PathEscape。HTTP >=400 视为失败。
func (b *barkNotifier) Send(ctx context.Context, title, content string) error {
	u := fmt.Sprintf("%s/%s/%s/%s",
		b.url, b.key,
		url.PathEscape(title),
		url.PathEscape(content),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	resp, err := notifierClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("bark HTTP %d", resp.StatusCode)
	}
	return nil
}
