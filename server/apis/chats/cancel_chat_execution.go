package chats

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 取消执行
type CancelChatExecution struct {
	ginx.MethodPost
	// 对话标识
	ChatID string `in:"path" name:"chatId" validate:"required"`
}

// Path 返回取消当前执行接口路径。
func (c *CancelChatExecution) Path() string {
	return "/:chatId/cancel"
}

// Output 触发当前 Runtime Invocation 取消。
func (c *CancelChatExecution) Output(ctx *gin.Context) (interface{}, error) {
	summary, err := global.ChatService.GetChatSummary(ctx, c.ChatID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	executionID := summary.Chat.ActiveExecutionID
	if executionID == "" {
		return ginx.CommonSuccessResponse(), nil
	}
	propagationErr := global.WorkflowService.CancelConsumerExecution(ctx, c.ChatID, executionID)
	if err := global.ExecutionService.CancelExecution(c.ChatID, executionID); err != nil {
		return nil, ginx.WithStack(err)
	}
	if propagationErr != nil {
		return nil, ginx.WithStack(propagationErr)
	}
	return ginx.CommonSuccessResponse(), nil
}
