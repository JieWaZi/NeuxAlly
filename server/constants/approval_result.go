package constants

//go:generate toolx gen enum ApprovalResult

// ApprovalResult 表示一次工具审批的权威处理结果。
type ApprovalResult string

const (
	// ApprovalResultUnspecified 表示尚未写入处理结果。
	ApprovalResultUnspecified ApprovalResult = ""
	// ApprovalResultSelected 表示用户选择了 ACP 提供的一个选项。
	ApprovalResultSelected ApprovalResult = "selected"
	// ApprovalResultCancelled 表示审批在用户选择前被取消。
	ApprovalResultCancelled ApprovalResult = "cancelled"
	// ApprovalResultInvalidated 表示执行结束或运行时断开使审批失效。
	ApprovalResultInvalidated ApprovalResult = "invalidated"
)
