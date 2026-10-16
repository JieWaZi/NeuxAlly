package constants

//go:generate toolx gen enum MCPTestStatus

// MCPTestStatus 表示最近一次 MCP 连接测试的状态。
type MCPTestStatus string

const (
	// MCPTestStatusUntested 表示连接器当前配置尚未完成测试。
	MCPTestStatusUntested MCPTestStatus = "untested"
	// MCPTestStatusSucceeded 表示最近一次连接测试成功。
	MCPTestStatusSucceeded MCPTestStatus = "succeeded"
	// MCPTestStatusFailed 表示最近一次连接测试失败。
	MCPTestStatusFailed MCPTestStatus = "failed"
)
