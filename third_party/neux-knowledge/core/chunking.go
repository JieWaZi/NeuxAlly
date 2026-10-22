package core

import (
	"github.com/JieWaZi/neux-knowledge/internal/chunker"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// ChunkingConfig 完整保留固定上游分块配置；解析引擎与表格指导未接入时明确拒绝。
type ChunkingConfig = types.ChunkingConfig

// ParserEngineRule 保留上游按文件类型选择解析器的配置合同。
type ParserEngineRule = types.ParserEngineRule

// DocumentChunk 保留上游原文、上下文标题、序号及原文 rune 偏移。
type DocumentChunk = chunker.Chunk

// DocumentChildChunk 保留子块与父块数组的对应关系；ParentIndex 为 -1 时无父块。
type DocumentChildChunk = chunker.ChildChunk

// DocumentChunks 保留上游父块与子块，平铺模式将全部结果放入 Children。
type DocumentChunks = chunker.ParentChildResult

// DefaultChunkingConfig 返回固定上游创建表单的完整默认设置。
func DefaultChunkingConfig() ChunkingConfig { return types.DefaultChunkingConfig() }

// SplitDocument 直接复用固定上游算法拆分 Reader 已提供的文本。
// 偏移以传入文本的 rune 为单位；调用方负责保存同一文本，不能在定位后再规范化换行。
// nil 配置采用新库默认值，显式配置按上游 NormalizeSplitterConfig 处理。
func SplitDocument(text string, cfg *ChunkingConfig) (DocumentChunks, error) {
	if cfg == nil {
		defaults := DefaultChunkingConfig()
		cfg = &defaults
	}
	if err := types.ValidateChunkingConfig(cfg); err != nil {
		return DocumentChunks{}, err
	}
	base := chunker.NormalizeSplitterConfig(chunker.SplitterConfig{
		ChunkSize: cfg.ChunkSize, ChunkOverlap: cfg.ChunkOverlap, Separators: cfg.Separators,
		Strategy: cfg.Strategy, TokenLimit: cfg.TokenLimit, Languages: cfg.Languages,
	})
	if cfg.EnableParentChild {
		parent, child := chunker.DeriveParentChildConfigs(base, cfg.ParentChunkSize, cfg.ChildChunkSize)
		return chunker.SplitParentChild(text, parent, child), nil
	}
	chunks := chunker.Split(text, base)
	out := DocumentChunks{}
	for _, chunk := range chunks {
		out.Children = append(out.Children, DocumentChildChunk{Chunk: chunk, ParentIndex: -1})
	}
	return out, nil
}
