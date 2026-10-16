package constants

//go:generate toolx gen enum MessageToolAction

// MessageToolAction 表示从结构化工具事实派生的安全语义动作。
type MessageToolAction string

const (
	// MessageToolActionReadFile 表示读取一个已知项目文件。
	MessageToolActionReadFile MessageToolAction = "read_file"
	// MessageToolActionListFiles 表示浏览项目文件或目录。
	MessageToolActionListFiles MessageToolAction = "list_files"
	// MessageToolActionSearchFiles 表示在项目中搜索文件或内容。
	MessageToolActionSearchFiles MessageToolAction = "search_files"
	// MessageToolActionWriteFile 表示创建或完整写入文件。
	MessageToolActionWriteFile MessageToolAction = "write_file"
	// MessageToolActionModifyFile 表示修改已有文件。
	MessageToolActionModifyFile MessageToolAction = "modify_file"
	// MessageToolActionRunTests 表示运行项目测试。
	MessageToolActionRunTests MessageToolAction = "run_tests"
	// MessageToolActionCheckProject 表示执行类型、Lint 或静态检查。
	MessageToolActionCheckProject MessageToolAction = "check_project"
	// MessageToolActionBuildProject 表示构建项目产物。
	MessageToolActionBuildProject MessageToolAction = "build_project"
	// MessageToolActionFormatCode 表示格式化项目代码。
	MessageToolActionFormatCode MessageToolAction = "format_code"
	// MessageToolActionInspectGit 表示读取 Git 状态或变更。
	MessageToolActionInspectGit MessageToolAction = "inspect_git"
	// MessageToolActionExecuteCommand 表示无法进一步安全归类的命令执行。
	MessageToolActionExecuteCommand MessageToolAction = "execute_command"
	// MessageToolActionFetchWeb 表示读取一个网页或网络资源。
	MessageToolActionFetchWeb MessageToolAction = "fetch_web"
	// MessageToolActionSearchWeb 表示执行网络搜索。
	MessageToolActionSearchWeb MessageToolAction = "search_web"
	// MessageToolActionCallService 表示调用 MCP 或其他外部服务。
	MessageToolActionCallService MessageToolAction = "call_service"
	// MessageToolActionUseSkill 表示加载并使用技能指令。
	MessageToolActionUseSkill MessageToolAction = "use_skill"
	// MessageToolActionViewImage 表示读取并查看项目图片。
	MessageToolActionViewImage MessageToolAction = "view_image"
	// MessageToolActionOther 表示尚未归入已知语义的操作。
	MessageToolActionOther MessageToolAction = "other"
)
