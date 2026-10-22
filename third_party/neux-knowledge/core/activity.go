package core

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// AuditLog 直接复用不可变活动领域模型。
type AuditLog = types.AuditLog

// AuditAction 沿用固定上游的动作值。
type AuditAction = types.AuditAction

// AuditOutcome 区分接纳、成功、失败、部分成功、取消与拒绝。
type AuditOutcome = types.AuditOutcome

// ActivityQuery 保留上游游标与精确过滤合同。
type ActivityQuery = types.ActivityQuery

// ActivityActor 是受信宿主提供的真实发起身份。
type ActivityActor = types.ActivityActor

const (
	AuditActionKBCreated               = types.AuditActionKBCreated
	AuditActionKBUpdated               = types.AuditActionKBUpdated
	AuditActionKBDeleted               = types.AuditActionKBDeleted
	AuditActionKnowledgeCreated        = types.AuditActionKnowledgeCreated
	AuditActionKnowledgeUpdated        = types.AuditActionKnowledgeUpdated
	AuditActionKnowledgeDeleted        = types.AuditActionKnowledgeDeleted
	AuditActionKnowledgeReparseStarted = types.AuditActionKnowledgeReparseStarted
	AuditActionKnowledgeParseCanceled  = types.AuditActionKnowledgeParseCanceled
	AuditActionWikiContentChanged      = types.AuditActionWikiContentChanged
	AuditOutcomeSuccess                = types.AuditOutcomeSuccess
	AuditOutcomeAccepted               = types.AuditOutcomeAccepted
	AuditOutcomeDenied                 = types.AuditOutcomeDenied
	AuditOutcomeFailed                 = types.AuditOutcomeFailed
	AuditOutcomePartial                = types.AuditOutcomePartial
	AuditOutcomeCanceled               = types.AuditOutcomeCanceled
)

// WithActivityActor 将宿主确认的真实身份附在本次调用上下文。
func WithActivityActor(ctx context.Context, actor ActivityActor) context.Context {
	return types.WithActivityActor(ctx, actor)
}

// ListLibraryActivity 直接查询持久活动，不从任务或回执推算历史。
// 游标按上游语义取本页最后 ID，空页返回零。
func (e *Engine) ListLibraryActivity(ctx context.Context, libraryID string, q ActivityQuery) (items []*AuditLog, nextCursor uint64, err error) {
	items, err = e.store.ListLibraryActivity(ctx, libraryID, &q)
	if err == nil && len(items) > 0 {
		nextCursor = items[len(items)-1].ID
	}
	return
}

// RecordLibraryExecutionDefaultsActivity 仅为宿主已提交的执行默认设置记录固定活动。
// 相同请求可安全重试，既有库版本不变；不能提交任意活动负载。
func (e *Engine) RecordLibraryExecutionDefaultsActivity(ctx context.Context, libraryID, requestID string) error {
	return e.store.RecordLibraryExecutionDefaultsActivity(ctx, libraryID, requestID)
}
