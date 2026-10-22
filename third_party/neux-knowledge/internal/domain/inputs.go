package types

// CreateLibraryInput 显式保存新库请求身份。
type CreateLibraryInput struct {
	ChunkingConfig   *ChunkingConfig   // 完整分块配置；创建省略采用上游默认，更新省略保留原值。
	RequestID        string            // 幂等请求标识。
	Name             string            // 名称。
	Kind             string            // 库类型。
	WikiLanguage     string            // Wiki 语言。
	Purpose          string            // 用途说明。
	WikiConfig       *WikiConfig       // 可选 Wiki 整理配置。
	IndexingStrategy *IndexingStrategy // 可选索引策略；省略时使用类型默认值。
	EmbeddingModelID string            // 向量索引所需的嵌入模型标识。
}

// UpdateLibraryInput 使用库版本保护配置和权限更新。
type UpdateLibraryInput struct {
	ChunkingConfig   *ChunkingConfig   // 完整分块配置；创建省略采用上游默认，更新省略保留原值。
	RequestID        string            // 幂等请求标识。
	LibraryID        string            // 知识库标识。
	ExpectedVersion  int64             // 当前库版本。
	Name             string            // 新名称。
	Purpose          string            // 新用途。
	Archived         bool              // 新归档状态。
	PolicyVersion    int64             // 新权限代次，不得降低。
	WikiConfig       *WikiConfig       // 新 Wiki 配置；nil 表示不修改。
	IndexingStrategy *IndexingStrategy // 新索引策略；nil 表示不修改。
	EmbeddingModelID *string           // 新嵌入模型；nil 表示不修改。
}

// ImportSourceInput 保存完整原始字节，不触发整理。
type ImportSourceInput struct {
	RequestID       string // 幂等请求标识。
	LibraryID       string // 所属库。
	SourceID        string // 新来源为空，替换时指定稳定来源身份。
	ExpectedVersion int64  // 新来源为 0，替换为当前原件代次。
	Filename        string // 原始文件名，不能是路径。
	MIME            string // 媒体类型。
	Bytes           []byte // 完整原始字节。
}

// ArchiveSourceInput 以状态行版本归档或恢复原件。
type ArchiveSourceInput struct {
	RequestID       string // 幂等请求标识。
	LibraryID       string // 所属库。
	SourceVersionID string // 来源版本身份。
	ExpectedVersion int64  // 来源状态行版本。
	Archived        bool   // 目标归档状态。
}

// WritePageInput 显式携带内容和目录基础版本。
type WritePageInput struct {
	ExpectedPolicyVersion int64    // 显式授权冻结的权限代次，零表示未启用；正值必须在内容写事务内一致。
	RequestID             string   // 幂等请求标识。
	LibraryID             string   // 目标库。
	ExpectedVersion       int      // 页面当前版本，创建为 0。
	StructureVersion      int64    // 当前结构版本。
	Page                  WikiPage // 页面内容；TenantID 忽略，库身份必须一致。
}

// RestorePageInput 恢复历史正文，保留当前主题与排序。
type RestorePageInput struct {
	RequestID        string // 幂等请求标识。
	LibraryID        string // 所属库。
	PageID           string // 稳定页面身份。
	ExpectedVersion  int    // 当前页面版本。
	RevisionVersion  int    // 要恢复的历史内容版本。
	StructureVersion int64  // 当前结构版本。
}

// WriteThemeInput 以结构版本保护主题写入和删除。
type WriteThemeInput struct {
	RequestID        string     // 幂等请求标识。
	LibraryID        string     // 目标库。
	StructureVersion int64      // 当前目录代次。
	Theme            WikiFolder // 主题内容。
	Delete           bool       // 是否删除空主题。
}

// AcceptJobInput 固定待整理来源和授权，不启动任何模型。
type AcceptJobInput struct {
	// Rebuild 明确重建所选来源的解析产物；省略时保留普通整理与旧请求摘要。
	Rebuild              bool               `json:"rebuild,omitempty"`
	EmbeddingFingerprint string             // 向量开启时冻结的端点与模型指纹。
	RequestID            string             // 幂等请求标识。
	LibraryID            string             // 目标库。
	ExpectedVersion      int64              // 接纳时库版本。
	StructureVersion     int64              // 接纳时目录代次。
	SourceVersionIDs     []string           // 显式冻结的来源版本集合。
	Permission           PermissionSnapshot // 宿主验证后的范围与权限。
	RulesVersion         string             // 固定规则版本。
	ConfigDigest         string             // 固定管线配置摘要。
}

// JobActionInput 保护取消和重试的任务状态。
type JobActionInput struct {
	RequestID       string // 幂等请求标识。
	JobID           string // 任务标识。
	ExpectedVersion int64  // 当前任务版本。
}

// PageQuery 使用 SQLite 关键词搜索与有界分页。
type PageQuery struct {
	LibraryID string  // 所属库。
	Query     string  // 转义后的普通关键词，不是 BM25。
	PageType  string  // 可选页面类型。
	ThemeID   *string // 可选精确主题，空字符串表示根。
	Status    string  // 可选状态。
	Offset    int     // 起始偏移。
	Limit     int     // 默认 50、最大 100。
}

// FailJobInput 如实保存管线失败，不需要伪造成功发布阶段。
type FailJobInput struct {
	JobActionInput                    // 任务请求身份与当前版本。
	Diagnostic     string             // 失败或部分失败的真实原因。
	Sources        []SourceConclusion // 仅可包含当前冻结来源的 failed/cancelled 结论。
}
