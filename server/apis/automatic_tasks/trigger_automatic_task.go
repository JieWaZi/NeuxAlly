package automatic_tasks

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 自动任务运行请求
type TriggerAutomaticTaskRequest struct {
	// 工作流参数
	Args map[string]any `json:"args,omitempty"`
	// 请求标识
	ClientRequestID string `json:"clientRequestId" validate:"required,max=64"`
}

// 立即运行自动任务
type TriggerAutomaticTask struct {
	// 请求方法
	ginx.MethodPost
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 任务标识
	TaskID string `in:"path" name:"taskId" validate:"required"`
	// 请求体
	Body TriggerAutomaticTaskRequest `in:"body"`
}

// Path 返回操作的相对路由。
func (t *TriggerAutomaticTask) Path() string { return "/:taskId/runs" }

// Output 完成校验后的任务管理请求。
func (o *TriggerAutomaticTask) Output(ctx *gin.Context) (interface{}, error) {
	result, err := currentTaskAPI().TriggerManual(ctx, o.WakerID, o.TaskID, o.Body.ClientRequestID, o.Body.Args)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	// 立即运行需要返回运行及 Chat 的稳定身份，便于打开历史和网络重放。
	return result, nil
}
