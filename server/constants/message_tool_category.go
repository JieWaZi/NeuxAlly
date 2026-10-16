package constants

//go:generate toolx gen enum MessageToolCategory

// MessageToolCategory 表示跨 Runtime 统一的工具展示类别。
type MessageToolCategory string

const (
	// MessageToolCategoryFileRead 表示文件读取工具。
	MessageToolCategoryFileRead MessageToolCategory = "file_read"
	// MessageToolCategoryFileSearch 表示文件搜索工具。
	MessageToolCategoryFileSearch MessageToolCategory = "file_search"
	// MessageToolCategoryFileModify 表示文件修改工具。
	MessageToolCategoryFileModify MessageToolCategory = "file_modify"
	// MessageToolCategoryCommand 表示命令执行工具。
	MessageToolCategoryCommand MessageToolCategory = "command"
	// MessageToolCategoryNetwork 表示网络访问工具。
	MessageToolCategoryNetwork MessageToolCategory = "network"
	// MessageToolCategoryMCP 表示 MCP 或外部服务工具。
	MessageToolCategoryMCP MessageToolCategory = "mcp"
	// MessageToolCategorySkill 表示加载或使用技能。
	MessageToolCategorySkill MessageToolCategory = "skill"
	// MessageToolCategoryOther 表示尚未归入已知类别的工具。
	MessageToolCategoryOther MessageToolCategory = "other"
)
