package chat

import (
	"context"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"

	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// InitialExecutionState 是产品入口对账首次接纳与运行结果的窄权威视图。
type InitialExecutionState struct {
	// Chat 是当前 Chat 的身份及待处理交互事实。
	Chat models.Chat
	// CreateRequestID 是首发时提交的稳定请求身份。
	CreateRequestID string
	// InitialExecution 是首次 Execution 的当前或最终状态，后续续聊不覆盖它。
	InitialExecution *models.Execution
}

// ReadExecution 从同一权威重放结果读取指定回合，不把后续回合误当成首次接纳。
func (m *service) ReadExecution(_ context.Context, chatID, executionID string) (models.Execution, error) {
	_, state, err := m.loadChatState(chatID)
	if err != nil {
		return models.Execution{}, err
	}
	value := state.executions[executionID]
	if value == nil {
		return models.Execution{}, status_error.ExecutionNotFound
	}
	result := *value
	if value.CompletedAt != nil {
		completed := *value.CompletedAt
		result.CompletedAt = &completed
	}
	return result, nil
}

// ReadInitialExecution 复用 Chat 权威重放，仅返回首次执行和交互状态。
func (m *service) ReadInitialExecution(_ context.Context, chatID string) (InitialExecutionState, error) {
	_, state, err := m.loadChatState(chatID)
	if err != nil {
		return InitialExecutionState{}, err
	}
	return InitialExecutionState{Chat: state.chat, CreateRequestID: state.createRequestID, InitialExecution: state.initialExecution}, nil
}
