package automatic_tasks

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 待处理自动运行
type ListAutomaticTaskAttention struct {
	// 请求方法
	ginx.MethodGet
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
}

// Path 返回自动运行待处理查询路径。
func (*ListAutomaticTaskAttention) Path() string { return "/attention" }

// Output 返回供概览及现有审批监视器使用的窄查询投影。
func (o *ListAutomaticTaskAttention) Output(ctx *gin.Context) (interface{}, error) {
	result, err := currentTaskAPI().GetAttention(ctx, o.WakerID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
