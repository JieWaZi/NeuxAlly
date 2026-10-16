package constants

//go:generate toolx gen enum MemoryScope

// MemoryScope 表示一条记忆归属的业务范围。
type MemoryScope string

const (
	// MemoryScopeUnspecified 表示数据库或框架产生的空范围。
	MemoryScopeUnspecified MemoryScope = ""
	// MemoryScopePartner 表示伙伴自己的长期记忆，跨项目存在。
	MemoryScopePartner MemoryScope = "partner"
	// MemoryScopeProject 表示当前项目内跨对话使用的长期记忆。
	MemoryScopeProject MemoryScope = "project"
	// MemoryScopeAll 仅用于检索入参，表示同时检索伙伴与项目范围。
	MemoryScopeAll MemoryScope = "all"
)
