package constants

//go:generate toolx gen enum ChatRecoveryIssue

// ChatRecoveryIssue 表示 Chat 启动恢复检查发现的稳定问题码。
type ChatRecoveryIssue string

const (
	// ChatRecoveryIssueLogCorruptOrRecoveryFailed 表示 JSONL 损坏或恢复流程失败。
	ChatRecoveryIssueLogCorruptOrRecoveryFailed ChatRecoveryIssue = "log_corrupt_or_recovery_failed"
	// ChatRecoveryIssueFileIndexRebuilt 表示文件索引已从权威日志重建。
	ChatRecoveryIssueFileIndexRebuilt ChatRecoveryIssue = "file_index_rebuilt"
	// ChatRecoveryIssueTailRepaired 表示权威日志尾部已完成安全修复。
	ChatRecoveryIssueTailRepaired ChatRecoveryIssue = "tail_repaired"
	// ChatRecoveryIssueChatProjectionMissing 表示 Chat SQLite 当前投影缺失。
	ChatRecoveryIssueChatProjectionMissing ChatRecoveryIssue = "chat_projection_missing"
	// ChatRecoveryIssueChatProjectionAhead 表示 Chat SQLite 投影序号领先权威日志。
	ChatRecoveryIssueChatProjectionAhead ChatRecoveryIssue = "chat_projection_ahead"
	// ChatRecoveryIssueChatProjectionContentMismatch 表示 Chat SQLite 投影内容与权威日志不一致。
	ChatRecoveryIssueChatProjectionContentMismatch ChatRecoveryIssue = "chat_projection_content_mismatch"
	// ChatRecoveryIssueChatProjectionLagging 表示 Chat SQLite 投影序号落后权威日志。
	ChatRecoveryIssueChatProjectionLagging ChatRecoveryIssue = "chat_projection_lagging"
	// ChatRecoveryIssueInterruptedExecutionCanceled 表示启动时收尾了上次中断的执行。
	ChatRecoveryIssueInterruptedExecutionCanceled ChatRecoveryIssue = "interrupted_execution_canceled"
)
