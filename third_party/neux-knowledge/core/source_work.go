package core

import (
	"context"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// OrganizeSourceWork 公开同次任务内的来源重排事实。
type OrganizeSourceWork = types.OrganizeSourceWork

// SourceWorkActionInput 公开来源待办的幂等延后输入。
type SourceWorkActionInput = types.SourceWorkActionInput

// ErrSourceUnavailable 表示已冻结来源失活，可按文档局部结算，不代表权限撤销。
const ErrSourceUnavailable = types.ErrSourceUnavailable

// EnsureOrganizeSourceWork 幂等登记当前任务尝试的冻结来源。
func (e *Engine) EnsureOrganizeSourceWork(ctx context.Context, jobID string) error {
	return e.store.EnsureOrganizeSourceWork(ctx, jobID)
}

// ListOrganizeSourceWork 返回当前尝试的待办、完成与死信来源。
func (e *Engine) ListOrganizeSourceWork(ctx context.Context, jobID string) ([]*OrganizeSourceWork, error) {
	return e.store.ListOrganizeSourceWork(ctx, jobID)
}

// DeferOrganizeSourceWork 幂等登记一次失败及来源局部重试时间。
func (e *Engine) DeferOrganizeSourceWork(ctx context.Context, in SourceWorkActionInput) (*Receipt, error) {
	return e.store.DeferOrganizeSourceWork(ctx, in)
}
