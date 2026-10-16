package constants

//go:generate toolx gen enum SkillResourceAction

// SkillResourceAction 表示保存技能版本前执行的资源树操作。
type SkillResourceAction string

const (
	// SkillResourceActionWrite 表示写入或覆盖一个资源文件。
	SkillResourceActionWrite SkillResourceAction = "write"
	// SkillResourceActionMkdir 表示创建一个资源目录。
	SkillResourceActionMkdir SkillResourceAction = "mkdir"
	// SkillResourceActionMove 表示移动一个资源文件或目录。
	SkillResourceActionMove SkillResourceAction = "move"
	// SkillResourceActionDelete 表示删除一个资源文件或目录。
	SkillResourceActionDelete SkillResourceAction = "delete"
)
