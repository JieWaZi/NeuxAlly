package constants

//go:generate toolx gen enum ElicitationResult

// ElicitationResult 表示一次结构化用户输入的权威处理结果。
type ElicitationResult string

const (
	// ElicitationResultUnspecified 表示尚未写入处理结果。
	ElicitationResultUnspecified ElicitationResult = ""
	// ElicitationResultAccepted 表示用户提交了表单内容。
	ElicitationResultAccepted ElicitationResult = "accepted"
	// ElicitationResultDeclined 表示用户明确拒绝本次输入请求。
	ElicitationResultDeclined ElicitationResult = "declined"
	// ElicitationResultCancelled 表示输入在用户提交前被取消。
	ElicitationResultCancelled ElicitationResult = "cancelled"
	// ElicitationResultInvalidated 表示执行结束或运行时断开使输入失效。
	ElicitationResultInvalidated ElicitationResult = "invalidated"
)
