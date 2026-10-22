package core

import "context"

// WritePage 按 SDK 合同执行对应知识领域操作。
func (e *Engine) WritePage(ctx context.Context, in WritePageInput) (*Receipt, error) {
	return e.store.WritePage(ctx, in)
}

// RestorePage 按 SDK 合同执行对应知识领域操作。
func (e *Engine) RestorePage(ctx context.Context, in RestorePageInput) (*Receipt, error) {
	return e.store.RestorePage(ctx, in)
}

// GetPage 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetPage(ctx context.Context, libraryID, pageID string) (*Page, error) {
	return e.store.GetPage(ctx, libraryID, pageID)
}

// GetPageBySlug 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetPageBySlug(ctx context.Context, libraryID, slug string) (*Page, error) {
	return e.store.GetPageBySlug(ctx, libraryID, slug)
}

// ListPages 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListPages(ctx context.Context, q PageQuery) ([]*Page, int64, error) {
	return e.store.ListPages(ctx, q)
}

// ListRevisions 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListRevisions(ctx context.Context, libraryID, pageID string, offset, limit int) ([]*Revision, int64, error) {
	return e.store.ListRevisions(ctx, libraryID, pageID, offset, limit)
}

// GetRevision 按 SDK 合同执行对应知识领域操作。
func (e *Engine) GetRevision(ctx context.Context, libraryID, pageID string, version int) (*Revision, error) {
	return e.store.GetRevision(ctx, libraryID, pageID, version)
}

// WriteTheme 按 SDK 合同执行对应知识领域操作。
func (e *Engine) WriteTheme(ctx context.Context, in WriteThemeInput) (*Receipt, error) {
	return e.store.WriteTheme(ctx, in)
}

// ListThemes 按 SDK 合同执行对应知识领域操作。
func (e *Engine) ListThemes(ctx context.Context, libraryID string) ([]*Theme, error) {
	return e.store.ListThemes(ctx, libraryID)
}
