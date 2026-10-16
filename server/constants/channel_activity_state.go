package constants

//go:generate toolx gen enum ChannelActivityState

// ChannelActivityState 是团队列表聚合全部已就绪对话后的活动状态。
type ChannelActivityState string

const (
	// ChannelActivityStateIdle 表示团队没有运行工作或待用户处理事项。
	ChannelActivityStateIdle ChannelActivityState = "idle"
	// ChannelActivityStateRunning 表示团队至少有一个对话仍在运行或排队。
	ChannelActivityStateRunning ChannelActivityState = "running"
	// ChannelActivityStateWaiting 表示团队至少有一个对话等待审批、输入或验收。
	ChannelActivityStateWaiting ChannelActivityState = "waiting"
)
