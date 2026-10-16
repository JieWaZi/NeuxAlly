package automatic_tasks

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 自动任务启停请求
type SetAutomaticTaskEnabledRequest struct {
	// 启停状态
	Enabled bool `json:"enabled"`
	// 配置版本
	Revision int64 `json:"revision" validate:"required,min=1"`
}

// 启停自动任务
type SetAutomaticTaskEnabled struct {
	// 请求方法
	ginx.MethodPut
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 任务标识
	TaskID string `in:"path" name:"taskId" validate:"required"`
	// 请求体
	Body SetAutomaticTaskEnabledRequest `in:"body"`
}

// Path 返回操作的相对路由。
func (s *SetAutomaticTaskEnabled) Path() string { return "/:taskId/enabled" }

// Output 完成校验后的任务管理请求。
func (o *SetAutomaticTaskEnabled) Output(ctx *gin.Context) (interface{}, error) {
	if err := currentTaskAPI().SetEnabled(ctx, o.WakerID, o.TaskID, o.Body.Revision, o.Body.Enabled); err != nil {
		return nil, ginx.WithStack(err)
	}
	return ginx.CommonSuccessResponse(), nil
}
