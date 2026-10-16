package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 执行获准能力命令
type Command struct {
	// MethodPost 声明本接口以 POST 接纳命令。
	ginx.MethodPost
}

// Path 将命令子路径交给同一个执行协议处理器。
func (*Command) Path() string { return "/*path" }

// Output 使用执行令牌处理请求并直接写入单个协议信封。
func (*Command) Output(ctx *gin.Context) (interface{}, error) { serve(ctx); return nil, nil }
