package types

// DeleteLibraryInput 以库配置版本永久删除整库及受管原件。
type DeleteLibraryInput struct {
	RequestID       string `json:"requestId"`       // 全局幂等请求标识。
	LibraryID       string `json:"libraryId"`       // 待永久删除的知识库。
	ExpectedVersion int64  `json:"expectedVersion"` // 用户确认的库配置版本。
}

// RecoverLibraryDeletionInput 使用持久清单继续原件清理。
type RecoverLibraryDeletionInput struct {
	RequestID       string `json:"requestId"`       // 全局幂等请求标识。
	LibraryID       string `json:"libraryId"`       // 已删除知识库身份。
	ExpectedVersion int64  `json:"expectedVersion"` // 当前清理清单版本。
}

// LibraryDeletionRecord 保留最小删除事实和可恢复清单，不保存知识正文。
type LibraryDeletionRecord struct {
	LibraryID     string             `json:"libraryId" gorm:"primaryKey"` // 永久撤销的库身份。
	Version       int64              `json:"version"`                     // 清理状态乐观锁。
	PolicyVersion int64              `json:"policyVersion"`               // 删除时授权代次，供边界验证恢复请求。
	State         string             `json:"state"`                       // 本地pending或completed；公开协议保留远端blocked。
	Objects       []PurgeBlobCleanup `json:"-" gorm:"serializer:json"`    // 核心生成的精确受管对象清单。
}

func (LibraryDeletionRecord) TableName() string { return "knowledge_library_deletions" }

// LibraryDeletionStatus 对外只公开清理计数与边界，不公开对象键。
type LibraryDeletionStatus struct {
	LibraryID               string `json:"libraryId"`               // 已删除库身份。
	Version                 int64  `json:"version"`                 // 清理状态版本。
	PolicyVersion           int64  `json:"policyVersion"`           // 删除时策略代次。
	State                   string `json:"state"`                   // 本地pending或completed；公开协议保留远端blocked。
	DeletedBlobCount        int    `json:"deletedBlobCount"`        // 已移除或确认不存在的对象数。
	RetainedSharedBlobCount int    `json:"retainedSharedBlobCount"` // 其他来源仍引用的保留对象数。
	BlockedBlobCount        int    `json:"blockedBlobCount"`        // 本地固定为0，远端按实际阻塞诊断返回。
	PendingBlobCount        int    `json:"pendingBlobCount"`        // 尚未处理的对象数。
}
