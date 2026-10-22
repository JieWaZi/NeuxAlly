package remote

import (
	"context"
	"net/url"

	"github.com/JieWaZi/neux-knowledge/core"
)

// PreviewSourcePurge 仅读取远端固定资源的实际清理影响，不建立本地副本。
func (c *Custom) PreviewSourcePurge(ctx context.Context, in core.PurgePreviewInput) (*core.SourcePurgePreview, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.SourceID) {
		return nil, &Error{Code: "invalid_request"}
	}
	p, err := customGet[core.SourcePurgePreview](ctx, c, "/source-purges/preview", url.Values{"sourceId": {in.SourceID}}, "libraryId", "sourceId", "versions", "digest", "blockingJobIds", "rawOwnership", "boundaries")
	if err == nil && (p.LibraryID != in.LibraryID || p.SourceID != in.SourceID || len(p.Digest) != 64 || p.PolicyVersion < 1) {
		return nil, protocolError()
	}
	return p, err
}

// ConfirmSourcePurge 只转交明确确认；结果未知时须使用同一 requestId 查询回执。
func (c *Custom) ConfirmSourcePurge(ctx context.Context, in core.ConfirmSourcePurgeInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.SourceID) || !in.Confirmed || len(in.PreviewDigest) != 64 {
		return nil, &Error{Code: "invalid_request"}
	}
	return c.write(ctx, "/source-purges/confirm", in.RequestID, in)
}

// GetSourcePurge 查询真实清理进度，包含共享保留和未知归属阻塞。
func (c *Custom) GetSourcePurge(ctx context.Context, purgeID string) (*core.SourcePurgeStatus, error) {
	if !safeID(purgeID) {
		return nil, &Error{Code: "invalid_request"}
	}
	p, err := customGet[core.SourcePurgeStatus](ctx, c, "/source-purges/"+purgeID, nil, "id", "libraryId", "sourceId", "state", "version", "versions", "boundaries")
	if err == nil && (p.ID != purgeID || p.LibraryID != c.ref.KnowledgeBaseID || p.Version < 1 || (p.State != "pending" && p.State != "completed" && p.State != "blocked")) {
		return nil, protocolError()
	}
	return p, err
}

// RecoverSourcePurge 使用远端当前授权继续原清单，不切换成本地删除。
func (c *Custom) RecoverSourcePurge(ctx context.Context, in core.RecoverSourcePurgeInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.PurgeID) || in.ExpectedPolicyVersion < 1 {
		return nil, &Error{Code: "invalid_request"}
	}
	return c.write(ctx, "/source-purges/"+in.PurgeID+"/recover", in.RequestID, in)
}

// ListSourcePurges 读取服务器权威清单，不依赖本机弹窗状态或来源仍然存在。
func (c *Custom) ListSourcePurges(ctx context.Context, offset, limit int) (*CustomList[core.SourcePurgeStatus], error) {
	p, err := customList[core.SourcePurgeStatus](ctx, c, "/source-purges", nil, offset, limit)
	if err != nil {
		return nil, err
	}
	for _, v := range p.Items {
		if v.LibraryID != c.ref.KnowledgeBaseID || !safeID(v.ID) || v.Version < 1 || (v.State != "pending" && v.State != "completed" && v.State != "blocked") {
			return nil, protocolError()
		}
	}
	return p, nil
}
