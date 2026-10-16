package constants

//go:generate toolx gen enum MCPActivationPolicy

// MCPActivationPolicy 描述连接器启用后何时实际挂载到 Runtime。
type MCPActivationPolicy string

const (
	// MCPActivationPolicyPerExecution 表示每轮执行按伙伴启用状态和 Runtime 能力挂载。
	MCPActivationPolicyPerExecution MCPActivationPolicy = "per_execution"
	// MCPActivationPolicyTeamContext 表示仅在 Team 执行中按负责人、成员和只读角色挂载。
	MCPActivationPolicyTeamContext MCPActivationPolicy = "team_context"
	// MCPActivationPolicyWorkflowContext 表示由工作流入口按当前执行签发，伙伴不能独立启停。
	MCPActivationPolicyWorkflowContext MCPActivationPolicy = "workflow_context"
)
