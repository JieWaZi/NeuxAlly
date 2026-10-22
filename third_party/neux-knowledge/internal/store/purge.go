package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"gorm.io/gorm"
)

type purgeSnapshot struct {
	Library       *types.Library             // 冻结的库配置及权限。
	Head          types.SourceHead           // 稳定来源的当前版本。
	Sources       []types.SourceVersion      // 本来源全部原件版本。
	Pages         []types.WikiPage           // 当前库页面，包含软删除记录以清理内部副本。
	Revisions     []types.WikiPageRevision   // 当前库全部历史快照。
	Jobs          []types.Job                // 当前库任务及终态诊断。
	SourceWork    []types.OrganizeSourceWork // 当前库来源重排诊断，可能保存原文内部副本。
	FinalizeOps   []types.WikiFinalizeOp     // 当前库尚存的收尾文档摘要和页面增量。
	Stages        []types.Stage              // 当前库所有尝试的内部输入。
	Issues        []types.WikiPageIssue      // 可能保存原文的持久问题记录。
	Imports       []types.ImportBlobIntent   // 已登记但业务尚未提交的本来源导入。
	SharedImports []types.ImportBlobIntent   // 其他来源仍在导入的共享对象，不能删除。
	Shared        []types.SourceVersion      // 其他来源仍引用的原件对象。
	RawOwnership  string                     // 当前原件根校验结果，固定为verified。
}

func purgeBoundaries() []string {
	return []string{
		"人工与多来源当前知识正文保留并标记待核查；来源无法再读取",
		"受影响页面历史正文和同库终态任务内部材料将清空",
		"其他来源共享的原件保留；原件根身份异常会拒绝清理",
		"不承诺SQLite空闲页、文件系统介质或备份物理擦除",
		"已导出文件、宿主聊天和未带来源标识的用户资料不在本次清理范围",
	}
}

func validBlobKey(key string) bool {
	b, err := hex.DecodeString(key)
	return err == nil && len(b) == 32 && strings.ToLower(key) == key
}
func validImportTemp(key string) bool {
	return strings.HasPrefix(key, ".import-") && strings.HasSuffix(key, ".tmp") && len(key) == 76 && validBlobKey(key[8:72])
}
func sortedUnique(items []string) []string {
	sort.Strings(items)
	out := []string{}
	for _, item := range items {
		if len(out) == 0 || out[len(out)-1] != item {
			out = append(out, item)
		}
	}
	return out
}

func eachPurgeBatch(ids []string, fn func([]string) error) error {
	for from := 0; from < len(ids); from += 500 {
		if err := fn(ids[from:min(from+500, len(ids))]); err != nil {
			return err
		}
	}
	return nil
}

func purgeReferences(evidence types.PageEvidence, sourceID string, versions map[string]bool) bool {
	for _, ref := range evidence.Sources {
		if ref.SourceID == sourceID || versions[ref.SourceVersionID] {
			return true
		}
	}
	return false
}

func removePurgedEvidence(evidence types.PageEvidence, sourceID string, versions map[string]bool) types.PageEvidence {
	remaining := types.PageEvidence{Sources: []types.SourceRef{}, Code: evidence.Code}
	for _, ref := range evidence.Sources {
		if ref.SourceID != sourceID && !versions[ref.SourceVersionID] {
			remaining.Sources = append(remaining.Sources, ref)
		}
	}
	return remaining
}

