package remote

import (
	"context"

	"github.com/JieWaZi/neux-knowledge/core"
)

// WriteAgentPage 使用远端实际Agent写入口，原请求身份用于未知结果恢复。
func (c *Custom) WriteAgentPage(ctx context.Context, in core.WritePageInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.writeMaintenance(ctx, "/pages/agent-write", in.RequestID, "page.agent-write", in.Page.ID, in)
}

// ReplacePageText 使用当前冻结远端资源执行精确替换。
func (c *Custom) ReplacePageText(ctx context.Context, in core.ReplacePageTextInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.PageID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return c.writeMaintenance(ctx, "/pages/"+in.PageID+"/replace-text", in.RequestID, "page.replace-text", in.PageID, in)
}

// RenamePage 保留远端页面身份并原子级联链接。
func (c *Custom) RenamePage(ctx context.Context, in core.RenamePageInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.PageID) || !safeSlug(in.NewSlug) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return c.writeMaintenance(ctx, "/pages/"+in.PageID+"/rename", in.RequestID, "page.rename", in.PageID, in)
}

// DeletePage 在远端归档页面并清理入链，未知结果保留原请求身份。
func (c *Custom) DeletePage(ctx context.Context, in core.DeletePageInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.PageID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return c.writeMaintenance(ctx, "/pages/"+in.PageID+"/delete", in.RequestID, "page.delete", in.PageID, in)
}

// writeMaintenance 校验页面维护动作、已知目标、已保存状态和正版本。
func (c *Custom) writeMaintenance(ctx context.Context, suffix, requestID, operation, resourceID string, in any) (*core.Receipt, error) {
	receipt, err := c.write(ctx, suffix, requestID, in)
	if err != nil {
		return nil, err
	}
	if receipt.Operation != operation || resourceID != "" && receipt.ResourceID != resourceID || receipt.State != "saved" || receipt.Version < 1 {
		return nil, &OutcomeUnknownError{RequestID: requestID}
	}
	return receipt, nil
}
