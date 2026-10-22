package knowledge

import (
	"context"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/JieWaZi/neux-knowledge/remote"
	"math"
	"strings"
)

// ReadProvider 只适配同一知识读取合同，不暴露本机生成、写入或索引端口。
type ReadProvider interface {
	Library(context.Context, ResourceRef) (*core.Library, error)
	Pages(context.Context, ResourceRef, core.PageQuery) ([]*core.Page, int64, error)
	Page(context.Context, ResourceRef, string, string) (*core.Page, error)
	Source(context.Context, ResourceRef, string) (map[string]any, error)
}

// LocalReadProvider 仅转调唯一独立SDK。
type LocalReadProvider struct {
	// Engine 是独立核心权威，不持有业务副本。
	Engine *core.Engine
	// FreezeEmbedder 冻结同一库独立配置，查询失败不回落到关键词。
	FreezeEmbedder func(context.Context, string, string) (core.DocumentEmbedder, error)
}

func (p LocalReadProvider) Library(ctx context.Context, ref ResourceRef) (*core.Library, error) {
	if err := RequireLocal(ref); err != nil {
		return nil, err
	}
	v, err := p.Engine.GetLibrary(ctx, ref.KnowledgeBaseID)
	return v, MapError(err)
}
func (p LocalReadProvider) Pages(ctx context.Context, ref ResourceRef, q core.PageQuery) ([]*core.Page, int64, error) {
	if err := RequireLocal(ref); err != nil {
		return nil, 0, err
	}
	q.LibraryID = ref.KnowledgeBaseID
	v, total, err := p.Engine.ListPages(ctx, q)
	return v, total, MapError(err)
}

// DocumentReadHit 保留命中子块的定位，并返回父块上下文。
type DocumentReadHit struct {
	Chunk  core.IndexedDocumentChunk  `json:"chunk"`            // 命中子块及真实来源版本。
	Score  float64                    `json:"score"`            // 实际向量相似度或关键词分数。
	Parent *core.IndexedDocumentChunk `json:"parent,omitempty"` // 父子模式的完整父块上下文。
}

// DocumentReadProvider 为既有 Wiki 搜索补充真实原件检索，不改变远端读取端口。
type DocumentReadProvider interface {
	SearchChunks(context.Context, ResourceRef, string) ([]DocumentReadHit, string, error)
}

// SearchChunks 按实际库策略检索原件，父块来自同一来源版本的持久结果。
func (p LocalReadProvider) SearchChunks(ctx context.Context, ref ResourceRef, query string) ([]DocumentReadHit, string, error) {
	lib, err := p.Library(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(query) == "" || (!lib.IndexingStrategy.VectorEnabled && !lib.IndexingStrategy.KeywordEnabled) {
		return nil, "", nil
	}
	mode := "keyword"
	var vector []float32
	fingerprint := ""
	if lib.IndexingStrategy.VectorEnabled {
		mode = "vector"
		if p.FreezeEmbedder == nil {
			return nil, "", status_error.KnowledgeEmbeddingNotConfigured
		}
		frozen, e := p.FreezeEmbedder(ctx, lib.ID, lib.EmbeddingModelID)
		if e != nil {
			return nil, "", e
		}
		if frozen.Embed == nil || frozen.Fingerprint == "" {
			return nil, "", status_error.KnowledgeEmbeddingNotConfigured
		}
		values, e := frozen.Embed(ctx, []string{query})
		if e != nil {
			return nil, "", e
		}
		if len(values) != 1 || len(values[0]) == 0 {
			return nil, "", status_error.KnowledgeEmbeddingResponseInvalid
		}
		norm := 0.0
		for _, x := range values[0] {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, "", status_error.KnowledgeEmbeddingResponseInvalid
			}
			norm += float64(x) * float64(x)
		}
		if norm == 0 {
			return nil, "", status_error.KnowledgeEmbeddingResponseInvalid
		}
		vector = values[0]
		fingerprint = frozen.Fingerprint
	}
	hits, err := p.Engine.SearchDocumentChunks(ctx, ref.KnowledgeBaseID, query, vector, fingerprint, 5)
	if err != nil {
		return nil, "", MapError(err)
	}
	out := make([]DocumentReadHit, 0, len(hits))
	parentIDs := []string{}
	for _, hit := range hits {
		if hit.Chunk.ParentID != "" {
			parentIDs = append(parentIDs, hit.Chunk.ParentID)
		}
	}
	parents := map[string]core.IndexedDocumentChunk{}
	if len(parentIDs) > 0 {
		rows, e := p.Engine.GetDocumentChunkParents(ctx, ref.KnowledgeBaseID, parentIDs)
		if e != nil {
			return nil, "", MapError(e)
		}
		for _, row := range rows {
			parents[row.ChunkID] = row
		}
	}
	for _, hit := range hits {
		item := DocumentReadHit{Chunk: hit.Chunk, Score: hit.Score}
		if hit.Chunk.ParentID != "" {
			parent, ok := parents[hit.Chunk.ParentID]
			if !ok || parent.SourceVersionID != hit.Chunk.SourceVersionID {
				return nil, "", status_error.KnowledgeStorageError
			}
			item.Parent = &parent
		}
		out = append(out, item)
	}
	return out, mode, nil
}

