package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// DocumentEmbedder 是宿主从既有配置冻结的真实嵌入调用，不保存密钥副本。
type DocumentEmbedder struct {
	PublishGuard func(context.Context) (func(), error)                // 外部嵌入完成后复核宿主授权，持锁覆盖索引提交。
	Fingerprint  string                                               // 服务端点与模型空间指纹。
	Embed        func(context.Context, []string) ([][]float32, error) // 沿用宿主已有嵌入模型服务。
}

// DocumentChunkHit 暴露真实持久块和检索分数。
type DocumentChunkHit = types.DocumentChunkHit

// IndexedDocumentChunk 暴露原文块状态，不序列化凭据与向量。
type IndexedDocumentChunk = types.EmbeddingChunkIndex

// ValidateDocumentEmbedding 拒绝空、维度不一、零范数和非有限数，不能以请求成功冒充有效向量。
func (e *Engine) ValidateDocumentEmbedding(ctx context.Context, embedder DocumentEmbedder, texts []string) error {
	_, err := embedDocumentTexts(ctx, embedder, texts)
	return err
}
func embedDocumentTexts(ctx context.Context, embedder DocumentEmbedder, texts []string) ([][]float32, error) {
	if embedder.Embed == nil || embedder.Fingerprint == "" {
		return nil, &Error{Code: ErrInvalid, Operation: "index.embed", Detail: "embedding service is not configured"}
	}
	// 移植 WeKnora internal/models/embedding/batch.go 的批次设置、顺序回填和首错停止。
	// 宿主已有模型端口，SQLite 适配顺序执行批次，避免另建 ants 连接池。
	size := os.Getenv("BATCH_EMBED_SIZE")
	if size == "" {
		size = "5"
	}
	batchSize, err := strconv.Atoi(size)
	if err != nil || batchSize < 1 {
		return nil, &Error{Code: ErrInvalid, Operation: "index.embed", Detail: "invalid BATCH_EMBED_SIZE"}
	}
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := min(start+batchSize, len(texts))
		batch, err := embedder.Embed(ctx, texts[start:end])
		if err != nil {
			return nil, &Error{Code: ErrStorage, Operation: "index.embed", Detail: "embedding service failed", Cause: err}
		}
		if len(batch) != end-start {
			return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "embedding count mismatch"}
		}
		vectors = append(vectors, batch...)
	}
	if len(vectors) != len(texts) || len(vectors) == 0 {
		return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "embedding count mismatch"}
	}
	dimensions := len(vectors[0])
	if dimensions == 0 {
		return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "empty embedding"}
	}
	for _, v := range vectors {
		norm := 0.0
		if len(v) != dimensions {
			return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "embedding dimensions mismatch"}
		}
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "embedding contains nonfinite values"}
			}
			norm += float64(x) * float64(x)
		}
		if norm == 0 {
			return nil, &Error{Code: ErrCandidateInvalid, Operation: "index.embed", Detail: "zero embedding"}
		}
	}
	return vectors, nil
}

