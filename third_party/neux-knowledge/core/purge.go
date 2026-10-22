package core

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// 来源清理公开模型不要求宿主导入 internal。
type (
	PurgePreviewInput       = types.PurgePreviewInput
	PurgeVersion            = types.PurgeVersion
	PurgePageImpact         = types.PurgePageImpact
	SourcePurgePreview      = types.SourcePurgePreview
	ConfirmSourcePurgeInput = types.ConfirmSourcePurgeInput
	RecoverSourcePurgeInput = types.RecoverSourcePurgeInput
	SourcePurgeStatus       = types.SourcePurgeStatus
	SourcePurgeReview       = types.SourcePurgeReview
)

// ParseSourcePurgeReview 白名单读取页面已有清理身份，供宿主安全展示及导出。
func ParseSourcePurgeReview(metadata JSON) *SourcePurgeReview {
	return types.ParseSourcePurgeReview(metadata)
}

// PreviewSourcePurge 预览一个来源所有版本及内部副本的实际影响，不执行删除。
func (e *Engine) PreviewSourcePurge(ctx context.Context, in PurgePreviewInput) (*SourcePurgePreview, error) {
	return e.store.PreviewSourcePurge(ctx, in)
}

// ConfirmSourcePurge 核对预览后原子撤销来源并创建清理清单；回执只表示已接纳。
func (e *Engine) ConfirmSourcePurge(ctx context.Context, in ConfirmSourcePurgeInput) (*Receipt, error) {
	return e.store.ConfirmSourcePurge(ctx, in)
}

// GetSourcePurge 读取原件实际删除、共享保留和未知归属阻塞事实。
func (e *Engine) GetSourcePurge(ctx context.Context, libraryID, purgeID string) (*SourcePurgeStatus, error) {
	return e.store.GetSourcePurge(ctx, libraryID, purgeID)
}

// RecoverSourcePurge 使用当前授权继续同一清单；可在重开核心后独立运行。
func (e *Engine) RecoverSourcePurge(ctx context.Context, in RecoverSourcePurgeInput) (*Receipt, error) {
	return e.store.RecoverSourcePurge(ctx, in)
}
