package constants

//go:generate toolx gen enum ChatRecoveryState

// ChatRecoveryState 表示 Chat 启动恢复后 SQLite 投影与权威 JSONL 的对齐状态。
type ChatRecoveryState string

const (
	// ChatRecoveryStateUnspecified 表示尚未完成恢复检查或状态未知。
	ChatRecoveryStateUnspecified ChatRecoveryState = ""
	// ChatRecoveryStateHealthy 表示权威日志、文件索引和当前 SQLite 投影一致。
	ChatRecoveryStateHealthy ChatRecoveryState = "healthy"
	// ChatRecoveryStateRepaired 表示启动时已增量追平投影或收尾中断执行。
	ChatRecoveryStateRepaired ChatRecoveryState = "repaired"
	// ChatRecoveryStateRebuilt 表示缺失或不可信投影已从权威 JSONL 全量重建。
	ChatRecoveryStateRebuilt ChatRecoveryState = "rebuilt"
	// ChatRecoveryStateIsolated 表示该 Chat 因损坏或恢复错误被隔离。
	ChatRecoveryStateIsolated ChatRecoveryState = "isolated"
)
