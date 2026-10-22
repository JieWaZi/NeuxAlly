// Package logger 将上游 chunker 的诊断端口适配到标准库，不引入宿主日志模块。
package logger

import (
	"context"
	"fmt"
	"log/slog"
)

// Debugf 保留上游诊断消息和上下文，日志级别由进程的标准日志处理器控制。
func Debugf(ctx context.Context, format string, args ...any) {
	slog.DebugContext(ctx, fmt.Sprintf(format, args...))
}
