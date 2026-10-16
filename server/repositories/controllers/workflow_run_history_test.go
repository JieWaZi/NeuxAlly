package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/shrewx/ginx/pkg/dbhelper"
	"gorm.io/gorm"
)

// seedRunHistory 用真实 SQLite 行构造确定的历史顺序，不绕过待验证的清理入口。
func seedRunHistory(t *testing.T, db *gorm.DB, definition string, count int, status constants.WorkflowRunStatus) {
	t.Helper()
	for index := 0; index < count; index++ {
		run := models.WorkflowRun{ID: fmt.Sprintf("%s-%03d", definition, index), DefinitionID: definition, Status: status, CreatedAt: 100, UpdatedAt: 100, Version: 1, LeaseOwner: "worker", Fence: 1, LeaseUntil: 1000}
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunHistoryPrunesOldestHalfOnlyAboveLimit(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	runs := NewWorkflowRunController(db)
	seedRunHistory(t, db, "flow", 29, constants.WorkflowRunStatusSucceeded)
	seedRunHistory(t, db, "other", 2, constants.WorkflowRunStatusSucceeded)
	create := func(id string) {
		t.Helper()
		err := NewTransactionController(dbhelper.NewGormTransactionManager(db)).Transaction(ctx, func(ctx context.Context) error {
			return runs.CreateRun(ctx, models.WorkflowRun{ID: id, DefinitionID: "flow", Status: constants.WorkflowRunStatusQueued, CreatedAt: 200}, models.WorkflowFiber{ID: id + "-fiber", RunID: id})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	create("flow-new-1")
	values, err := runs.ListRuns(ctx, "flow", 0, "", 100)
	if err != nil || len(values) != 30 {
		t.Fatalf("30 条时不得清理: %d %v", len(values), err)
	}
	create("flow-new-2")
	values, err = runs.ListRuns(ctx, "flow", 0, "", 100)
	if err != nil || len(values) != 16 || values[len(values)-1].ID != "flow-015" {
		t.Fatalf("31 条应清理最早 15 条，同时间按身份稳定排序: %+v %v", values, err)
	}
	old, err := runs.GetRun(ctx, "flow-000")
	if err != nil || old.HistoryDeletedAt == nil || old.Version != 1 || old.UpdatedAt != 100 {
		t.Fatalf("清理不得改写运行事实或活动时间: %+v %v", old, err)
	}
	other, err := runs.ListRuns(ctx, "other", 0, "", 100)
	if err != nil || len(other) != 2 {
		t.Fatalf("不能影响其他工作流: %+v %v", other, err)
	}
	var events []models.WorkflowEvent
	if err := db.Where("type = ?", constants.WorkflowRunHistoryPrunedEvent).Find(&events).Error; err != nil || len(events) != 1 || events[0].StreamID != "flow" {
		t.Fatalf("清理必须在定义事件流通知列表: %+v %v", events, err)
	}
}

func TestRunHistoryProtectsActiveRunsAndPrunesWhenTheyEnd(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	runs := NewWorkflowRunController(db)
	seedRunHistory(t, db, "flow", 31, constants.WorkflowRunStatusWaiting)
	// 终态提交前仍超限，授权等待期间不可删除。
	if err := db.Model(&models.WorkflowRun{}).Where("id = ?", "flow-001").UpdateColumn("status", constants.WorkflowRunStatusWaiting).Error; err != nil {
		t.Fatal(err)
	}
	if err := runs.DeleteRunHistory(ctx, "flow", "flow-001", 100); err != nil {
		t.Fatal(err)
	}
	active, err := runs.GetRun(ctx, "flow-001")
	if err != nil || active.HistoryDeletedAt != nil {
		t.Fatalf("等待中的运行仍是活跃事实: %+v %v", active, err)
	}
	run, err := runs.GetRun(ctx, "flow-000")
	if err != nil {
		t.Fatal(err)
	}
	ended := int64(101)
	run.Status, run.CompletedAt, run.Result = constants.WorkflowRunStatusSucceeded, &ended, map[string]any{"report": "完成"}
	results := NewWorkResultController(db)
	if err := results.EnableTracking(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := NewTransactionController(dbhelper.NewGormTransactionManager(db)).Transaction(ctx, func(ctx context.Context) error {
		return runs.CommitRunState(ctx, WorkflowRunState{Run: run}, 1, 101)
	}); err != nil {
		t.Fatal(err)
	}
	visible, err := runs.ListRuns(ctx, "flow", 0, "", 100)
	if err != nil || len(visible) != 30 {
		t.Fatalf("活跃记录应保留，结束后自动继续清理: %d %v", len(visible), err)
	}
	completed, err := runs.GetRun(ctx, run.ID)
	if err != nil || completed.HistoryDeletedAt == nil || completed.Result == nil {
		t.Fatalf("隐藏入口不得删除真实完成结果: %+v %v", completed, err)
	}
	if _, found, err := results.Get(ctx, constants.WorkResultOwnerKindWorkflowRun, run.ID); err != nil || !found {
		t.Fatalf("清理不得跳过成果回执提交: %v %v", found, err)
	}
}

func TestRunHistoryPrunesBacklogButKeepsChildFacts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	runs := NewWorkflowRunController(db)
	seedRunHistory(t, db, "flow", 100, constants.WorkflowRunStatusFailed)
	if err := db.Model(&models.WorkflowRun{}).Where("id = ?", "flow-000").UpdateColumn("parent_run_id", "flow-001").Error; err != nil {
		t.Fatal(err)
	}
	if err := NewTransactionController(dbhelper.NewGormTransactionManager(db)).Transaction(ctx, func(ctx context.Context) error {
		return runs.CreateRun(ctx, models.WorkflowRun{ID: "new", DefinitionID: "flow", Status: constants.WorkflowRunStatusQueued, CreatedAt: 200}, models.WorkflowFiber{ID: "new-fiber", RunID: "new"})
	}); err != nil {
		t.Fatal(err)
	}
	visible, err := runs.ListRuns(ctx, "flow", 0, "", 100)
	if err != nil || len(visible) != 26 {
		t.Fatalf("积压历史按同一减半规则收敛: %d %v", len(visible), err)
	}
	children, err := runs.ListChildren(ctx, "flow-001")
	if err != nil || len(children) != 1 || children[0].ID != "flow-000" || children[0].HistoryDeletedAt == nil {
		t.Fatalf("历史清理不能切断父子运行证据: %+v %v", children, err)
	}
}

func TestRunHistoryPruningRollsBackWithAdmission(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	runs := NewWorkflowRunController(db)
	seedRunHistory(t, db, "flow", 30, constants.WorkflowRunStatusCancelled)
	err := NewTransactionController(dbhelper.NewGormTransactionManager(db)).Transaction(ctx, func(ctx context.Context) error {
		if err := runs.CreateRun(ctx, models.WorkflowRun{ID: "new", DefinitionID: "flow", Status: constants.WorkflowRunStatusQueued}, models.WorkflowFiber{ID: "new-fiber", RunID: "new"}); err != nil {
			return err
		}
		return status_error.WorkflowStateConflict
	})
	if err != status_error.WorkflowStateConflict {
		t.Fatal(err)
	}
	values, err := runs.ListRuns(ctx, "flow", 0, "", 100)
	if err != nil || len(values) != 30 {
		t.Fatalf("接纳回滚必须同时回滚历史清理: %d %v", len(values), err)
	}
	var count int64
	if err := db.Model(&models.WorkflowEvent{}).Where("type = ?", constants.WorkflowRunHistoryPrunedEvent).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("不得留下未提交的清理事件: %d %v", count, err)
	}
}

func TestRemovedRunHistoryDoesNotAppearInWorkBoardOrUnreadCounts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if err := db.Create(&models.WorkflowDefinition{ID: "flow", Title: "历史清理"}).Error; err != nil {
		t.Fatal(err)
	}
	seedRunHistory(t, db, "flow", 2, constants.WorkflowRunStatusSucceeded)
	results := NewWorkResultController(db)
	if err := results.EnableTracking(ctx, 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"flow-000", "flow-001"} {
		if err := results.Publish(ctx, models.WorkResultReceipt{OwnerKind: constants.WorkResultOwnerKindWorkflowRun, OwnerID: id, LatestVersion: 1, AvailableAt: 200, Reference: models.WorkResultReference{WorkflowRunID: id}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewWorkflowRunController(db).DeleteRunHistory(ctx, "flow", "flow-000", 300); err != nil {
		t.Fatal(err)
	}
	board := NewWorkBoardController(db)
	for _, query := range []WorkBoardQuery{{}, {Source: "workflow"}, {ExecutorID: "flow"}, {Query: "历史"}} {
		count, err := board.ResultCount(ctx, query)
		if err != nil || count != 1 {
			t.Fatalf("所有未读计数路径应排除删除的历史: %+v %d %v", query, count, err)
		}
	}
	for _, method := range []func(context.Context, WorkBoardQuery) ([]models.WorkBoardEntry, error){board.List, board.Results} {
		values, err := method(ctx, WorkBoardQuery{Limit: 50})
		if err != nil || len(values) != 1 || values[0].OwnerID != "flow-001" {
			t.Fatalf("工作看板不得继续出现已移除的记录: %+v %v", values, err)
		}
	}
	if _, found, err := results.Get(ctx, constants.WorkResultOwnerKindWorkflowRun, "flow-000"); err != nil || !found {
		t.Fatalf("成果凭据必须保留: %v %v", found, err)
	}
}

// 关联取消必须扫描完整活跃集合，同时保持索引游标读取与每页一次 SQL。
func TestConsumerRunQueryUsesBoundedIndexTraversal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	values := make([]models.WorkflowRun, 0, 800)
	for i := 0; i < 800; i++ {
		status := constants.WorkflowRunStatusQueued
		lease := "selected"
		if i >= 257 {
			lease = "other"
		}
		if i >= 600 {
			status = constants.WorkflowRunStatusSucceeded
			lease = "selected"
		}
		values = append(values, models.WorkflowRun{ID: fmt.Sprintf("run-%04d", i), DefinitionID: "definition", ConsumerLeaseID: lease, Status: status})
	}
	if err := db.CreateInBatches(values, 50).Error; err != nil {
		t.Fatal(err)
	}
	observer := &sqlPerformanceLog{}
	if err := db.Callback().Query().After("gorm:query").Register("test:consumer_query_count", func(query *gorm.DB) { observer.Queries++; observer.MaxRows = max(observer.MaxRows, query.RowsAffected) }); err != nil {
		t.Fatal(err)
	}
	runs := NewWorkflowRunController(db)
	count := 0
	after := ""
	for {
		ids, err := runs.ListActiveConsumerRuns(ctx, "selected", after, 128)
		if err != nil {
			t.Fatal(err)
		}
		count += len(ids)
		if len(ids) < 128 {
			break
		}
		after = ids[len(ids)-1]
	}
	if count != 257 || observer.Queries != 3 || observer.MaxRows > 128 {
		t.Fatalf("分页遗漏或查询无界: count=%d queries=%d maxRows=%d", count, observer.Queries, observer.MaxRows)
	}
	var plans []struct { // Detail 是 SQLite 的真实查询计划。
		Detail string `gorm:"column:detail"`
	}
	if err := db.Raw("EXPLAIN QUERY PLAN SELECT id FROM workflow_runs WHERE consumer_lease_id = ? AND id > ? AND status NOT IN (?,?,?) ORDER BY id ASC LIMIT 128", "selected", "", constants.WorkflowRunStatusSucceeded, constants.WorkflowRunStatusFailed, constants.WorkflowRunStatusCancelled).Scan(&plans).Error; err != nil {
		t.Fatal(err)
	}
	indexed := false
	for _, plan := range plans {
		t.Log(plan.Detail)
		if strings.Contains(plan.Detail, "idx_workflow_consumer_runs") {
			indexed = true
		}
	}
	if !indexed {
		t.Fatal("关联查询未使用消费者索引")
	}
	t.Logf("257 active / 800 total: SQL=%d, maxRows=%d", observer.Queries, observer.MaxRows)
}

// 初始化搬移必须有界访问旧行与消费者，已有新列归属不得被旧 Context 覆盖。
func TestLegacyConsumerAssociationMigrationUsesBoundedIndexedReads(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	for i := 0; i < 300; i++ {
		id := fmt.Sprintf("run-%03d", i)
		owner := fmt.Sprintf("owner-%03d", i)
		if err := db.Create(&models.WorkflowConsumerLease{ID: owner, ChatID: "chat", DefinitionIDs: []string{"definition"}}).Error; err != nil {
			t.Fatal(err)
		}
		run := models.WorkflowRun{ID: id, DefinitionID: "definition", RevisionID: "revision", Status: constants.WorkflowRunStatusQueued, UpdatedAt: 7, Context: map[string]any{"consumerLeaseId": owner, "runId": id, "workflowId": "definition", "revisionId": "revision"}}
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	existing := models.WorkflowRun{ID: "already-migrated", ConsumerLeaseID: "current-owner", DefinitionID: "definition", RevisionID: "revision", Status: constants.WorkflowRunStatusQueued, Context: map[string]any{"consumerLeaseId": "owner-000"}}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	queries, maxRows, updates := 0, int64(0), 0
	if err := db.Callback().Query().After("gorm:query").Register("test:legacy_read_budget", func(query *gorm.DB) { queries++; maxRows = max(maxRows, query.RowsAffected) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("test:legacy_update_budget", func(query *gorm.DB) { updates++ }); err != nil {
		t.Fatal(err)
	}
	if err := RestoreWorkflowConsumerAssociations(ctx, db); err != nil {
		t.Fatal(err)
	}
	if queries != 6 || maxRows > 128 || updates != 300 {
		t.Fatalf("unbounded migration: reads=%d maxRows=%d writes=%d", queries, maxRows, updates)
	}
	t.Logf("300 old runs/300 consumers: reads=%d maxRows=%d writes=%d", queries, maxRows, updates)
	queries, updates = 0, 0
	if err := RestoreWorkflowConsumerAssociations(ctx, db); err != nil {
		t.Fatal(err)
	}
	if queries != 1 || updates != 0 {
		t.Fatalf("repeat initialization rewrote data: reads=%d writes=%d", queries, updates)
	}
	var kept models.WorkflowRun
	if err := db.First(&kept, "id = ?", existing.ID).Error; err != nil || kept.ConsumerLeaseID != "current-owner" {
		t.Fatalf("old context overwrote authoritative column: %+v %v", kept, err)
	}
	var migrated models.WorkflowRun
	if err := db.First(&migrated, "id = ?", "run-299").Error; err != nil || migrated.ConsumerLeaseID != "owner-299" || migrated.UpdatedAt != 7 {
		t.Fatalf("migration lost tail or changed business time: %+v %v", migrated, err)
	}
	for _, statement := range []struct {
		// SQL 是生产查询对应的实际 SQLite 查询计划语句。
		SQL string
		// Args 是实际查询条件。
		Args []any
		// Index 是必须命中的索引名。
		Index string
	}{
		{"EXPLAIN QUERY PLAN SELECT id, definition_id, revision_id, context FROM workflow_runs WHERE consumer_lease_id = ? AND id > ? ORDER BY id ASC LIMIT 128", []any{"", ""}, "idx_workflow_consumer_runs"},
		{"EXPLAIN QUERY PLAN SELECT id,chat_id,definition_ids FROM workflow_consumer_leases WHERE id IN (?,?)", []any{"owner-000", "owner-001"}, "sqlite_autoindex_workflow_consumer_leases_1"},
		{"EXPLAIN QUERY PLAN UPDATE workflow_runs SET consumer_lease_id=?,context=? WHERE id=? AND consumer_lease_id=? AND context=?", []any{"owner-000", "{}", "run-000", "", "{}"}, "sqlite_autoindex_workflow_runs_1"},
	} {
		var plans []struct {
			// Detail 是 SQLite 实际选用的索引与读取方式。
			Detail string
		}
		if err := db.Raw(statement.SQL, statement.Args...).Scan(&plans).Error; err != nil {
			t.Fatal(err)
		}
		indexed := false
		for _, plan := range plans {
			t.Log(plan.Detail)
			if strings.Contains(plan.Detail, statement.Index) {
				indexed = true
			}
			if strings.Contains(plan.Detail, "TEMP B-TREE") {
				t.Fatal("migration introduced temporary sort")
			}
		}
		if !indexed {
			t.Fatalf("missing bounded lookup index %s", statement.Index)
		}
	}
}

// 数据库中途写入失败必须回滚当前批次，修复故障后原初始化可恢复，已提交批次保持幂等。
func TestLegacyConsumerMigrationRollsBackFailedBatchAndRecovers(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if err := db.Create(&models.WorkflowConsumerLease{ID: "owner", ChatID: "chat", DefinitionIDs: []string{"definition"}}).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "second"} {
		if err := db.Create(&models.WorkflowRun{ID: id, DefinitionID: "definition", RevisionID: "revision", Status: constants.WorkflowRunStatusQueued, Context: map[string]any{"runId": id, "workflowId": "definition", "revisionId": "revision", "consumerLeaseId": "owner"}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`CREATE TRIGGER fail_legacy_association BEFORE UPDATE ON workflow_runs WHEN NEW.id = 'second' BEGIN SELECT RAISE(ABORT, 'fixture migration write failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := RestoreWorkflowConsumerAssociations(ctx, db); !errors.Is(err, status_error.DatabaseError) {
		t.Fatalf("write failure hidden: %v", err)
	}
	var first models.WorkflowRun
	if err := db.First(&first, "id = ?", "first").Error; err != nil || first.ConsumerLeaseID != "" || first.Context["consumerLeaseId"] != "owner" {
		t.Fatalf("failed batch partly committed: %+v %v", first, err)
	}
	if err := db.Exec(`DROP TRIGGER fail_legacy_association`).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := RestoreWorkflowConsumerAssociations(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"first", "second"} {
		var run models.WorkflowRun
		if err := db.First(&run, "id = ?", id).Error; err != nil || run.ConsumerLeaseID != "owner" || run.Context["consumerLeaseId"] != nil {
			t.Fatalf("initialization did not recover: %+v %v", run, err)
		}
	}
}
