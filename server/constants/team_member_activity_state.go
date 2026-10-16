package constants

//go:generate toolx gen enum TeamMemberActivityState

// TeamMemberActivityState 是 Team 快照中成员 Chat 的权威活动状态。
type TeamMemberActivityState string

const (
	// TeamMemberActivityStateIdle 表示成员 Chat 当前没有活跃执行或待处理交互。
	TeamMemberActivityStateIdle TeamMemberActivityState = "idle"
	// TeamMemberActivityStateRunning 表示成员 Chat 当前存在活跃执行。
	TeamMemberActivityStateRunning TeamMemberActivityState = "running"
	// TeamMemberActivityStateRequiresAction 表示成员 Chat 正等待用户审批或结构化输入。
	TeamMemberActivityStateRequiresAction TeamMemberActivityState = "requires_action"
)
