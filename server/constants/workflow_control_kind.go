package constants

//go:generate toolx gen enum WorkflowControlKind

// WorkflowControlKind 表示源码画布的只读控制结构，不改变 Runner 的执行语义。
type WorkflowControlKind string

const (
	// WorkflowControlKindNode 表示一个已有身份的宿主调用。
	WorkflowControlKindNode WorkflowControlKind = "node"
	// WorkflowControlKindDecision 表示互斥判断，包括短路表达式。
	WorkflowControlKindDecision WorkflowControlKind = "decision"
	// WorkflowControlKindLoop 表示条件或逐项循环。
	WorkflowControlKindLoop WorkflowControlKind = "loop"
	// WorkflowControlKindParallel 表示独立任务分发和等待全部任务结束。
	WorkflowControlKindParallel WorkflowControlKind = "parallel"
	// WorkflowControlKindPipeline 表示每个输入独立按步骤推进。
	WorkflowControlKindPipeline WorkflowControlKind = "pipeline"
	// WorkflowControlKindReturn 表示当前主流程返回。
	WorkflowControlKindReturn WorkflowControlKind = "return"
	// WorkflowControlKindTaskReturn 表示当前任务完成，仍参加所属分组的汇合。
	WorkflowControlKindTaskReturn WorkflowControlKind = "task_return"
	// WorkflowControlKindThrow 表示抛出异常。
	WorkflowControlKindThrow WorkflowControlKind = "throw"
	// WorkflowControlKindBreak 表示离开当前循环。
	WorkflowControlKindBreak WorkflowControlKind = "break"
	// WorkflowControlKindContinue 表示进入下一轮循环。
	WorkflowControlKindContinue WorkflowControlKind = "continue"
	// WorkflowControlKindTry 表示正常路径和异常处理路径。
	WorkflowControlKindTry WorkflowControlKind = "try"
	// WorkflowControlKindJoinWorkflow 表示显式等待此前启动的子流程。
	WorkflowControlKindJoinWorkflow WorkflowControlKind = "join_workflow"
)
