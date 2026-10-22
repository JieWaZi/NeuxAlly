package types

// OrganizeSourceWork 保存同一任务尝试内的文档局部重排与失败预算。
type OrganizeSourceWork struct {
	StartedAt       int64  `json:"startedAt"`                                                   // 当前任务尝试首次实际开始阅读的时间，Unix毫秒。
	ID              string `json:"id" gorm:"primaryKey"`                                        // 稳定待办身份。
	LibraryID       string `json:"libraryId" gorm:"index"`                                      // 资料所属库，供清理与隔离校验。
	JobID           string `json:"jobId" gorm:"uniqueIndex:idx_organize_source_work"`           // 所属持久任务。
	JobAttempt      int    `json:"jobAttempt" gorm:"uniqueIndex:idx_organize_source_work"`      // 冻结来源所属任务尝试。
	SourceVersionID string `json:"sourceVersionId" gorm:"uniqueIndex:idx_organize_source_work"` // 本次重排的不可变来源版本。
	FailureCount    int    `json:"failureCount"`                                                // 已持久失败次数；第六次进入死信。
	NotBefore       int64  `json:"notBefore"`                                                   // 最早重试时间，Unix 毫秒；零表示立即可执行。
	State           string `json:"state"`                                                       // pending、done 或 dead。
	Diagnostic      string `json:"diagnostic"`                                                  // 最近一次真实失败诊断。
	Version         int64  `json:"version"`                                                     // 待办乐观锁版本。
	CreatedAt       int64  `json:"createdAt"`                                                   // 首次登记时间，Unix 毫秒。
	UpdatedAt       int64  `json:"updatedAt"`                                                   // 最近提交时间，Unix 毫秒。
}

func (OrganizeSourceWork) TableName() string { return "knowledge_organize_source_work" }

// SourceWorkActionInput 以请求身份及版本保护单文档延后重试。
type SourceWorkActionInput struct {
	RequestID       string // 幂等请求标识。
	JobID           string // 所属任务；只允许操作其当前尝试。
	JobAttempt      int    // 必填任务尝试号；旧尝试的迟到请求不得写入新队列。
	SourceVersionID string // 任务冻结范围内的不可变来源。
	ExpectedVersion int64  // 当前待办版本。
	Diagnostic      string // 此次失败的实际诊断。
	NotBefore       int64  // 宿主选定的最早重试时间，Unix 毫秒。
}
