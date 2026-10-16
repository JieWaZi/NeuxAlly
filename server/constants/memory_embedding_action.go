package constants

//go:generate toolx gen enum MemoryEmbeddingAction

// MemoryEmbeddingAction 是嵌入调用对应的记忆操作类型，用于记忆嵌入调用。
type MemoryEmbeddingAction string

const (
	// MemoryEmbeddingActionAdd 表示写入记忆时的嵌入调用。
	MemoryEmbeddingActionAdd MemoryEmbeddingAction = "add"
	// MemoryEmbeddingActionSearch 表示检索记忆时的嵌入调用。
	MemoryEmbeddingActionSearch MemoryEmbeddingAction = "search"
	// MemoryEmbeddingActionUpdate 表示更新记忆时的嵌入调用。
	MemoryEmbeddingActionUpdate MemoryEmbeddingAction = "update"
)
