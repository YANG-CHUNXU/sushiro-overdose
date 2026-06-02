package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type barkNotifier struct {
	url string
	key string
}

func (b *barkNotifier) Name() string { return "bark" }

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
