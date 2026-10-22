package core

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// SourceQuery 指定未归档来源的展示状态筛选与全局分页。
type SourceQuery = types.SourceQuery

// ListSourcesQuery 在同一读取事务中按展示状态筛选、计数与分页。
func (e *Engine) ListSourcesQuery(ctx context.Context, libraryID string, query SourceQuery) ([]*SourceVersion, int64, error) {
	return e.store.ListSourcesQuery(ctx, libraryID, query)
}

// SourceActivity 复制来源并投影当前任务尝试，不修改原始持久对象。
func (e *Engine) SourceActivity(ctx context.Context, libraryID string, sources []*SourceVersion) ([]*SourceVersion, error) {
	return e.store.SourceActivity(ctx, libraryID, sources)
}
