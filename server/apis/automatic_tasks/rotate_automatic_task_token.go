package automatic_tasks

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 任务密钥轮换请求
type RotateAutomaticTaskTokenRequest struct {
	// 配置版本
	Revision int64 `json:"revision" validate:"required,min=1"`
}

// 轮换任务密钥
type RotateAutomaticTaskToken struct {
	// 请求方法
	ginx.MethodPost
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 任务标识
	TaskID string `in:"path" name:"taskId" validate:"required"`
	// 请求体
	Body RotateAutomaticTaskTokenRequest `in:"body"`
}

// Path 返回操作的相对路由。
func (r *RotateAutomaticTaskToken) Path() string { return "/:taskId/token" }

// Output 完成校验后的任务管理请求。
func (o *RotateAutomaticTaskToken) Output(ctx *gin.Context) (interface{}, error) {
	result, err := currentTaskAPI().RotateToken(ctx, o.WakerID, o.TaskID, o.Body.Revision)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	// 轮换必须只在这次响应中返回明文密钥。
	return result, nil
}
