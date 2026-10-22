package core

import "context"

// SourcePurgeList 是不含原文和内部文件键的权威清理清单窗口。
type SourcePurgeList struct {
	Items []SourcePurgeStatus `json:"items"` // 清理进度窗口。
	Total int64               `json:"total"` // 同库记录总数。
}

// ListSourcePurges 允许关闭弹窗或重启后继续查询与恢复原清理。
func (e *Engine) ListSourcePurges(ctx context.Context, libraryID string, offset, limit int) (*SourcePurgeList, error) {
	rows, total, err := e.store.ListSourcePurges(ctx, libraryID, offset, limit)
	if err != nil {
		return nil, err
	}
	return &SourcePurgeList{Items: rows, Total: total}, nil
}
