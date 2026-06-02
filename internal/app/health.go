package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/api"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"context"
	"fmt"
	"time"
)

const healthCheckInterval = 5 * time.Minute

// startHealthCheck runs a background goroutine that periodically verifies
// token validity by calling GetTimeslots. On auth failure it notifies and stops.
func startHealthCheck(ctx context.Context, client *Client, storeIDs []string) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				for _, storeID := range storeIDs {
					_, err := client.GetTimeslots(ctx, storeID)
					if err != nil {
						if isAuthError(err) {
							LogMessage(time.Now(), "健康检查：认证参数已失效")
							sendNotification("寿司郎 - 认证过期", "健康检测发现认证参数已失效，请重新打开 sushiro-overdose 重新捕获")
							DeleteLocalConfig()
							return
						}
						if fmt.Sprintf("%v", err) != "" {
							LogMessage(time.Now(), fmt.Sprintf("健康检查：%v", err))
						}
						break
					}
					break // one success is enough
				}
			}
		}
	}()

	return stop
}
