package types

import (
	"database/sql/driver"
	"encoding/json"
)

// IndexingStrategy 控制知识库启用的索引与整理管线；各开关彼此独立。
type IndexingStrategy struct {
	// VectorEnabled 是否启用语义向量索引与检索。
	VectorEnabled bool `json:"vectorEnabled" yaml:"vector_enabled"`
	// KeywordEnabled 是否启用关键词检索。
	KeywordEnabled bool `json:"keywordEnabled" yaml:"keyword_enabled"`
	// WikiEnabled 是否启用文档 Wiki 整理。
	WikiEnabled bool `json:"wikiEnabled" yaml:"wiki_enabled"`
	// GraphEnabled 是否启用知识图谱抽取。
	GraphEnabled bool `json:"graphEnabled" yaml:"graph_enabled"`
}

// DefaultIndexingStrategy 是 NeuxAlly 本地核心的默认策略：关键词开启，向量/Wiki/图谱默认关闭。
func DefaultIndexingStrategy() IndexingStrategy {
	return IndexingStrategy{VectorEnabled: false, KeywordEnabled: true, WikiEnabled: false, GraphEnabled: false}
}

// DefaultGeneralLibraryIndexingStrategy 是 NeuxAlly 通用库的本地整理默认：Wiki 开启，向量关闭，关键词开启。
func DefaultGeneralLibraryIndexingStrategy() IndexingStrategy {
	return IndexingStrategy{VectorEnabled: false, KeywordEnabled: true, WikiEnabled: true, GraphEnabled: false}
}

// NeedsEmbedding 报告本地是否必须配置嵌入模型；SQLite关键词分支不调用Embedding。
// 固定上游的向量和关键词都经检索引擎BatchIndex，因此它的条件为vector或keyword。
func (s IndexingStrategy) NeedsEmbedding() bool {
	return s.VectorEnabled
}

// IsZero 报告是否未启用任何管线。
func (s IndexingStrategy) IsZero() bool {
	return !s.VectorEnabled && !s.KeywordEnabled && !s.WikiEnabled && !s.GraphEnabled
}

// Value 实现 driver.Valuer。
func (s IndexingStrategy) Value() (driver.Value, error) { return json.Marshal(s) }

// Scan 实现 sql.Scanner；空列回落到 DefaultIndexingStrategy。
func (s *IndexingStrategy) Scan(value interface{}) error {
	if value == nil {
		*s = DefaultIndexingStrategy()
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		*s = DefaultIndexingStrategy()
		return nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		*s = DefaultIndexingStrategy()
		return err
	}
	return nil
}

// HasAnyIndexing 沿用固定上游至少启用一种管线的校验。
// 本地支持 Wiki、向量和关键词；未接入的图谱管线不能单独满足该条件。
func (s IndexingStrategy) HasAnyIndexing() bool {
	return s.VectorEnabled || s.KeywordEnabled || s.WikiEnabled
}
