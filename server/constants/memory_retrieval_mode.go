package constants

//go:generate toolx gen enum MemoryRetrievalMode

// MemoryRetrievalMode 表示记忆检索当前实际使用的召回方式。
type MemoryRetrievalMode string

const (
	// MemoryRetrievalModeUnspecified 表示数据库或框架产生的空模式。
	MemoryRetrievalModeUnspecified MemoryRetrievalMode = ""
	// MemoryRetrievalModeSemantic 表示已配置 Embedding，使用 sqlite-vec 语义 KNN 召回。
	MemoryRetrievalModeSemantic MemoryRetrievalMode = "semantic"
	// MemoryRetrievalModeKeyword 表示未配置 Embedding 或语义索引冲突，使用关键词召回。
	MemoryRetrievalModeKeyword MemoryRetrievalMode = "keyword"
)
