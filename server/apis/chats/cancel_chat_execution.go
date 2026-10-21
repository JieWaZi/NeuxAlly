package chats

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 取消执行
type CancelChatExecution struct {
	// MethodPost 声明停止使用显式 POST。
	ginx.MethodPost
	// 对话标识
	ChatID string `in:"path" name:"chatId" validate:"required,max=32"`
	// 执行标识
	ExecutionID string `in:"path" name:"executionId" validate:"required,max=32"`
}

// Path 返回取消指定执行接口路径。
func (c *CancelChatExecution) Path() string {
	return "/:chatId/executions/:executionId/cancel"
}

// Output 核验原执行归属并补偿停止，重试不重新绑定当前活跃执行。
func (c *CancelChatExecution) Output(ctx *gin.Context) (interface{}, error) {
	_, err := global.ChatService.ReadExecution(ctx, c.ChatID, c.ExecutionID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	propagationErr := global.WorkflowService.CancelConsumerExecution(ctx, c.ChatID, c.ExecutionID)
	if err := global.ExecutionService.CancelExecution(c.ChatID, c.ExecutionID); err != nil {
		return nil, ginx.WithStack(err)
	}
	if propagationErr != nil {
		return nil, ginx.WithStack(propagationErr)
	}
	return ginx.CommonSuccessResponse(), nil
}