func sourcePurgeSnapshot(tx *gorm.DB, in types.PurgePreviewInput) (*purgeSnapshot, *types.SourcePurgePreview, error) {
	l, err := getLibrary(tx, in.LibraryID, true)
	if err != nil {
		return nil, nil, err
	}
	snap := &purgeSnapshot{Library: l, RawOwnership: "verified"}
	if err = tx.Where("id = ? AND library_id = ?", in.SourceID, in.LibraryID).Take(&snap.Head).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Where("source_id = ? AND library_id = ?", in.SourceID, in.LibraryID).Order("id ASC").Find(&snap.Sources).Error; err != nil {
		return nil, nil, err
	}
	if len(snap.Sources) == 0 {
		return nil, nil, fail(types.ErrNotFound, "source.purge.preview", "source versions are missing")
	}
	if err = tx.Where("source_id = ? AND library_id = ? AND purge_id = ?", in.SourceID, in.LibraryID, "").Order("request_id ASC").Find(&snap.Imports).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Unscoped().Where("knowledge_base_id = ?", in.LibraryID).Order("id ASC").Find(&snap.Pages).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Where("knowledge_base_id = ?", in.LibraryID).Order("id ASC").Find(&snap.Revisions).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Where("library_id = ?", in.LibraryID).Order("id ASC").Find(&snap.Jobs).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Where("library_id = ?", in.LibraryID).Order("id ASC").Find(&snap.SourceWork).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Where("library_id = ?", in.LibraryID).Order("id ASC").Find(&snap.FinalizeOps).Error; err != nil {
		return nil, nil, err
	}
	if err = tx.Unscoped().Where("knowledge_base_id = ?", in.LibraryID).Order("id ASC").Find(&snap.Issues).Error; err != nil {
		return nil, nil, err
	}
	jobIDs := []string{}
	for _, job := range snap.Jobs {
		jobIDs = append(jobIDs, job.ID)
	}
	if err = eachPurgeBatch(jobIDs, func(ids []string) error {
		var rows []types.Stage
		if e := tx.Where("job_id IN ?", ids).Find(&rows).Error; e != nil {
			return e
		}
		snap.Stages = append(snap.Stages, rows...)
		return nil
	}); err != nil {
		return nil, nil, err
	}
	sort.Slice(snap.Stages, func(i, j int) bool { return snap.Stages[i].ID < snap.Stages[j].ID })
	p := &types.SourcePurgePreview{LibraryID: l.ID, SourceID: in.SourceID, LibraryVersion: l.Version, StructureVersion: l.StructureVersion, PolicyVersion: l.PolicyVersion, Versions: []types.PurgeVersion{}, Pages: []types.PurgePageImpact{}, JobCount: len(snap.Jobs), StageCount: len(snap.Stages), IssueCount: len(snap.Issues), RawOwnership: snap.RawOwnership, BlockingJobIDs: []string{}, Boundaries: purgeBoundaries()}
	versions := map[string]bool{}
	keys := []string{}
	for _, v := range snap.Sources {
		if !validBlobKey(v.RawKey) || v.RawKey != v.SHA256 {
			return nil, nil, fail(types.ErrStorage, "source.purge.preview", "source object identity is invalid")
		}
		versions[v.ID] = true
		keys = append(keys, v.RawKey)
		p.Versions = append(p.Versions, types.PurgeVersion{VersionID: v.ID, Version: v.Version, RowVersion: v.RowVersion})
	}
	p.PendingImportCount = len(snap.Imports)
	for _, pending := range snap.Imports {
		if !validBlobKey(pending.RawKey) || !validImportTemp(pending.TempKey) {
			return nil, nil, fail(types.ErrStorage, "source.purge.preview", "pending import object identity is invalid")
		}
		keys = append(keys, pending.RawKey)
	}
	keys = sortedUnique(keys)
	p.BlobCount = len(keys) + len(snap.Imports)
	if err = eachPurgeBatch(keys, func(batch []string) error {
		var rows []types.SourceVersion
		if e := tx.Where("raw_key IN ? AND source_id <> ?", batch, in.SourceID).Order("id ASC").Find(&rows).Error; e != nil {
			return e
		}
		snap.Shared = append(snap.Shared, rows...)
		return nil
	}); err != nil {
		return nil, nil, err
	}
	sort.Slice(snap.Shared, func(i, j int) bool { return snap.Shared[i].ID < snap.Shared[j].ID })
	if err = eachPurgeBatch(keys, func(batch []string) error {
		var rows []types.ImportBlobIntent
		if e := tx.Where("raw_key IN ? AND source_id <> ? AND purge_id = ?", batch, in.SourceID, "").Order("request_id ASC").Find(&rows).Error; e != nil {
			return e
		}
		snap.SharedImports = append(snap.SharedImports, rows...)
		return nil
	}); err != nil {
		return nil, nil, err
	}
	sort.Slice(snap.SharedImports, func(i, j int) bool { return snap.SharedImports[i].RequestID < snap.SharedImports[j].RequestID })
	sharedKeys := map[string]bool{}
	for _, v := range snap.Shared {
		sharedKeys[v.RawKey] = true
	}
	for _, pending := range snap.SharedImports {
		sharedKeys[pending.RawKey] = true
	}
	p.SharedBlobCount = len(sharedKeys)
	affected := map[string]bool{}
	for _, page := range snap.Pages {
		if purgeReferences(page.Evidence, in.SourceID, versions) {
			affected[page.ID] = true
		}
	}
	for _, rev := range snap.Revisions {
		if purgeReferences(rev.Evidence, in.SourceID, versions) {
			affected[rev.PageID] = true
		}
	}
	for _, rev := range snap.Revisions {
		if affected[rev.PageID] {
			p.RevisionCount++
		}
	}
	for _, page := range snap.Pages {
		if affected[page.ID] {
			remaining := removePurgedEvidence(page.Evidence, in.SourceID, versions)
			preserve := page.LastEditSource == "user" || len(page.ProtectedContent) > 0 || len(remaining.Sources) > 0 || len(remaining.Code) > 0
			p.Pages = append(p.Pages, types.PurgePageImpact{PageID: page.ID, Version: page.Version, PreserveBody: preserve})
		}
	}
	for _, job := range snap.Jobs {
		if job.State == "queued" || job.State == "processing" || job.State == "cancel_requested" {
			p.BlockingJobIDs = append(p.BlockingJobIDs, job.ID)
		}
	}
	for _, stage := range snap.Stages {
		if stage.State == "running" {
			p.BlockingJobIDs = append(p.BlockingJobIDs, stage.JobID)
		}
	}
	p.BlockingJobIDs = sortedUnique(p.BlockingJobIDs)
	serialized, err := json.Marshal(snap)
	if err != nil {
		return nil, nil, fail(types.ErrStorage, "source.purge.preview", "persisted purge snapshot cannot be committed")
	}
	p.Digest = hash(serialized)
	return snap, p, nil
}

