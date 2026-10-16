package controllers

import (
	"container/heap"
	"sort"

	"gorm.io/gorm"
)

// repositoryReadBatch 限制关联查询参数数量，避免逐行读取或超过 SQLite 参数上限。
const repositoryReadBatch = 256

// readRelatedRows 仅按已知候选身份分批读取指定字段，不加载无关历史。
func readRelatedRows[T any](db *gorm.DB, column, fields string, ids []string) ([]T, error) {
	unique := uniqueRowIDs(ids)
	result := []T{}
	for start := 0; start < len(unique); start += repositoryReadBatch {
		var rows []T
		if err := db.Session(&gorm.Session{}).Select(fields).Where(column+" IN ?", unique[start:min(start+repositoryReadBatch, len(unique))]).Find(&rows).Error; err != nil {
			return nil, databaseError("read_related_rows", column, err)
		}
		result = append(result, rows...)
	}
	return result, nil
}

// uniqueRowIDs 排除空关联并去重，避免空外键被误认作业务来源。
func uniqueRowIDs(ids []string) []string {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}

// walkRepositoryRows 使用主键续批；回调完成后释放当前批，不使用越来越深的 OFFSET。
func walkRepositoryRows[T any](query *gorm.DB, visit func([]T) error) error {
	var rows []T
	var callbackErr error
	result := query.FindInBatches(&rows, repositoryReadBatch, func(_ *gorm.DB, _ int) error { callbackErr = visit(rows); return callbackErr })
	if callbackErr != nil {
		return callbackErr
	}
	if result.Error != nil {
		return databaseError("walk_repository_rows", "", result.Error)
	}
	return nil
}

// repositoryPage 维护分页候选的最大堆；扫描过程不收集完整领域记录。
type repositoryPage[T any] struct {
	// rows 是当前最靠前的候选窗口，堆顶为其中最后一项。
	rows []T
	// before 比较业务排序，不依赖输入扫描顺序。
	before func(T, T) bool
	// offset 是显式跳过的候选数量。
	offset int
	// limit 是页面及续页探测需要的数量。
	limit int
}

func newRepositoryPage[T any](offset, limit int, before func(T, T) bool) *repositoryPage[T] {
	return &repositoryPage[T]{before: before, offset: max(offset, 0), limit: max(limit, 1)}
}

// Len 返回候选窗口大小。
func (p repositoryPage[T]) Len() int { return len(p.rows) }

// Less 将业务顺序最后的候选放在堆顶。
func (p repositoryPage[T]) Less(i, j int) bool { return p.before(p.rows[j], p.rows[i]) }

// Swap 交换两个候选的位置。
func (p repositoryPage[T]) Swap(i, j int) { p.rows[i], p.rows[j] = p.rows[j], p.rows[i] }

// Push 加入排序候选。
func (p *repositoryPage[T]) Push(v any) { p.rows = append(p.rows, v.(T)) }

// Pop 移除堆尾候选。
func (p *repositoryPage[T]) Pop() any {
	last := len(p.rows) - 1
	v := p.rows[last]
	p.rows = p.rows[:last]
	return v
}
func (p *repositoryPage[T]) add(v T) {
	if len(p.rows) < p.offset+p.limit {
		heap.Push(p, v)
	} else if p.before(v, p.rows[0]) {
		p.rows[0] = v
		heap.Fix(p, 0)
	}
}
func (p *repositoryPage[T]) page() []T {
	sort.Slice(p.rows, func(i, j int) bool { return p.before(p.rows[i], p.rows[j]) })
	return p.rows[min(p.offset, len(p.rows)):]
}
