package core

import "context"

// AcceptJob 按 SDK 合同执行对应知识领域操作。
func (e *Engine) AcceptJob(ctx context.Context, in AcceptJobInput) (*Receipt, error) {
	return e.store.AcceptJob(ctx, in)
}

// GetJob 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetJob(ctx context.Context, jobID string) (*Job, error) {
	return e.store.GetJob(ctx, jobID)
}

// ListJobs 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListJobs(ctx context.Context, libraryID string, offset, limit int) ([]*Job, int64, error) {
	return e.store.ListJobs(ctx, libraryID, offset, limit)
}

// RetryJob 按 SDK 合同执行对应知识领域操作。
func (e *Engine) RetryJob(ctx context.Context, in JobActionInput) (*Receipt, error) {
	return e.store.RetryJob(ctx, in)
}

// GetStage 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetStage(ctx context.Context, stageID string) (*Stage, error) {
	return e.store.GetStage(ctx, stageID)
}

// ListStages 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListStages(ctx context.Context, jobID string) ([]*Stage, error) {
	return e.store.ListStages(ctx, jobID)
}

// GetReceipt 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetReceipt(ctx context.Context, requestID string) (*Receipt, error) {
	return e.store.GetReceipt(ctx, requestID)
}

// FailJob 如实结算失败任务并保留已发布部分。
func (e *Engine) FailJob(ctx context.Context, in FailJobInput) (*Receipt, error) {
	return e.store.FailJob(ctx, in)
}

// GetActiveJob 返回同库唯一占槽任务；没有活跃任务时返回nil。
func (e *Engine) GetActiveJob(ctx context.Context, libraryID string) (*Job, error) {
	return e.store.GetActiveJob(ctx, libraryID)
}
