package constants

//go:generate toolx gen enum NotificationKind

// NotificationKind 标识由原业务事实派生的用户关注事项。
type NotificationKind string

const (
	// NotificationKindUnspecified 表示未指定通知类型。
	NotificationKindUnspecified NotificationKind = ""
	// NotificationKindApproval 表示原工具审批或工作流操作许可。
	NotificationKindApproval NotificationKind = "approval"
	// NotificationKindInput 表示原结构化输入或工作流人工请求。
	NotificationKindInput NotificationKind = "input"
	// NotificationKindReview 表示当前围栏的团队人工验收。
	NotificationKindReview NotificationKind = "review"
	// NotificationKindResult 表示原工作容器最新未确认成果。
	NotificationKindResult NotificationKind = "result"
	// NotificationKindAutomaticFailure 表示没有原成果覆盖的当前自动失败。
	NotificationKindAutomaticFailure NotificationKind = "automatic_failure"
)
