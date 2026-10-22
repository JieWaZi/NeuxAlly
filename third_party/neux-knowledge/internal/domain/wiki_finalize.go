package types

// WikiFinalizeOp 保存 Lite 批次已经发布的全库收尾增量；同事务写入，崩溃后继续合并。
type WikiFinalizeOp struct {
	ID         string `json:"id" gorm:"primaryKey"`                            // 稳定收尾行身份。
	LibraryID  string `json:"libraryId" gorm:"index"`                          // 所属知识库。
	JobID      string `json:"jobId" gorm:"uniqueIndex:idx_lite_finalize"`      // 所属显式整理任务。
	JobAttempt int    `json:"jobAttempt" gorm:"uniqueIndex:idx_lite_finalize"` // 实际任务尝试。
	Key        string `json:"key" gorm:"uniqueIndex:idx_lite_finalize"`        // slug或change的稳定去重键。
	Kind       string `json:"kind"`                                            // slug、added或removed。
	Slug       string `json:"slug"`                                            // 实际发布的页面地址。
	Title      string `json:"title"`                                           // 实际发布的页面标题。
	DocTitle   string `json:"docTitle"`                                        // 实际变化的文档标题。
	DocSummary string `json:"docSummary"`                                      // 变化文档的一句摘要。
	CreatedAt  int64  `json:"createdAt"`                                       // 首次写入时间，Unix毫秒。
}

func (WikiFinalizeOp) TableName() string { return "knowledge_wiki_finalize_ops" }
