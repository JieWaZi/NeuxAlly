package types

// PurgePreviewInput 明确选择一个库的一项稳定来源，包含该来源所有版本。
type PurgePreviewInput struct {
	LibraryID string `json:"libraryId"` // 来源所属库。
	SourceID  string `json:"sourceId"`  // 稳定来源身份，不是单一版本。
}

// PurgeVersion 只返回不可用身份与并发代次，不返回文件名或原文。
type PurgeVersion struct {
	VersionID  string `json:"versionId"`  // 待清理的不可变来源版本。
	Version    int64  `json:"version"`    // 来源原件代次。
	RowVersion int64  `json:"rowVersion"` // 来源处理及归档状态代次。
}

// PurgePageImpact 固定引用页面版本及正文保留策略。
type PurgePageImpact struct {
	PageID       string `json:"pageId"`       // 当前被影响页面。
	Version      int    `json:"version"`      // 预览读取的页面版本。
	PreserveBody bool   `json:"preserveBody"` // 人工或多来源知识保留当前正文，否则归档并清空正文。
}

// SourcePurgePreview 是无原文的影响预览；Digest 绑定实际库内记录及策略快照。
type SourcePurgePreview struct {
	LibraryID          string            `json:"libraryId"`          // 目标库身份。
	SourceID           string            `json:"sourceId"`           // 删除全部版本的稳定来源身份。
	LibraryVersion     int64             `json:"libraryVersion"`     // 库配置版本。
	StructureVersion   int64             `json:"structureVersion"`   // 目录结构版本。
	PolicyVersion      int64             `json:"policyVersion"`      // 当前授权代次。
	Versions           []PurgeVersion    `json:"versions"`           // 全部待清理来源版本。
	Pages              []PurgePageImpact `json:"pages"`              // 实际受影响页面及保留正文策略。
	RevisionCount      int               `json:"revisionCount"`      // 将清空潜在原文的历史修订数。
	JobCount           int               `json:"jobCount"`           // 内部诊断被清理的同库任务数。
	StageCount         int               `json:"stageCount"`         // 输入和诊断被清理的同库阶段数。
	PendingImportCount int               `json:"pendingImportCount"` // 已登记但尚未完成的来源导入数。
	IssueCount         int               `json:"issueCount"`         // 被清理的同库问题记录数。
	BlobCount          int               `json:"blobCount"`          // 不同原件对象数量。
	SharedBlobCount    int               `json:"sharedBlobCount"`    // 其他来源仍使用的对象数量。
	RawOwnership       string            `json:"rawOwnership"`       // 本地固定verified；远端按真实归属诊断返回。
	BlockingJobIDs     []string          `json:"blockingJobIds"`     // 同库活任务或仍在执行阶段所属任务。
	Digest             string            `json:"digest"`             // 影响快照的 SHA-256，不包含可读原文。
	Boundaries         []string          `json:"boundaries"`         // 已知保留与物理擦除边界。
}

// ConfirmSourcePurgeInput 只接受明确布尔确认和原预览摘要。
type ConfirmSourcePurgeInput struct {
	RequestID     string `json:"requestId"`     // 确认操作的幂等身份。
	LibraryID     string `json:"libraryId"`     // 目标库身份。
	SourceID      string `json:"sourceId"`      // 目标稳定来源。
	PreviewDigest string `json:"previewDigest"` // 用户已核对的影响摘要。
	Confirmed     bool   `json:"confirmed"`     // 必须显式为 true。
}

// RecoverSourcePurgeInput 重新取得当前授权后继续同一个持久清理清单。
type RecoverSourcePurgeInput struct {
	RequestID             string `json:"requestId"`             // 本次恢复操作的幂等身份。
	LibraryID             string `json:"libraryId"`             // 原清理所属库。
	PurgeID               string `json:"purgeId"`               // 已接受的清理身份。
	ExpectedPolicyVersion int64  `json:"expectedPolicyVersion"` // 本次恢复实际获得的授权代次。
}