// PreviewSourcePurge 读取真实影响与阻塞任务，输出中不含原件、文件名或阅读材料。
func (s *Store) PreviewSourcePurge(ctx context.Context, in types.PurgePreviewInput) (result *types.SourcePurgePreview, err error) {
	release, err := s.lockBlobs(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	err = s.blobBinding(ctx, false)
	if err != nil {
		return nil, err
	}
	err = s.transaction(ctx, func(tx *gorm.DB) error { _, p, e := sourcePurgeSnapshot(tx, in); result = p; return e })
	return
}

func purgeMetadata(prior types.JSON, record *types.SourcePurgeRecord) string {
	m := types.ParseSourcePurgeReview(prior)
	if m == nil {
		m = &types.SourcePurgeReview{}
	}
	m.NeedsReview = true
	m.UnavailableSourceIDs = append(m.UnavailableSourceIDs, record.SourceID)
	m.PurgeIDs = append(m.PurgeIDs, record.ID)
	for _, v := range record.Versions {
		m.UnavailableVersionIDs = append(m.UnavailableVersionIDs, v.VersionID)
	}
	m.UnavailableSourceIDs = sortedUnique(m.UnavailableSourceIDs)
	m.UnavailableVersionIDs = sortedUnique(m.UnavailableVersionIDs)
	m.PurgeIDs = sortedUnique(m.PurgeIDs)
	b, _ := json.Marshal(struct {
		SourcePurge *types.SourcePurgeReview `json:"sourcePurge"` // 只保存不可用身份与待核查事实。
	}{m})
	return string(b)
}

func evidenceJSON(e types.PageEvidence) string { b, _ := json.Marshal(e); return string(b) }
func evidenceIDsJSON(e types.PageEvidence) string {
	ids := []string{}
	for _, ref := range e.Sources {
		ids = append(ids, ref.SourceVersionID)
	}
	b, _ := json.Marshal(sortedUnique(ids))
	return string(b)
}

func scrubSourcePurge(tx *gorm.DB, snap *purgeSnapshot, preview *types.SourcePurgePreview, record *types.SourcePurgeRecord) error {
	versions := map[string]bool{}
	for _, v := range record.Versions {
		versions[v.VersionID] = true
	}
	impacts := map[string]types.PurgePageImpact{}
	for _, impact := range preview.Pages {
		impacts[impact.PageID] = impact
		record.PageIDs = append(record.PageIDs, impact.PageID)
	}
	now := time.Now()
	for _, page := range snap.Pages {
		impact, affected := impacts[page.ID]
		if !affected {
			continue
		}
		remaining := removePurgedEvidence(page.Evidence, record.SourceID, versions)
		values := map[string]any{"evidence": evidenceJSON(remaining), "source_refs": evidenceIDsJSON(remaining), "chunk_refs": "[]", "summary": "", "page_metadata": purgeMetadata(page.PageMetadata, record), "version": page.Version + 1, "updated_at": now}
		if !impact.PreserveBody {
			values["content"] = ""
			values["protected_content"] = "[]"
			values["aliases"] = "[]"
			values["status"] = "archived"
			values["out_links"] = "[]"
		}
		updated := tx.Unscoped().Model(&types.WikiPage{}).Where("id = ? AND knowledge_base_id = ? AND version = ?", page.ID, record.LibraryID, page.Version).Updates(values)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fail(types.ErrVersionConflict, "source.purge.confirm", "referencing page changed")
		}
	}
	for _, rev := range snap.Revisions {
		if _, affected := impacts[rev.PageID]; !affected && !purgeReferences(rev.Evidence, record.SourceID, versions) {
			continue
		}
		remaining := removePurgedEvidence(rev.Evidence, record.SourceID, versions)
		if err := tx.Model(&types.WikiPageRevision{}).Where("id = ?", rev.ID).Updates(map[string]any{"content": "", "summary": "", "protected_content": "[]", "aliases": "[]", "chunk_refs": "[]", "page_metadata": purgeMetadata(rev.PageMetadata, record), "evidence": evidenceJSON(remaining), "source_refs": evidenceIDsJSON(remaining)}).Error; err != nil {
			return err
		}
	}
	for _, job := range snap.Jobs {
		if err := tx.Model(&types.Job{}).Where("id = ?", job.ID).Updates(map[string]any{"diagnostic": "", "cancel_requested": true, "version": job.Version + 1, "updated_at": now.UnixMilli()}).Error; err != nil {
			return err
		}
	}
	// 单来源清理同时移除同库可能复述原件的内部重排和收尾摘要。
	for _, model := range []any{&types.OrganizeSourceWork{}, &types.WikiFinalizeOp{}} {
		if err := tx.Where("library_id = ?", record.LibraryID).Delete(model).Error; err != nil {
			return err
		}
	}
	for _, pending := range snap.Imports {
		if err := tx.Model(&types.ImportBlobIntent{}).Where("request_id = ?", pending.RequestID).Updates(map[string]any{"raw_key": "", "temp_key": "", "purge_id": record.ID}).Error; err != nil {
			return err
		}
	}
	for _, stage := range snap.Stages {
		if err := tx.Model(&types.Stage{}).Where("id = ?", stage.ID).Updates(map[string]any{"input": []byte("{}"), "result_text": "", "usage": nil, "diagnostics": nil, "diagnostic": "", "outcome": "cancelled", "completeness": "unknown", "stop_reason": "source_purged", "state": "finished", "version": stage.Version + 1}).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("knowledge_base_id = ?", record.LibraryID).Delete(&types.WikiPageIssue{}).Error; err != nil {
		return err
	}
	for _, v := range record.Versions {
		if err := tx.Where("library_id = ? AND source_version_id = ?", record.LibraryID, v.VersionID).Delete(&types.EmbeddingChunkIndex{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&types.SourceTombstone{VersionID: v.VersionID, SourceID: record.SourceID, LibraryID: record.LibraryID, PurgeID: record.ID}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("source_id = ? AND library_id = ?", record.SourceID, record.LibraryID).Delete(&types.SourceVersion{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id = ? AND library_id = ?", record.SourceID, record.LibraryID).Delete(&types.SourceHead{}).Error; err != nil {
		return err
	}
	return refreshBacklinks(tx, record.LibraryID)
}

// ConfirmSourcePurge 在同一事务重算影响、撤销来源、清理内部副本并保存原件清理清单。
func (s *Store) ConfirmSourcePurge(ctx context.Context, in types.ConfirmSourcePurgeInput) (*types.Receipt, error) {
	if !in.Confirmed || !validBlobKey(in.PreviewDigest) {
		return nil, fail(types.ErrInvalid, "source.purge.confirm", "explicit confirmation and preview digest are required")
	}
	release, err := s.lockBlobs(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	err = s.blobBinding(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, in.RequestID, "source.purge.confirm", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		snap, preview, err := sourcePurgeSnapshot(tx, types.PurgePreviewInput{LibraryID: in.LibraryID, SourceID: in.SourceID})
		if err != nil {
			return nil, err
		}
		if preview.Digest != in.PreviewDigest {
			return nil, fail(types.ErrVersionConflict, "source.purge.confirm", "purge impact or policy changed")
		}
		if len(preview.BlockingJobIDs) > 0 {
			return nil, fail(types.ErrVersionConflict, "source.purge.confirm", "active jobs must finish before source purge")
		}
		now := time.Now().UnixMilli()
		record := &types.SourcePurgeRecord{ID: newID(), LibraryID: in.LibraryID, SourceID: in.SourceID, Version: 1, State: "pending", PreviewDigest: preview.Digest, Versions: preview.Versions, Objects: []types.PurgeBlobCleanup{}, PageIDs: []string{}, CreatedAt: now, UpdatedAt: now}
		keys := []string{}
		for _, source := range snap.Sources {
			keys = append(keys, source.RawKey)
		}
		for _, pending := range snap.Imports {
			keys = append(keys, pending.RawKey)
		}
		for _, key := range sortedUnique(keys) {
			record.Objects = append(record.Objects, types.PurgeBlobCleanup{Key: key, State: "pending", Kind: "raw"})
		}
		for _, pending := range snap.Imports {
			record.Objects = append(record.Objects, types.PurgeBlobCleanup{Key: pending.TempKey, State: "pending", Kind: "temporary"})
		}
		if err = tx.Create(record).Error; err != nil {
			return nil, err
		}
		if err = scrubSourcePurge(tx, snap, preview, record); err != nil {
			return nil, err
		}
		pageIDs, _ := json.Marshal(record.PageIDs)
		if err = tx.Model(record).Update("page_ids", string(pageIDs)).Error; err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: record.ID, Version: record.Version, State: "accepted", PageIDs: record.PageIDs}, nil
	})
}

func purgeStatus(record *types.SourcePurgeRecord) *types.SourcePurgeStatus {
	p := &types.SourcePurgeStatus{ID: record.ID, LibraryID: record.LibraryID, SourceID: record.SourceID, Version: record.Version, State: record.State, Versions: record.Versions, PageIDs: record.PageIDs, Boundaries: purgeBoundaries()}
	for _, object := range record.Objects {
		switch object.State {
		case "deleted":
			p.DeletedBlobCount++
		case "retained_shared":
			p.RetainedSharedBlobCount++
		default:
			p.PendingBlobCount++
		}
	}
	return p
}

func (s *Store) GetSourcePurge(ctx context.Context, libraryID, purgeID string) (result *types.SourcePurgeStatus, err error) {
	err = s.transaction(ctx, func(tx *gorm.DB) error {
		var record types.SourcePurgeRecord
		if e := tx.Where("id = ? AND library_id = ?", purgeID, libraryID).Take(&record).Error; e != nil {
			return e
		}
		result = purgeStatus(&record)
		return nil
	})
	return
}

// RecoverSourcePurge 先获得跨进程原件锁，再用写事务持有授权与引用检查至删除结束。
// 文件删除发生在既有持久清单之后；事务回滚或进程退出时可按原清单继续，文件不存在视为已处理。
func (s *Store) RecoverSourcePurge(ctx context.Context, in types.RecoverSourcePurgeInput) (*types.Receipt, error) {
	if in.ExpectedPolicyVersion < 1 {
		return nil, fail(types.ErrInvalid, "source.purge.recover", "current policy version is required")
	}
	release, err := s.lockBlobs(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	err = s.blobBinding(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, in.RequestID, "source.purge.recover", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		l, err := getLibrary(tx, in.LibraryID, false)
		if err != nil {
			return nil, err
		}
		if l.PolicyVersion != in.ExpectedPolicyVersion {
			return nil, fail(types.ErrScopeViolation, "source.purge.recover", "policy version changed")
		}
		var record types.SourcePurgeRecord
		if err = tx.Where("id = ? AND library_id = ?", in.PurgeID, in.LibraryID).Take(&record).Error; err != nil {
			return nil, err
		}
		// 在触碰文件前取得 SQLite 写锁；授权撤销、其他写入与引用变化不能跨过检查。
		if err = tx.Model(&types.SourcePurgeRecord{}).Where("id = ? AND version = ?", record.ID, record.Version).UpdateColumn("version", record.Version).Error; err != nil {
			return nil, err
		}
		state := "completed"
		for i := range record.Objects {
			object := &record.Objects[i]
			if (object.Kind == "temporary" && !validImportTemp(object.Key)) || (object.Kind != "temporary" && !validBlobKey(object.Key)) {
				return nil, fail(types.ErrStorage, "source.purge.recover", "cleanup object key is invalid")
			}
			if object.State == "deleted" {
				continue
			}
			var shared int64
			if object.Kind != "temporary" {
				if err = tx.Model(&types.SourceVersion{}).Where("raw_key = ?", object.Key).Count(&shared).Error; err != nil {
					return nil, err
				}
				var pending int64
				if err = tx.Model(&types.ImportBlobIntent{}).Where("raw_key = ? AND purge_id = ?", object.Key, "").Count(&pending).Error; err != nil {
					return nil, err
				}
				shared += pending
			}
			if shared > 0 {
				object.State = "retained_shared"
				continue
			}
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			if err = s.raw.Remove(object.Key); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fail(types.ErrStorage, "source.purge.recover", "managed object removal failed; cleanup remains pending")
			}
			object.State = "deleted"
		}
		record.State = state
		record.Version++
		record.UpdatedAt = time.Now().UnixMilli()
		if err = tx.Model(&types.SourcePurgeRecord{}).Where("id = ?", record.ID).Select("state", "version", "objects", "updated_at").Updates(&record).Error; err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: record.ID, Version: record.Version, State: record.State, PageIDs: record.PageIDs}, nil
	})
}
