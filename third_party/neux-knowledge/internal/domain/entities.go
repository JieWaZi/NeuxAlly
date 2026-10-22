package types

import "encoding/json"

// Library 保存知识库配置与结构及权限的独立代次。
type Library struct {
	ChunkingConfig   *ChunkingConfig  `json:"chunkingConfig,omitempty" gorm:"serializer:json"` // 固定上游分块设置；未指定时采用创建默认值。
	ID               string           `json:"id" gorm:"primaryKey"`                            // 稳定库标识。
	Name             string           `json:"name"`                                            // 用户可见名称。
	Kind             string           `json:"kind"`                                            // 当前库类型，固定为general。
	WikiLanguage     string           `json:"wikiLanguage"`                                    // Wiki 正文语言。
	Purpose          string           `json:"purpose"`                                         // 库的用途说明。
	WikiConfig       *WikiConfig      `json:"wikiConfig,omitempty" gorm:"type:json"`           // Wiki 整理粒度与补充指令。
	IndexingStrategy IndexingStrategy `json:"indexingStrategy" gorm:"type:json"`               // 启用的索引与 Wiki 管线。
	EmbeddingModelID string           `json:"embeddingModelId,omitempty"`                      // 向量或关键词索引使用的嵌入模型标识。
	Version          int64            `json:"version"`                                         // 所有库配置写入的乐观锁。
	StructureVersion int64            `json:"structureVersion"`                                // 主题及页面位置的结构代次。
	PolicyVersion    int64            `json:"policyVersion"`                                   // 宿主授权策略代次，变更阻止旧任务发布。
	Archived         bool             `json:"archived"`                                        // 是否归档。
	CreatedAt        int64            `json:"createdAt"`                                       // 创建时间，Unix 毫秒。
	UpdatedAt        int64            `json:"updatedAt"`                                       // 更新时间，Unix 毫秒。
}

func (Library) TableName() string { return "knowledge_libraries" }

// SourceVersion 保存不可变原件身份与可变处理状态；原字节另存受管目录。
type SourceVersion struct {
	// Description 是普通文档profile生成的短摘要，不是Wiki页面。
	Description string `json:"description,omitempty"`
	// Profile 保存上游规范化文档主题、类型与问题，不包含原件全文。
	Profile *KnowledgeProfile `json:"profile,omitempty" gorm:"serializer:json"`
	// SummaryStatus 表示资料摘要的真实持久处理状态。
	SummaryStatus string `json:"summaryStatus,omitempty"`

	ID         string          `json:"id" gorm:"primaryKey"`      // 不可变来源版本标识。
	LibraryID  string          `json:"libraryId" gorm:"index"`    // 所属知识库。
	SourceID   string          `json:"sourceId" gorm:"index"`     // 稳定来源标识。
	Version    int64           `json:"version"`                   // 同一来源的原件代次。
	RowVersion int64           `json:"rowVersion"`                // 归档与处理状态的乐观锁。
	Filename   string          `json:"filename"`                  // 原始文件名，不作为磁盘路径。
	MIME       string          `json:"mime"`                      // 来源媒体类型。
	SHA256     string          `json:"sha256"`                    // 原始字节摘要。
	Size       int64           `json:"size"`                      // 原始字节长度。
	RawKey     string          `json:"rawKey"`                    // 受管对象键，仅由核心生成。
	State      string          `json:"state"`                     // pending、processed、no-change、failed 或 cancelled。
	Archived   bool            `json:"archived"`                  // 来源是否已归档。
	Coverage   json.RawMessage `json:"coverage" gorm:"type:json"` // Reader 提供的真实阅读覆盖。
	Diagnostic string          `json:"diagnostic"`                // 最近处理诊断。
	CreatedAt  int64           `json:"createdAt"`                 // 创建时间，Unix 毫秒。
}

func (SourceVersion) TableName() string { return "knowledge_source_versions" }

// SourceHead 确定来源当前版本；不以文件名判断同一来源。
type SourceHead struct {
	ID        string `gorm:"primaryKey"` // 稳定来源标识。
	LibraryID string `gorm:"index"`      // 所属库。
	VersionID string // 当前来源版本标识。
	Version   int64  // 当前原件代次。
}

func (SourceHead) TableName() string { return "knowledge_sources" }

// PermissionSnapshot 固定接纳时的权限与真实项目范围。
type PermissionSnapshot struct {
	PolicyVersion int64    `json:"policyVersion"` // 接纳时权限策略代次。
	Scope         []string `json:"scope"`         // 宿主已授权的真实项目范围。
	Operations    []string `json:"operations"`    // 明确允许的领域操作。
}

