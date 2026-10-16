package constants

//go:generate toolx gen enum RuntimeSessionPolicy

// RuntimeSessionPolicy 表示 Execution 对原生 Runtime Session 的续接策略。
type RuntimeSessionPolicy string

const (
	// RuntimeSessionPolicyContinue 表示允许在历史连续时续接 Runtime Session。
	RuntimeSessionPolicyContinue RuntimeSessionPolicy = "continue"
	// RuntimeSessionPolicyNew 表示本次执行强制建立新的 Runtime Session。
	RuntimeSessionPolicyNew RuntimeSessionPolicy = "new"
)
