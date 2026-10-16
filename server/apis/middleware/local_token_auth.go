// Package middleware 定义 NeuxAlly Core 的 HTTP 中间件。
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shrewx/ginx"
)

// LocalTokenAuth 校验桌面进程启动 Core 时协商的本地令牌。
type LocalTokenAuth struct {
	// MiddlewareType 声明该 Operator 是 ginx 中间件。
	ginx.MiddlewareType
}

// Output 返回使用常量时间比较的 Bearer Token 中间件。
func (a *LocalTokenAuth) Output(*gin.Context) (interface{}, error) {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		expected := global.Config.Security.LocalToken
		provided, found := strings.CutPrefix(
			ctx.GetHeader("Authorization"),
			"Bearer ",
		)
		// 仅固定事件升级入口接受浏览器可设置的子协议令牌，普通 REST 仍要求 Bearer。
		if !found && ctx.Request.Method == http.MethodGet && ctx.Request.URL.Path == "/api/v1/events" && websocket.IsWebSocketUpgrade(ctx.Request) {
			for _, protocol := range strings.Split(ctx.GetHeader("Sec-WebSocket-Protocol"), ",") {
				if token, ok := strings.CutPrefix(strings.TrimSpace(protocol), "ally.auth."); ok {
					provided = token
					found = true
					break
				}
			}
		}
		if !found ||
			expected == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "本地认证失败",
			})
			return
		}
		if ctx.Request.Method == http.MethodPost && strings.HasPrefix(ctx.Request.URL.Path, "/api/v1/workflows/") && strings.HasSuffix(ctx.Request.URL.Path, "/files") {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, constants.WorkflowFileMaximumBytes+(1<<20))
		}
		ctx.Next()
	}), nil
}
