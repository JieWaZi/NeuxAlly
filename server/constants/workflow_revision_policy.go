package constants

//go:generate toolx gen enum WorkflowRevisionPolicy

// WorkflowRevisionPolicy 指定触发机会接纳时如何选择工作流版本。
type WorkflowRevisionPolicy string

const (
	// WorkflowRevisionPolicyUnspecified 表示尚未选择版本策略。
	WorkflowRevisionPolicyUnspecified WorkflowRevisionPolicy = ""
	// WorkflowRevisionPolicyCurrent 在领取机会时冻结当前版本。
	WorkflowRevisionPolicyCurrent WorkflowRevisionPolicy = "current"
	// WorkflowRevisionPolicyPinned 始终使用明确指定的不可变版本。
	WorkflowRevisionPolicyPinned WorkflowRevisionPolicy = "pinned"
)
