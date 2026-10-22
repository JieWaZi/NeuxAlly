package core

import "context"

// CreateLibrary 按 SDK 合同执行对应知识领域操作。
func (e *Engine) CreateLibrary(ctx context.Context, in CreateLibraryInput) (*Receipt, error) {
	return e.store.CreateLibrary(ctx, in)
}

// UpdateLibrary 按 SDK 合同执行对应知识领域操作。
func (e *Engine) UpdateLibrary(ctx context.Context, in UpdateLibraryInput) (*Receipt, error) {
	return e.store.UpdateLibrary(ctx, in)
}

// GetLibrary 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetLibrary(ctx context.Context, id string) (*Library, error) {
	return e.store.GetLibrary(ctx, id)
}

// ListLibraries 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListLibraries(ctx context.Context, offset, limit int) ([]*Library, int64, error) {
	return e.store.ListLibraries(ctx, offset, limit)
}