// IndexDocument 使用 WeKnora 原分块器及接纳时配置，保存父子块，再执行真实嵌入与原子就绪提交。
func (e *Engine) IndexDocument(ctx context.Context, jobID, sourceID string, artifact ReadArtifact, embedder DocumentEmbedder) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	lib := job.LibrarySnapshot
	if lib == nil {
		lib, err = e.GetLibrary(ctx, job.LibraryID)
		if err != nil {
			return err
		}
	}
	if job.EmbeddingFingerprint != "" && lib.IndexingStrategy.VectorEnabled && job.EmbeddingFingerprint != embedder.Fingerprint {
		return &Error{Code: ErrVersionConflict, Operation: "index.embed", Detail: "embedding configuration changed after acceptance"}
	}
	if artifact.SourceVersionID != sourceID || artifact.Status != "complete" {
		return &Error{Code: ErrCandidateInvalid, Operation: "index.document", Detail: "complete verified source is required"}
	}
	var text strings.Builder
	type boundary struct {
		start, end int           // 原始文本中的rune覆盖边界。
		locator    SourceLocator // Reader已核对的来源位置。
	}
	ranges := []boundary{}
	for _, block := range artifact.Blocks {
		start := utf8.RuneCountInString(text.String())
		text.WriteString(block.Content)
		ranges = append(ranges, boundary{start, utf8.RuneCountInString(text.String()), block.Locator})
	}
	chunks, err := SplitDocument(text.String(), lib.ChunkingConfig)
	if err != nil {
		return &Error{Code: ErrInvalid, Operation: "index.chunk", Cause: err}
	}
	locate := func(start, end int) SourceLocator {
		var result SourceLocator
		found := 0
		for _, b := range ranges {
			if b.end > start && b.start < end {
				if found == 0 {
					result = b.locator
				} else if result.Kind == "line" && b.locator.Kind == "line" {
					result.EndLine = b.locator.EndLine
				}
				found++
			}
		}
		if found > 1 && result.Kind != "line" {
			return SourceLocator{Kind: "range", Section: fmt.Sprintf("runes:%d-%d", start, end)}
		}
		return result
	}
	identity := func(kind string, seq int) string {
		h := sha256.Sum256([]byte(sourceID + "/" + kind + fmt.Sprint(seq)))
		return hex.EncodeToString(h[:])
	}
	rows := []types.EmbeddingChunkIndex{}
	for _, chunk := range chunks.Parents {
		rows = append(rows, types.EmbeddingChunkIndex{ID: identity("parent", chunk.Seq), ChunkID: identity("parent", chunk.Seq), ChunkType: "parent", Content: chunk.Content, ContextHeader: chunk.ContextHeader, Start: chunk.Start, End: chunk.End, Sequence: chunk.Seq, Locator: locate(chunk.Start, chunk.End), State: "parsed"})
	}
	texts := []string{}
	indices := []int{}
	for _, chunk := range chunks.Children {
		parent := ""
		if chunk.ParentIndex >= 0 && chunk.ParentIndex < len(chunks.Parents) {
			parent = identity("parent", chunks.Parents[chunk.ParentIndex].Seq)
		}
		state := "parsed"
		fingerprint := ""
		if lib.IndexingStrategy.VectorEnabled {
			state = "pending"
			fingerprint = embedder.Fingerprint
			texts = append(texts, chunk.EmbeddingContent())
			indices = append(indices, len(rows))
		}
		rows = append(rows, types.EmbeddingChunkIndex{ID: identity("child", chunk.Seq), ChunkID: identity("child", chunk.Seq), ChunkType: "child", ParentID: parent, Content: chunk.Content, ContextHeader: chunk.ContextHeader, Start: chunk.Start, End: chunk.End, Sequence: chunk.Seq, Locator: locate(chunk.Start, chunk.End), EmbeddingModelID: lib.EmbeddingModelID, Fingerprint: fingerprint, State: state})
	}
	if len(rows) == 0 {
		return &Error{Code: ErrCandidateInvalid, Operation: "index.chunk", Detail: "source produced no chunks"}
	}
	if lib.IndexingStrategy.VectorEnabled && (embedder.Embed == nil || embedder.Fingerprint == "") {
		return &Error{Code: ErrInvalid, Operation: "index.embed", Detail: "enabled vector index requires a frozen embedding service"}
	}
	if lib.IndexingStrategy.VectorEnabled {
		vectors, err := embedDocumentTexts(ctx, embedder, texts)
		if err != nil {
			return err
		}
		for i, index := range indices {
			rows[index].Vector = vectors[i]
			rows[index].Dimensions = len(vectors[i])
		}
	}
	// 外部嵌入不持授权锁，返回后再复核；撤权可以立即完成，迟到结果不得落库。
	if embedder.PublishGuard != nil {
		release, err := embedder.PublishGuard(ctx)
		if err != nil {
			return err
		}
		if release == nil {
			return &Error{Code: ErrInvalid, Operation: "index.document", Detail: "publication guard has no release function"}
		}
		defer release()
	}
	if err = e.store.SaveDocumentChunks(ctx, jobID, sourceID, rows); err != nil {
		return err
	}
	if !lib.IndexingStrategy.VectorEnabled {
		return nil
	}
	if err = e.store.CompleteDocumentChunks(ctx, jobID, sourceID, rows); err != nil {
		return err
	}
	return nil
}

// ListDocumentChunks 返回来源同一版本的实际父子块。
func (e *Engine) ListDocumentChunks(ctx context.Context, libraryID, sourceID string) ([]IndexedDocumentChunk, error) {
	return e.store.ListDocumentChunks(ctx, libraryID, sourceID)
}

// SearchDocumentChunks 提供实际向量或关键词检索；向量路径不回落到 LIKE。
func (e *Engine) SearchDocumentChunks(ctx context.Context, libraryID, query string, vector []float32, fingerprint string, limit int) ([]DocumentChunkHit, error) {
	return e.store.SearchDocumentChunks(ctx, libraryID, query, vector, fingerprint, limit)
}

// PrepareIndexedDocument 将实际持久子块交给 Wiki 引用，避免另建一套证据分块规则。
func (e *Engine) PrepareIndexedDocument(ctx context.Context, jobID, sourceID string, artifact ReadArtifact, embedder DocumentEmbedder) (ReadArtifact, error) {
	if err := e.IndexDocument(ctx, jobID, sourceID, artifact, embedder); err != nil {
		return artifact, err
	}
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return artifact, err
	}
	rows, err := e.ListDocumentChunks(ctx, job.LibraryID, sourceID)
	if err != nil {
		return artifact, err
	}
	artifact.Blocks = nil
	for _, row := range rows {
		if row.ChunkType == "child" {
			artifact.Blocks = append(artifact.Blocks, &Chunk{ID: row.ChunkID, ChunkIndex: row.Sequence, StartAt: row.Start, EndAt: row.End, Content: row.Content, ChunkType: "text", Locator: row.Locator})
		}
	}
	return artifact, nil
}

// GetDocumentChunkParents 有界读取命中父块，避免为少量命中解码整份原件的子块向量。
func (e *Engine) GetDocumentChunkParents(ctx context.Context, libraryID string, ids []string) ([]IndexedDocumentChunk, error) {
	return e.store.GetDocumentChunkParents(ctx, libraryID, ids)
}
