package remote

import (
	"context"
	"github.com/JieWaZi/neux-knowledge/core"
)

// DeleteLibrary 向固定远端库提交永久删除，不清理本机数据库。
func (c *Custom) DeleteLibrary(ctx context.Context, in core.DeleteLibraryInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if in.ExpectedVersion < 1 {
		return nil, &Error{Code: "invalid_request"}
	}
	return c.write(ctx, "/delete", in.RequestID, in)
}

// GetLibraryDeletion 查询已删除库的真实原件清理状态。
func (c *Custom) GetLibraryDeletion(ctx context.Context) (*core.LibraryDeletionStatus, error) {
	v, err := customGet[core.LibraryDeletionStatus](ctx, c, "/deletion", nil, "libraryId", "version", "policyVersion", "state", "deletedBlobCount", "retainedSharedBlobCount", "blockedBlobCount", "pendingBlobCount")
	if err == nil && (v.LibraryID != c.ref.KnowledgeBaseID || v.Version < 1 || v.PolicyVersion < 1 || (v.State != "pending" && v.State != "blocked" && v.State != "completed") || v.DeletedBlobCount < 0 || v.RetainedSharedBlobCount < 0 || v.BlockedBlobCount < 0 || v.PendingBlobCount < 0) {
		return nil, protocolError()
	}
	return v, err
}

// RecoverLibraryDeletion 以服务器清单版本恢复原件清理。
func (c *Custom) RecoverLibraryDeletion(ctx context.Context, in core.RecoverLibraryDeletionInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if in.ExpectedVersion < 1 {
		return nil, &Error{Code: "invalid_request"}
	}
	return c.write(ctx, "/deletion/recover", in.RequestID, in)
}
