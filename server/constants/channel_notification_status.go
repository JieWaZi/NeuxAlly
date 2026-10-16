package constants

//go:generate toolx gen enum ChannelNotificationStatus

// ChannelNotificationStatus 是成员结果通知团队负责人的投递状态。
type ChannelNotificationStatus string

const (
	// ChannelNotificationStatusPending 表示通知已持久化但尚未交给负责人 Chat。
	ChannelNotificationStatusPending ChannelNotificationStatus = "pending"
	// ChannelNotificationStatusDelivering 表示通知已进入一次负责人汇总执行。
	ChannelNotificationStatusDelivering ChannelNotificationStatus = "delivering"
	// ChannelNotificationStatusConsumed 表示负责人 Chat 已经接收该通知。
	ChannelNotificationStatusConsumed ChannelNotificationStatus = "consumed"
)