// PurgeBlobCleanup 仅内部持久化内容寻址键及清理进度，不携带原件字节。
type PurgeBlobCleanup struct {
	Key   string `json:"key"`   // 受管原件的 SHA-256 对象键。
	State string `json:"state"` // pending、deleted 或 retained_shared。
	Kind  string `json:"kind"`  // raw 或 temporary，分别为内容对象与精确导入临时对象。
}

// ImportBlobIntent 在写文件前固定导入身份，使中断后的未登记对象仍可准确归属。
type ImportBlobIntent struct {
	RequestID string `gorm:"primaryKey"` // 原导入请求身份。
	Digest    string // 完整原请求摘要，不保存原字节。
	LibraryID string `gorm:"index"` // 目标库身份。
	SourceID  string `gorm:"index"` // 预先固定的稳定来源身份。
	RawKey    string // 预期原件内容摘要键；清理确认后清空。
	TempKey   string // 精确导入临时文件名；清理确认后清空。
	PurgeID   string // 非空表示该请求已被永久撤销，只保留最小幂等墓碑。
	CreatedAt int64  // 意图登记时间，Unix 毫秒。
}

func (ImportBlobIntent) TableName() string { return "knowledge_import_blob_intents" }

// SourcePurgeRecord 拥有逻辑清理与磁盘清理的持久清单；不直接作为 API 输出。
type SourcePurgeRecord struct {
	ID            string             `gorm:"primaryKey"`  // 稳定清理身份。
	LibraryID     string             `gorm:"index"`       // 原来源所属库。
	SourceID      string             `gorm:"uniqueIndex"` // 已撤销的稳定来源身份。
	Version       int64              // 清理进度乐观锁。
	State         string             // 本地pending或completed；公开协议保留远端blocked。
	PreviewDigest string             // 确认时核对的完整影响摘要。
	Versions      []PurgeVersion     `gorm:"serializer:json"` // 已撤销版本身份。
	Objects       []PurgeBlobCleanup `gorm:"serializer:json"` // 原件对象及各自清理状态。
	PageIDs       []string           `gorm:"serializer:json"` // 正文或证据已更新的页面身份。
	CreatedAt     int64              // 确认提交时间，Unix 毫秒。
	UpdatedAt     int64              // 最近恢复落库时间，Unix 毫秒。
}

func (SourcePurgeRecord) TableName() string { return "knowledge_source_purges" }

// SourceTombstone 保留最小不可用来源身份，不保留文件名、摘要、摘录或原文。
type SourceTombstone struct {
	VersionID string `gorm:"primaryKey"` // 已撤销的不可变版本身份。
	SourceID  string `gorm:"index"`      // 已撤销的稳定来源身份。
	LibraryID string `gorm:"index"`      // 原所属库。
	PurgeID   string // 负责清理的持久清单。
}

func (SourceTombstone) TableName() string { return "knowledge_source_tombstones" }

// SourcePurgeStatus 只返回可验证清理事实，不声称介质物理擦除。
type SourcePurgeStatus struct {
	ID                      string         `json:"id"`                      // 清理身份。
	LibraryID               string         `json:"libraryId"`               // 原所属库。
	SourceID                string         `json:"sourceId"`                // 已撤销来源身份。
	Version                 int64          `json:"version"`                 // 清理进度版本。
	State                   string         `json:"state"`                   // 本地pending或completed；公开协议保留远端blocked。
	Versions                []PurgeVersion `json:"versions"`                // 不可用来源版本。
	DeletedBlobCount        int            `json:"deletedBlobCount"`        // 已删除或已不存在的非共享原件数。
	RetainedSharedBlobCount int            `json:"retainedSharedBlobCount"` // 因其他来源使用而保留的原件数。
	BlockedBlobCount        int            `json:"blockedBlobCount"`        // 本地固定为0，远端按实际阻塞诊断返回。
	PendingBlobCount        int            `json:"pendingBlobCount"`        // 尚需实际处理的原件数。
	PageIDs                 []string       `json:"pageIds"`                 // 已更新的知识页面身份。
	Boundaries              []string       `json:"boundaries"`              // 人工正文、备份和物理介质等保留边界。
}
