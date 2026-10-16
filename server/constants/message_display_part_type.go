package constants

//go:generate toolx gen enum MessageDisplayPartType

// MessageDisplayPartType 表示消息展示投影中的部件类型。
type MessageDisplayPartType string

const (
	// MessageDisplayPartText 表示消息展示投影中的可见文本部件。
	MessageDisplayPartText MessageDisplayPartType = "text"
	// MessageDisplayPartProcessCommentary 表示工作过程内的过程说明部件。
	MessageDisplayPartProcessCommentary MessageDisplayPartType = "process_commentary"
	// MessageDisplayPartReasoning 表示消息展示投影中的思考部件。
	MessageDisplayPartReasoning MessageDisplayPartType = "reasoning"
	// MessageDisplayPartTool 表示消息展示投影中的工具活动部件。
	MessageDisplayPartTool MessageDisplayPartType = "tool"
)
