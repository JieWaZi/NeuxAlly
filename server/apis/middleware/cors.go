package middleware

import (
	"net/http"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/gin-gonic/gin"
)

// CORS 返回桌面 Renderer 跨源访问本地 Core 所需的响应头中间件。
func CORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Headers", constants.LocalCORSAllowedHeaders)
		ctx.Header("Access-Control-Allow-Methods", constants.LocalCORSAllowedMethods)

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
