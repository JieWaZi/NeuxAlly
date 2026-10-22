package types

// EmbeddingChunkIndex 保存上游分块的原文、父子关系及实际向量索引状态。
type EmbeddingChunkIndex struct {
	Content          string        `json:"content"`                        // 真实分块正文，不用页面摘要代替。
	ContextHeader    string        `json:"contextHeader,omitempty"`        // 上游分块的标题路径上下文。
	ParentID         string        `json:"parentId,omitempty"`             // 子块的父块持久身份。
	ChunkType        string        `json:"chunkType"`                      // parent 或 child，父块不单独嵌入。
	Start            int           `json:"start"`                          // 上游确认的原文 rune 起始偏移。
	End              int           `json:"end"`                            // 上游确认的原文 rune 结束偏移。
	Sequence         int           `json:"sequence"`                       // 上游同级块顺序。
	Locator          SourceLocator `json:"locator" gorm:"serializer:json"` // 阅读器确认的来源定位。
	Fingerprint      string        `json:"-"`                              // 规范化服务地址与模型指纹，不包含凭据。
	Dimensions       int           `json:"dimensions"`                     // 已验证向量维度。
	Vector           []float32     `json:"-" gorm:"serializer:json"`       // 真实向量；只有indexed状态可检索。
	ID               string        `json:"id" gorm:"primaryKey"`           // 稳定索引行标识。
	LibraryID        string        `json:"libraryId" gorm:"index"`         // 所属库。
	SourceVersionID  string        `json:"sourceVersionId" gorm:"index"`   // 不可变来源版本。
	ChunkID          string        `json:"chunkId" gorm:"index"`           // Reader 块身份。
	EmbeddingModelID string        `json:"embeddingModelId"`               // 期望嵌入模型标识。
	State            string        `json:"state"`                          // pending、indexed 或 failed。
	Diagnostic       string        `json:"diagnostic"`                     // 最近处理说明。
	CreatedAt        int64         `json:"createdAt"`                      // 登记时间，Unix 毫秒。
	UpdatedAt        int64         `json:"updatedAt"`                      // 更新时间，Unix 毫秒。
}

func (EmbeddingChunkIndex) TableName() string { return "knowledge_embedding_chunks" }

// EnqueueEmbeddingChunksInput 登记待嵌入块；Phase 2 再写入真实向量。
type EnqueueEmbeddingChunksInput struct {
	LibraryID        string   // 所属库。
	SourceVersionID  string   // 不可变来源版本。
	ChunkIDs         []string // Reader 块身份。
	EmbeddingModelID string   // 期望嵌入模型。
}

// DocumentChunkHit 保存检索排名与可回溯的实际原文块。
type DocumentChunkHit struct {
	Chunk EmbeddingChunkIndex `json:"chunk"` // 真实同版本持久原文块。
	Score float64             `json:"score"` // 向量相似度或关键词分数。
}
