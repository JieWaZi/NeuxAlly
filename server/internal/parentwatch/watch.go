// Package parentwatch 监控桌面父进程是否仍然存活。
package parentwatch

import (
	"context"
	"time"
)

// Wait 阻塞到父进程消失或调用方取消；父进程消失时返回 true。
func Wait(ctx context.Context, parentPID int, interval time.Duration) bool {
	if parentPID <= 0 {
		return true
	}
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if !processExists(parentPID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
