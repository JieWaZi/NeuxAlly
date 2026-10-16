package constants

//go:generate toolx gen enum AutomaticTaskTargetKind

// AutomaticTaskTargetKind 区分自动任务的唯一执行目标。
type AutomaticTaskTargetKind string

const (
	// AutomaticTaskTargetKindUnspecified 由既有目标字段在保存时确定类型。
	AutomaticTaskTargetKindUnspecified AutomaticTaskTargetKind = ""
	// AutomaticTaskTargetKindWaker 启动独立伙伴的标准执行。
	AutomaticTaskTargetKindWaker AutomaticTaskTargetKind = "waker"
	// AutomaticTaskTargetKindTeam 启动团队首发。
	AutomaticTaskTargetKindTeam AutomaticTaskTargetKind = "team"
	// AutomaticTaskTargetKindWorkflow 启动冻结版本的工作流。
	AutomaticTaskTargetKindWorkflow AutomaticTaskTargetKind = "workflow"
)