// Job 是持久知识作业；不拥有模型宿主。
type Job struct {
	// ActivityActor 固定接纳时的真实发起身份，供后台终态活动沿用。
	ActivityActor        ActivityActor `json:"activityActor,omitempty" gorm:"serializer:json"`
	EmbeddingFingerprint string        `json:"-"`                            // 接纳时冻结的嵌入模型空间指纹，不含密钥。
	LibrarySnapshot      *Library      `json:"-" gorm:"serializer:json"`     // 接纳事务固定的完整库配置，后续设置变化不影响在途任务。
	ID                   string        `json:"id" gorm:"primaryKey"`         // 稳定任务标识。
	LibraryID            string        `json:"libraryId" gorm:"index"`       // 所属知识库。
	RequestID            string        `json:"requestId" gorm:"uniqueIndex"` // 接纳幂等键。
	// State 待处理记录的当前生命周期状态。
	State            string             `json:"state"`                                   // queued、processing、completed、partial_failed、failed、cancel_requested 或 cancelled。
	Attempt          int                `json:"attempt"`                                 // 当前任务尝试号。
	Version          int64              `json:"version"`                                 // 任务状态乐观锁。
	StructureVersion int64              `json:"structureVersion"`                        // 接纳时目录基础版本。
	SourceVersionIDs []string           `json:"sourceVersionIds" gorm:"serializer:json"` // 冻结的来源版本集合。
	Permission       PermissionSnapshot `json:"permission" gorm:"serializer:json"`       // 冻结权限快照。
	RulesVersion     string             `json:"rulesVersion"`                            // 固定规则版本。
	ConfigDigest     string             `json:"configDigest"`                            // 固定管线与宿主配置摘要。
	Diagnostic       string             `json:"diagnostic"`                              // 最近一次失败或取消诊断。
	AttemptStartedAt int64              `json:"attemptStartedAt,omitempty"`              // 当前尝试的首次接纳时间，Unix 毫秒；恢复不得改写。
	CancelRequested  bool               `json:"cancelRequested"`                         // 持久取消屏障。
	CreatedAt        int64              `json:"createdAt"`                               // 接纳时间，Unix 毫秒。
	UpdatedAt        int64              `json:"updatedAt"`                               // 状态变更时间，Unix 毫秒。
}

func (Job) TableName() string { return "knowledge_jobs" }

// ModelUsage 保存模型供应方报告的本次请求用量。
type ModelUsage struct {
	InputTokens  int64 `json:"inputTokens"`  // 实际输入 token。
	OutputTokens int64 `json:"outputTokens"` // 实际输出 token。
	TotalTokens  int64 `json:"totalTokens"`  // 实际总 token。
}

// ModelDiagnostics 保存直接 CLI 调用可核实的能力与推理设置。
type ModelDiagnostics struct {
	ActualReasoning   string   `json:"actualReasoning,omitempty"`   // CLI 实际采用的推理档位。
	ThinkingDisabled  bool     `json:"thinkingDisabled"`            // 已核实关闭思考。
	ReasoningFallback string   `json:"reasoningFallback,omitempty"` // CLI 缺少关闭选项时实际降级。
	ToolsDisabled     bool     `json:"toolsDisabled"`               // 已核实关闭工具。
	Limitations       []string `json:"limitations,omitempty"`       // 原生能力不能保证的限制。
}

type Stage struct {
	Diagnostics *ModelDiagnostics `json:"diagnostics,omitempty" gorm:"serializer:json"` // 本次原生调用的可核实诊断。

	SourceVersionID  string      `json:"sourceVersionId,omitempty"`                         // 单文档调用绑定的不可变来源。
	SourceVersionIDs []string    `json:"sourceVersionIds,omitempty" gorm:"serializer:json"` // 本阶段实际读取的多来源集合。
	ResultText       string      `json:"resultText"`                                        // 模型实际返回的完整原文。
	Usage            *ModelUsage `json:"usage,omitempty" gorm:"serializer:json"`            // 模型真实用量，未知为空。
	FinishedAt       int64       `json:"finishedAt"`                                        // 阶段真实结束时间，Unix毫秒。

	ID           string          `json:"id" gorm:"primaryKey"`   // 阶段标识。
	JobID        string          `json:"jobId" gorm:"index"`     // 所属任务。
	Name         string          `json:"name"`                   // 管线阶段名称。
	Kind         string          `json:"kind"`                   // map、cite、taxonomy、reduce 或 finalize 等阶段类型。
	JobAttempt   int             `json:"jobAttempt"`             // 创建时的任务尝试号。
	Attempt      int             `json:"attempt"`                // 当前阶段尝试号。
	Version      int64           `json:"version"`                // 阶段状态乐观锁。
	State        string          `json:"state"`                  // pending、running、finished 或 published。
	Outcome      string          `json:"outcome"`                // unknown、succeeded、failed、cancelled 或取消前未接纳的 not_started。
	Completeness string          `json:"completeness"`           // unknown、complete 或 truncated。
	StopReason   string          `json:"stopReason"`             // 宿主原生停止原因。
	Diagnostic   string          `json:"diagnostic"`             // 可信执行诊断。
	Input        json.RawMessage `json:"input" gorm:"type:json"` // 可恢复阶段输入。
	CreatedAt    int64           `json:"createdAt"`              // 创建时间，Unix 毫秒。
}

