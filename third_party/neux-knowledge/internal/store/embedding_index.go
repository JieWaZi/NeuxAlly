package store

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"gorm.io/gorm"
)

// EnqueueEmbeddingChunks 在 vector 索引启用时持久 pending 行，供后续异步嵌入消费。
func (s *Store) EnqueueEmbeddingChunks(ctx context.Context, in types.EnqueueEmbeddingChunksInput) error {
	if in.LibraryID == "" || in.SourceVersionID == "" || in.EmbeddingModelID == "" || len(in.ChunkIDs) == 0 {
		return fail(types.ErrInvalid, "embedding.enqueue", "library, source, model and chunks are required")
	}
	return s.transaction(ctx, func(tx *gorm.DB) error {
		now := time.Now().UnixMilli()
		for _, chunkID := range in.ChunkIDs {
			row := types.EmbeddingChunkIndex{ID: newID(), LibraryID: in.LibraryID, SourceVersionID: in.SourceVersionID, ChunkID: chunkID, EmbeddingModelID: in.EmbeddingModelID, State: "pending", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveDocumentChunks 按冻结任务接纳真实块，禁止向同库混入不同模型空间。
func (s *Store) SaveDocumentChunks(ctx context.Context, jobID, sourceID string, rows []types.EmbeddingChunkIndex) error {
	return s.transaction(ctx, func(tx *gorm.DB) error {
		job, lib, err := activeJob(tx, jobID)
		if err != nil {
			return err
		}
		if err = validateOrganizeSource(tx, job, sourceID); err != nil {
			return err
		}
		var existing []types.EmbeddingChunkIndex
		if err = tx.Where("library_id = ? AND state = ?", lib.ID, "indexed").Find(&existing).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if row.ChunkType == "child" && row.Fingerprint != "" {
				for _, old := range existing {
					if old.Fingerprint != row.Fingerprint || old.EmbeddingModelID != row.EmbeddingModelID || (row.Dimensions > 0 && old.Dimensions != row.Dimensions) {
						return fail(types.ErrVersionConflict, "index.write", "embedding endpoint or model changed; rebuild is required")
					}
				}
			}
		}
		if err = tx.Where("library_id = ? AND source_version_id = ?", lib.ID, sourceID).Delete(&types.EmbeddingChunkIndex{}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].LibraryID = lib.ID
			rows[i].SourceVersionID = sourceID
			rows[i].CreatedAt = time.Now().UnixMilli()
			rows[i].UpdatedAt = rows[i].CreatedAt
			if err = tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CompleteDocumentChunks 只在模型完整返回且任务围栏仍有效时原子写入真实向量。
func (s *Store) CompleteDocumentChunks(ctx context.Context, jobID, sourceID string, rows []types.EmbeddingChunkIndex) error {
	return s.transaction(ctx, func(tx *gorm.DB) error {
		job, lib, err := activeJob(tx, jobID)
		if err != nil {
			return err
		}
		if err = validateOrganizeSource(tx, job, sourceID); err != nil {
			return err
		}
		for _, row := range rows {
			if row.ChunkType != "child" {
				continue
			}
			result := tx.Model(&types.EmbeddingChunkIndex{}).Where("id = ? AND library_id = ? AND source_version_id = ? AND fingerprint = ? AND state = ?", row.ID, lib.ID, sourceID, row.Fingerprint, "pending").Updates(map[string]any{"vector": string(mustJSON(row.Vector)), "dimensions": row.Dimensions, "state": "indexed", "diagnostic": "", "updated_at": time.Now().UnixMilli()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fail(types.ErrVersionConflict, "index.complete", "chunk changed before embedding completed")
			}
		}
		return nil
	})
}

// FailDocumentChunks 持久记录失败；pending 或失败绝不作为可检索向量。
func (s *Store) FailDocumentChunks(ctx context.Context, libraryID, sourceID string) error {
	return s.transaction(ctx, func(tx *gorm.DB) error {
		return tx.Model(&types.EmbeddingChunkIndex{}).Where("library_id = ? AND source_version_id = ? AND state = ?", libraryID, sourceID, "pending").Updates(map[string]any{"state": "failed", "diagnostic": "embedding failed; retry the original job", "updated_at": time.Now().UnixMilli()}).Error
	})
}

// ListDocumentChunks 返回实际原文块，不包含被永久删除的来源。
func (s *Store) ListDocumentChunks(ctx context.Context, libraryID, sourceID string) ([]types.EmbeddingChunkIndex, error) {
	rows := []types.EmbeddingChunkIndex{}
	err := s.transaction(ctx, func(tx *gorm.DB) error {
		if _, e := getLibrary(tx, libraryID, true); e != nil {
			return e
		}
		if _, e := getSource(tx, libraryID, sourceID); e != nil {
			return e
		}
		return tx.Where("library_id = ? AND source_version_id = ?", libraryID, sourceID).Order("chunk_type ASC, sequence ASC").Find(&rows).Error
	})
	return rows, err
}

// SearchDocumentChunks 是 SQLite 本地存储适配：只比较同空间、同维度的已就绪向量。
func (s *Store) SearchDocumentChunks(ctx context.Context, libraryID, query string, vector []float32, fingerprint string, limit int) ([]types.DocumentChunkHit, error) {
	rows := []types.EmbeddingChunkIndex{}
	err := s.transaction(ctx, func(tx *gorm.DB) error {
		if _, e := getLibrary(tx, libraryID, true); e != nil {
			return e
		}
		return tx.Where("library_id = ? AND chunk_type = ? AND state IN ?", libraryID, "child", []string{"parsed", "indexed"}).Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	hits := []types.DocumentChunkHit{}
	for _, row := range rows {
		score := 0.0
		if len(vector) > 0 {
			if row.State != "indexed" {
				continue
			}
			if row.Fingerprint != fingerprint || row.Dimensions != len(vector) {
				return nil, fail(types.ErrVersionConflict, "index.search", "embedding endpoint, model or dimensions changed; rebuild is required")
			}
			score = cosineSimilarity(vector, row.Vector)
		} else {
			if query == "" || !strings.Contains(strings.ToLower(row.Content), strings.ToLower(query)) {
				continue
			}
			score = 1
		}
		hits = append(hits, types.DocumentChunkHit{Chunk: row, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Chunk.ID < hits[j].Chunk.ID
		}
		return hits[i].Score > hits[j].Score
	})
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// cosineSimilarity 原样迁自 WeKnora internal/types/memory.go 的 CosineSimilarity。
func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		normA += x * x
		normB += y * y
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GetDocumentChunkParents 仅在指定库内按最多五个实际命中身份读取父块。
func (s *Store) GetDocumentChunkParents(ctx context.Context, libraryID string, ids []string) ([]types.EmbeddingChunkIndex, error) {
	if len(ids) > 5 {
		return nil, &types.Error{Code: types.ErrInvalid, Operation: "index.parents", Detail: "parent lookup exceeds retrieval limit"}
	}
	rows := []types.EmbeddingChunkIndex{}
	err := s.transaction(ctx, func(tx *gorm.DB) error {
		if _, err := getLibrary(tx, libraryID, true); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Where("library_id = ? AND chunk_type = ? AND chunk_id IN ?", libraryID, "parent", ids).Find(&rows).Error
	})
	return rows, err
}
