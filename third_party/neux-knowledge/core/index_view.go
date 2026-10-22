package core

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// WikiIndexResponse 保留上游intro与分页groups结构，避免重复存储完整目录Markdown。
type WikiIndexResponse = types.WikiIndexResponse

// WikiIndexGroup 暴露按页面类型独立分页的真实目录。
type WikiIndexGroup = types.WikiIndexGroup

// WikiIndexEntry 是不读取正文的轻量目录条目。
type WikiIndexEntry = types.WikiIndexEntry

// GetIndexView 读取全库结构化索引；无持久index时仍提供现有Wiki目录。
func (e *Engine) GetIndexView(ctx context.Context, libraryID string, pageTypes []string, limit int, cursor string) (*WikiIndexResponse, error) {
	return e.store.GetIndexView(ctx, libraryID, pageTypes, limit, cursor)
}
