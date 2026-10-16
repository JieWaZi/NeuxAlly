package constants

//go:generate toolx gen enum MemoryEvent

// MemoryEvent 表示记忆历史记录中的变更事件类型。
type MemoryEvent string

const (
	// MemoryEventUnspecified 表示数据库或框架产生的空事件。
	MemoryEventUnspecified MemoryEvent = ""
	// MemoryEventAdd 表示新增一条记忆。
	MemoryEventAdd MemoryEvent = "ADD"
	// MemoryEventUpdate 表示更新一条记忆的正文。
	MemoryEventUpdate MemoryEvent = "UPDATE"
	// MemoryEventDelete 表示遗忘一条记忆，当前记录物理删除后保留该历史。
	MemoryEventDelete MemoryEvent = "DELETE"
)
