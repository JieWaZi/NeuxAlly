package constants

//go:generate toolx gen enum WorkResultOwnerKind

// WorkResultOwnerKind 标识阅读事实所属的既有工作容器。
type WorkResultOwnerKind string

const (
	// WorkResultOwnerKindUnspecified 表示未指定容器。
	WorkResultOwnerKindUnspecified WorkResultOwnerKind = ""
	// WorkResultOwnerKindChat 表示独立伙伴对话。
	WorkResultOwnerKindChat WorkResultOwnerKind = "chat"
	// WorkResultOwnerKindTeamThread 表示团队对话。
	WorkResultOwnerKindTeamThread WorkResultOwnerKind = "team_thread"
	// WorkResultOwnerKindWorkflowRun 表示顶层工作流运行。
	WorkResultOwnerKindWorkflowRun WorkResultOwnerKind = "workflow_run"
)