func (p LocalReadProvider) Page(ctx context.Context, ref ResourceRef, id, slug string) (*core.Page, error) {
	if err := RequireLocal(ref); err != nil {
		return nil, err
	}
	v, err := p.Engine.GetPage(ctx, ref.KnowledgeBaseID, id)
	return v, MapError(err)
}
func (p LocalReadProvider) Source(ctx context.Context, ref ResourceRef, id string) (map[string]any, error) {
	if err := RequireLocal(ref); err != nil {
		return nil, err
	}
	v, err := p.Engine.GetSource(ctx, ref.KnowledgeBaseID, id)
	if err != nil {
		return nil, MapError(err)
	}
	return map[string]any{"sourceId": v.SourceID, "sourceVersionId": v.ID, "sha256": v.SHA256, "filename": v.Filename, "mime": v.MIME, "state": v.State, "coverage": v.Coverage, "diagnostic": v.Diagnostic}, nil
}

// RemoteReadProvider 只有HTTP客户端，不能触达本机Reader、Generator或索引。
type RemoteReadProvider struct {
	// Custom 是绑定完整远端引用的自有协议客户端。
	Custom *remote.Custom
	// Official 是官方固定合同客户端。
	Official *remote.WeKnora
}

func (p RemoteReadProvider) Library(ctx context.Context, ref ResourceRef) (*core.Library, error) {
	if p.Custom != nil {
		v, err := p.Custom.GetLibrary(ctx)
		return v, MapRemoteError(err)
	}
	if p.Official == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	v, err := p.Official.GetLibrary(ctx, ref.KnowledgeBaseID)
	if err != nil {
		return nil, MapRemoteError(err)
	}
	return &core.Library{ID: v.ID, Name: v.Name, Kind: "official"}, nil
}
func (p RemoteReadProvider) Pages(ctx context.Context, ref ResourceRef, q core.PageQuery) ([]*core.Page, int64, error) {
	if p.Custom != nil {
		v, err := p.Custom.ListPages(ctx, q)
		if err != nil {
			return nil, 0, MapRemoteError(err)
		}
		return v.Items, v.Total, nil
	}
	if p.Official == nil {
		return nil, 0, status_error.KnowledgeUnsupported
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Offset%q.Limit != 0 {
		return nil, 0, status_error.KnowledgeInvalid
	}
	v, err := p.Official.ListPages(ctx, ref.KnowledgeBaseID, remote.PageOptions{Page: q.Offset/q.Limit + 1, PageSize: q.Limit, Query: q.Query, PageType: q.PageType, FolderID: q.ThemeID})
	if err != nil {
		return nil, 0, MapRemoteError(err)
	}
	items := []*core.Page{}
	for _, page := range v.Pages {
		items = append(items, officialReadPage(page))
	}
	return items, v.Total, nil
}
func (p RemoteReadProvider) Page(ctx context.Context, ref ResourceRef, id, slug string) (*core.Page, error) {
	if p.Custom != nil {
		v, err := p.Custom.GetPage(ctx, id)
		return v, MapRemoteError(err)
	}
	if p.Official == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	if slug == "" {
		return nil, status_error.KnowledgeInvalid
	}
	v, err := p.Official.GetPage(ctx, ref.KnowledgeBaseID, slug)
	if err != nil {
		return nil, MapRemoteError(err)
	}
	if v.ID != id {
		return nil, status_error.KnowledgeNotFound
	}
	return officialReadPage(*v), nil
}
func (p RemoteReadProvider) Source(ctx context.Context, ref ResourceRef, id string) (map[string]any, error) {
	if p.Custom != nil {
		v, err := p.Custom.GetSource(ctx, id)
		if err != nil {
			return nil, MapRemoteError(err)
		}
		return map[string]any{"sourceId": v.SourceID, "sourceVersionId": v.ID, "sha256": v.SHA256, "filename": v.Filename, "mime": v.MIME, "state": v.State, "coverage": v.Coverage, "diagnostic": v.Diagnostic}, nil
	}
	if p.Official == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	v, err := p.Official.GetSource(ctx, ref.KnowledgeBaseID, id)
	if err != nil {
		return nil, MapRemoteError(err)
	}
	return map[string]any{"sourceId": v.ID, "filename": v.Filename, "state": v.ParseStatus, "versioning": "unavailable"}, nil
}
func officialReadPage(v remote.Page) *core.Page {
	return &core.Page{KnowledgeBaseID: v.KnowledgeBaseID, ID: v.ID, Slug: v.Slug, Title: v.Title, PageType: v.PageType, Content: v.Content, Version: v.Version, FolderID: v.FolderID, Aliases: v.Aliases, SourceRefs: v.SourceRefs, InLinks: v.InLinks, OutLinks: v.OutLinks}
}

// MapRemoteError 在SDK首个外部边界保留稳定分类，不泄露远端响应或凭据。
func MapRemoteError(err error) error {
	if err == nil {
		return nil
	}
	var e *remote.Error
	if errors.As(err, &e) {
		switch e.Code {
		case "unauthenticated":
			return status_error.KnowledgeRemoteUnauthorized
		case "forbidden", "scope_violation", "policy_changed", "write_ip_forbidden":
			return status_error.KnowledgeForbidden
		case "not_found":
			return status_error.KnowledgeNotFound
		case "version_conflict", "idempotency_conflict", "manual_protected":
			return status_error.KnowledgeConflict
		case "unsupported", "unsupported_profile":
			return status_error.KnowledgeUnsupported
		case "invalid", "invalid_connection", "invalid_resource", "invalid_request", "invalid_query", "invalid_pagination":
			return status_error.KnowledgeInvalid
		case "outcome_unknown":
			return status_error.KnowledgeRemoteOutcomeUnknown
		case "protocol_incompatible", "redirect_rejected", "response_too_large", "response_incomplete":
			return status_error.KnowledgeRemoteIncompatible
		}
	}
	return status_error.KnowledgeRemoteUnreachable
}
