package store

import (
	"context"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"gorm.io/gorm"
)

// ListSourcePurges 从权威清单恢复已接纳操作入口，来源被移除后仍可查询进度。
func (s *Store) ListSourcePurges(ctx context.Context, libraryID string, offset, limit int) (items []types.SourcePurgeStatus, total int64, err error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, fail(types.ErrInvalid, "source.purge.list", "invalid pagination")
	}
	items = []types.SourcePurgeStatus{}
	err = s.transaction(ctx, func(tx *gorm.DB) error {
		if _, e := getLibrary(tx, libraryID, false); e != nil {
			return e
		}
		q := tx.Model(&types.SourcePurgeRecord{}).Where("library_id = ?", libraryID)
		if e := q.Count(&total).Error; e != nil {
			return e
		}
		var records []types.SourcePurgeRecord
		if e := q.Order("created_at DESC, id ASC").Offset(offset).Limit(limit).Find(&records).Error; e != nil {
			return e
		}
		for i := range records {
			items = append(items, *purgeStatus(&records[i]))
		}
		return nil
	})
	return
}
