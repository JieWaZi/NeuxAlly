package constants

//go:generate toolx gen enum ChatDeliveryFailureStage

// ChatDeliveryFailureStage 表示执行终态向 Chat 交付失败时所处的阶段。
type ChatDeliveryFailureStage string

const (
	// ChatDeliveryFailureStageFlush 表示执行终态 Flush 阶段向 Chat 交付失败。
	ChatDeliveryFailureStageFlush ChatDeliveryFailureStage = "chat_flush"
	// ChatDeliveryFailureStageLog 表示执行终态日志提交阶段向 Chat 交付失败。
	ChatDeliveryFailureStageLog ChatDeliveryFailureStage = "chat_log"
	// ChatDeliveryFailureStageSession 表示执行终态 Session 保存阶段向 Chat 交付失败。
	ChatDeliveryFailureStageSession ChatDeliveryFailureStage = "chat_session"
)
