package core

import "context"

// ImportSource 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ImportSource(ctx context.Context, in ImportSourceInput) (*Receipt, error) {
	return e.store.ImportSource(ctx, in)
}

// ArchiveSource 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ArchiveSource(ctx context.Context, in ArchiveSourceInput) (*Receipt, error) {
	return e.store.ArchiveSource(ctx, in)
}

// GetSource 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetSource(ctx context.Context, libraryID, versionID string) (*SourceVersion, error) {
	return e.store.GetSource(ctx, libraryID, versionID)
}

// ListSources 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListSources(ctx context.Context, libraryID string, offset, limit int) ([]*SourceVersion, int64, error) {
	return e.store.ListSources(ctx, libraryID, offset, limit)
}

// ReadRaw 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ReadRaw(ctx context.Context, libraryID, versionID string) ([]byte, error) {
	return e.store.ReadRaw(ctx, libraryID, versionID)
}
