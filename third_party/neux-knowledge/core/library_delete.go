package core

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

type DeleteLibraryInput = types.DeleteLibraryInput
type RecoverLibraryDeletionInput = types.RecoverLibraryDeletionInput
type LibraryDeletionStatus = types.LibraryDeletionStatus

// DeleteLibrary 永久移除知识内容并按持久清单清理受管原件。
func (e *Engine) DeleteLibrary(ctx context.Context, in DeleteLibraryInput) (*Receipt, error) {
	return e.store.DeleteLibrary(ctx, in)
}

// GetLibraryDeletion 读取删除后仍然存在的最小清理状态。
func (e *Engine) GetLibraryDeletion(ctx context.Context, id string) (*LibraryDeletionStatus, error) {
	return e.store.GetLibraryDeletion(ctx, id)
}

// RecoverLibraryDeletion 继续已确认整库删除的原件清理。
func (e *Engine) RecoverLibraryDeletion(ctx context.Context, in RecoverLibraryDeletionInput) (*Receipt, error) {
	return e.store.RecoverLibraryDeletion(ctx, in)
}
