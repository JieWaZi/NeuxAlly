package constants

//go:generate toolx gen enum IMChatKind

// IMChatKind 定义 IM 集成的稳定业务值。
type IMChatKind string

const (
	// IMChatKindUnspecified 表示尚无聊天类型。
	IMChatKindUnspecified IMChatKind = ""
	// IMChatKindDirect 表示平台单聊。
	IMChatKindDirect IMChatKind = "direct"
	// IMChatKindGroup 表示平台群聊。
	IMChatKindGroup IMChatKind = "group"
)