func (Stage) TableName() string { return "knowledge_stages" }

// PageMutation 显式声明候选生成所依据的页面版本。
type PageMutation struct {
	Page            WikiPage `json:"page"`            // 完整的新页面内容；页面标识稳定。
	ExpectedVersion int      `json:"expectedVersion"` // 创建为 0，更新为生成基础版本。
}

// ThemeMutation 以库结构代次保护主题创建和修改。
type ThemeMutation struct {
	Theme  WikiFolder `json:"theme"`  // 主题内容；两层以内。
	Delete bool       `json:"delete"` // 是否删除空主题。
}

// SourceConclusion 只标记真实已读来源的处理结论。
type SourceConclusion struct {
	// Description 是同次真实摘要阶段产生并待原子发布的来源摘要。
	Description string `json:"description,omitempty"`
	// Profile 是与摘要一起发布的规范化来源档案。
	Profile *KnowledgeProfile `json:"profile,omitempty"`

	SourceVersionID string          `json:"sourceVersionId"` // 结论对应的不可变版本。
	State           string          `json:"state"`           // processed、no-change、failed 或 cancelled。
	Complete        bool            `json:"complete"`        // 是否确认规定范围完整读取。
	Coverage        json.RawMessage `json:"coverage"`        // 可核对的实际覆盖。
	Diagnostic      string          `json:"diagnostic"`      // 失败或限制原因。
}

// Publication 是可信管线校验后固定的单批原子变更。
type Publication struct {
	Maintenance      bool               `json:"maintenance,omitempty"`      // 全局收尾写入不再次追加同一收尾队列。
	SourceVersionIDs []string           `json:"sourceVersionIds,omitempty"` // 本次实际贡献的冻结来源读集；含无变化页面的来源输入，不包含仅保留的历史归因。
	PageVersions     map[string]int     `json:"pageVersions,omitempty"`     // 生成实际读取的页面版本，包括无变化结论的读集。
	Pages            []PageMutation     `json:"pages"`                      // 需要原子写入的页面。
	Themes           []ThemeMutation    `json:"themes"`                     // 需要原子写入的主题。
	Sources          []SourceConclusion `json:"sources"`                    // 本批已确认的来源结论。
	Final            bool               `json:"final"`                      // 是否结束本任务。
}

// Receipt 保存幂等操作实际提交后的结果。
type Receipt struct {
	JobAttempt int      `json:"jobAttempt,omitempty"`                     // 实际变更所属任务尝试，供内部来源重排诊断核对。
	RequestID  string   `json:"requestId" gorm:"primaryKey"`              // 全局幂等请求标识。
	Digest     string   `json:"-"`                                        // 原请求摘要，不向应用序列化。
	Operation  string   `json:"operation"`                                // 实际操作类型。
	LibraryID  string   `json:"libraryId" gorm:"index"`                   // 操作知识库。
	ResourceID string   `json:"resourceId"`                               // 创建或修改的主资源标识。
	JobID      string   `json:"jobId,omitempty"`                          // 关联知识任务。
	Version    int64    `json:"version"`                                  // 提交后的主资源版本。
	State      string   `json:"state"`                                    // 已保存、已接纳或已发布的真实事实。
	PageIDs    []string `json:"pageIds,omitempty" gorm:"serializer:json"` // 本批实际改变的页面。
	CreatedAt  int64    `json:"createdAt"`                                // 原子提交时间，Unix 毫秒。
}

func (Receipt) TableName() string { return "knowledge_receipts" }
