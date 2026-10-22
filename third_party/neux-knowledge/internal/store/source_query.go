package store

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	gosqlite "github.com/glebarez/go-sqlite"
	"gorm.io/gorm"
)

// 注册发生在数据库连接建立之前；SQLite 内置 lower 仅支持 ASCII，不能复用前端的 Unicode 搜索语义。
var sourceContainsRegistration = gosqlite.RegisterDeterministicScalarFunction("neux_source_contains", 2, func(_ *gosqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	filename, _ := args[0].(string)
	query, _ := args[1].(string)
	if strings.Contains(strings.ToLower(filename), strings.ToLower(query)) {
		return int64(1), nil
	}
	return int64(0), nil
})

// sourceActivity 保存当前尝试的轻量状态覆盖，不加载原件内容。
type sourceActivity struct {
	State      string // 当前尝试的展示状态。
	Diagnostic string // 当前死信诊断，其余在途状态清空旧诊断。
}

// sourceActivities 仅以占槽任务和当前尝试待办确定覆盖，done 保留来源终态。
func sourceActivities(tx *gorm.DB, libraryID string) (map[string]sourceActivity, error) {
	var jobs []types.Job
	if err := tx.Select("id,attempt,source_version_ids").Where("library_id = ? AND state IN ?", libraryID, []string{"queued", "processing", "cancel_requested"}).Limit(1).Find(&jobs).Error; err != nil {
		return nil, err
	}
	result := map[string]sourceActivity{}
	if len(jobs) == 0 {
		return result, nil
	}
	j := jobs[0]
	var work []types.OrganizeSourceWork
	if err := tx.Select("source_version_id,state,diagnostic,started_at").Where("job_id = ? AND job_attempt = ?", j.ID, j.Attempt).Find(&work).Error; err != nil {
		return nil, err
	}
	current := make(map[string]types.OrganizeSourceWork, len(work))
	for _, w := range work {
		current[w.SourceVersionID] = w
	}
	for _, id := range j.SourceVersionIDs {
		w, ok := current[id]
		if ok && w.State == "done" {
			continue
		}
		activity := sourceActivity{State: "queued"}
		if ok && w.State == "dead" {
			activity.State = "failed"
			activity.Diagnostic = w.Diagnostic
		} else if ok && w.StartedAt > 0 {
			activity.State = "processing"
		}
		result[id] = activity
	}
	return result, nil
}

func projectSourceActivity(sources []*types.SourceVersion, activity map[string]sourceActivity) []*types.SourceVersion {
	result := make([]*types.SourceVersion, 0, len(sources))
	for _, s := range sources {
		if s == nil {
			continue
		}
		v := *s
		if a, ok := activity[s.ID]; ok && !s.Archived && s.State != "processed" && s.State != "no-change" {
			v.State = a.State
			v.Diagnostic = a.Diagnostic
		}
		result = append(result, &v)
	}
	return result
}

// SourceActivity 复用列表状态规则，原件终态与归档状态始终优先。
func (s *Store) SourceActivity(ctx context.Context, libraryID string, sources []*types.SourceVersion) (items []*types.SourceVersion, err error) {
	err = s.transaction(ctx, func(tx *gorm.DB) error {
		if _, err := getLibrary(tx, libraryID, false); err != nil {
			return err
		}
		for _, v := range sources {
			if v != nil && v.LibraryID != libraryID {
				return fail(types.ErrScopeViolation, "source.activity", "source belongs to another library")
			}
		}
		activity, e := sourceActivities(tx, libraryID)
		if e != nil {
			return e
		}
		items = projectSourceActivity(sources, activity)
		return nil
	})
	return
}

// ListSourcesQuery 使用单表条件完成展示状态筛选，计数与分页共享快照。
func (s *Store) ListSourcesQuery(ctx context.Context, libraryID string, in types.SourceQuery) (items []*types.SourceVersion, total int64, err error) {
	if sourceContainsRegistration != nil {
		return nil, 0, sourceContainsRegistration
	}
	state := strings.TrimSpace(in.State)
	if state == "" {
		state = "all"
	}
	if state == "unchanged" {
		state = "no-change"
	}
	switch state {
	case "all", "pending", "queued", "processing", "processed", "no-change", "failed", "cancelled":
	default:
		return nil, 0, fail(types.ErrInvalid, "source.list", "invalid source state")
	}
	order := "created_at DESC, id DESC"
	switch in.Sort {
	case "", "newest":
	case "oldest":
		order = "created_at ASC, id ASC"
	default:
		return nil, 0, fail(types.ErrInvalid, "source.list", "invalid source sort")
	}
	offset, limit := pagination(in.Offset, in.Limit)
	err = s.transaction(ctx, func(tx *gorm.DB) error {
		if _, e := getLibrary(tx, libraryID, false); e != nil {
			return e
		}
		activity, e := sourceActivities(tx, libraryID)
		if e != nil {
			return e
		}
		q := tx.Model(&types.SourceVersion{}).Where("library_id = ? AND archived = ?", libraryID, false)
		if in.Query != "" {
			q = q.Where("neux_source_contains(filename, ?) = 1", in.Query)
		}
		if state != "all" {
			ids := map[string][]string{}
			for id, a := range activity {
				ids[a.State] = append(ids[a.State], id)
			}
			// 每组身份只绑定一个 JSON 参数，避免活动来源较多时超过 SQLite 变量上限。
			encoded := make(map[string]string, 3)
			for _, key := range []string{"failed", "processing", "queued"} {
				values, err := json.Marshal(ids[key])
				if err != nil {
					return err
				}
				encoded[key] = string(values)
			}
			q = q.Where("CASE WHEN state IN ('processed','no-change') THEN state WHEN id IN (SELECT value FROM json_each(?)) THEN 'failed' WHEN id IN (SELECT value FROM json_each(?)) THEN 'processing' WHEN id IN (SELECT value FROM json_each(?)) THEN 'queued' ELSE state END = ?", encoded["failed"], encoded["processing"], encoded["queued"], state)
		}
		if e = q.Count(&total).Error; e != nil {
			return e
		}
		if e = q.Order(order).Offset(offset).Limit(limit).Find(&items).Error; e != nil {
			return e
		}
		items = projectSourceActivity(items, activity)
		return nil
	})
	return
}
