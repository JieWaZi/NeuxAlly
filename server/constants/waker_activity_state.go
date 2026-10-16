package constants

//go:generate toolx gen enum WakerActivityState

// WakerActivityState 是伙伴全部 Chat 聚合后的活动状态。
type WakerActivityState string

const (
	// WakerActivityStateIdle 表示伙伴没有运行工作或待用户处理事项。
	WakerActivityStateIdle WakerActivityState = "idle"
	// WakerActivityStateRunning 表示伙伴至少有一个 Chat 正在运行。
	WakerActivityStateRunning WakerActivityState = "running"
	// WakerActivityStateWaiting 表示伙伴至少有一个 Chat 等待审批或输入。
	WakerActivityStateWaiting WakerActivityState = "waiting"
)
