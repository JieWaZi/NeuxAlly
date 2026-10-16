package automatic_tasks

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 删除自动任务
type DeleteAutomaticTask struct {
	// 请求方法
	ginx.MethodDelete
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 任务标识
	TaskID string `in:"path" name:"taskId" validate:"required"`
}

// Path 返回操作的相对路由。
func (d *DeleteAutomaticTask) Path() string { return "/:taskId" }

// Output 完成校验后的任务管理请求。
func (o *DeleteAutomaticTask) Output(ctx *gin.Context) (interface{}, error) {
	if err := currentTaskAPI().Delete(ctx, o.WakerID, o.TaskID); err != nil {
		return nil, ginx.WithStack(err)
	}
	return ginx.CommonSuccessResponse(), nil
}
