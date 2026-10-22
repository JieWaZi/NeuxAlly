package core

import (
	"context"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// 核查与导出复用内部只读模型，调用者无需导入 internal。
type (
	LintIssue          = types.LintIssue
	LintReport         = types.LintReport
	ExportInput        = types.ExportInput
	ExportSourceRef    = types.ExportSourceRef
	ExportPageMetadata = types.ExportPageMetadata
	MarkdownFile       = types.MarkdownFile
	MarkdownExport     = types.MarkdownExport
)

// LintLibrary 对一个一致快照执行只读核查，不自动修复或持久化问题。
func (e *Engine) LintLibrary(ctx context.Context, libraryID string) (*LintReport, error) {
	return e.store.LintLibrary(ctx, libraryID)
}

// ExportMarkdown 导出已发布知识的可移交副本；SQLite 始终是唯一权威。
func (e *Engine) ExportMarkdown(ctx context.Context, in ExportInput) (*MarkdownExport, error) {
	return e.store.ExportMarkdown(ctx, in)
}
