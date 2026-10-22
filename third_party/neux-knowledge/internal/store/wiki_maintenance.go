// Copyright (C) 2026 Tencent. All rights reserved.
// 迁移自固定WeKnora维护工具与测试；完整MIT许可见upstream/WeKnora.LICENSE。
package store

import (
	"context"
	"slices"
	"strings"
	"time"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"github.com/JieWaZi/neux-knowledge/internal/wiki"
	"gorm.io/gorm"
)

// pageEdit 只由已验证的维护用例构造，不接受模型直接设置来源或保护策略。
type pageEdit struct {
	agent   bool                // 当前写入是否来自显式智能体维护。
	rename  bool                // 当前操作是否允许保持稳定身份修改slug。
	rewrite func(string) string // 精确变换已有保护片段，未命中片段保持原样。
}

func maintenanceLibrary(tx *gorm.DB, lib string, structure, policy int64) (*types.Library, error) {
	l, err := getLibrary(tx, lib, true)
	if err != nil {
		return nil, err
	}
	if policy < 0 {
		return nil, fail(types.ErrInvalid, "wiki.maintain", "invalid policy version")
	}
	if policy > 0 && policy != l.PolicyVersion {
		return nil, fail(types.ErrScopeViolation, "wiki.maintain", "accepted permission is no longer valid")
	}
	if structure != l.StructureVersion {
		return nil, fail(types.ErrVersionConflict, "wiki.maintain", "structure changed")
	}
	return l, nil
}

func maintenancePage(tx *gorm.DB, l *types.Library, id string, expected int) (*types.WikiPage, error) {
	p, err := getPage(tx, l.ID, id)
	if err != nil {
		return nil, err
	}
	if expected < 1 || p.Version != expected {
		return nil, fail(types.ErrVersionConflict, "wiki.maintain", "page changed")
	}
	return p, nil
}

// WriteAgentPage 复用页面发布、证据及历史CAS，智能体新正文不会成为人工保护。
func (s *Store) WriteAgentPage(ctx context.Context, in types.WritePageInput) (*types.Receipt, error) {
	return s.mutate(ctx, in.RequestID, "page.agent-write", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		l, err := maintenanceLibrary(tx, in.LibraryID, in.StructureVersion, in.ExpectedPolicyVersion)
		if err != nil {
			return nil, err
		}
		p := in.Page
		if in.ExpectedVersion == 0 {
			p.Slug, err = normalizeAgentSlug(p.Slug)
			if err != nil {
				return nil, err
			}
		}
		if in.ExpectedVersion == 0 && (strings.HasPrefix(p.Slug, "summary/") || strings.EqualFold(p.PageType, types.WikiPageTypeSummary)) {
			return nil, fail(types.ErrInvalid, "page.agent-write", "summary pages are system generated")
		}
		structure, err := writePageWithEdit(tx, l, &p, in.ExpectedVersion, true, nil, pageEdit{agent: true})
		if err != nil {
			return nil, err
		}
		if structure {
			if err = bumpStructure(tx, l); err != nil {
				return nil, err
			}
		}
		if err = refreshBacklinks(tx, l.ID); err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: p.ID, Version: int64(p.Version), PageIDs: []string{p.ID}, State: "saved"}, nil
	})
}

// ReplacePageText 迁移上游全部精确匹配语义；明确命中的人工片段同步修订而非解除保护。
func (s *Store) ReplacePageText(ctx context.Context, in types.ReplacePageTextInput) (*types.Receipt, error) {
	return s.mutate(ctx, in.RequestID, "page.replace-text", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		l, err := maintenanceLibrary(tx, in.LibraryID, in.StructureVersion, in.ExpectedPolicyVersion)
		if err != nil {
			return nil, err
		}
		p, err := maintenancePage(tx, l, in.PageID, in.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		if in.OldText == "" || strings.Count(p.Content, in.OldText) == 0 {
			return nil, fail(types.ErrInvalid, "page.replace-text", "old text must match current content")
		}
		rewrite := func(content string) string { return strings.ReplaceAll(content, in.OldText, in.NewText) }
		p.Content = rewrite(p.Content)
		if in.Evidence != nil {
			p.Evidence = *in.Evidence
			p.SourceRefs = nil
		}
		if _, err = writePageWithEdit(tx, l, p, in.ExpectedVersion, true, nil, pageEdit{agent: true, rewrite: rewrite}); err != nil {
			return nil, err
		}
		if err = refreshBacklinks(tx, l.ID); err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: p.ID, Version: int64(p.Version), PageIDs: []string{p.ID}, State: "saved"}, nil
	})
}

// normalizeAgentSlug 迁移上游wiki_write_page的规范化与字符白名单。
func normalizeAgentSlug(raw string) (string, error) {
	s := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), " ", "-")
	if s == "" || strings.Contains(s, "//") || strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return "", fail(types.ErrInvalid, "wiki.maintain", "invalid page slug")
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '/':
		case r >= 0x4E00 && r <= 0x9FFF:
		default:
			return "", fail(types.ErrInvalid, "wiki.maintain", "invalid page slug character")
		}
	}
	return s, nil
}

// RenamePage 迁移上游精确链接重写规则，以同一SQLite事务替代补偿回滚并保留页面ID。
func (s *Store) RenamePage(ctx context.Context, in types.RenamePageInput) (*types.Receipt, error) {
	return s.mutate(ctx, in.RequestID, "page.rename", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		l, err := maintenanceLibrary(tx, in.LibraryID, in.StructureVersion, in.ExpectedPolicyVersion)
		if err != nil {
			return nil, err
		}
		p, err := maintenancePage(tx, l, in.PageID, in.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		newSlug, err := normalizeAgentSlug(in.NewSlug)
		if err != nil {
			return nil, err
		}
		in.NewSlug = newSlug
		if in.NewSlug == p.Slug {
			return nil, fail(types.ErrInvalid, "page.rename", "new slug must be valid and different")
		}
		var count int64
		if err = tx.Model(&types.WikiPage{}).Where("knowledge_base_id = ? AND slug = ?", l.ID, in.NewSlug).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, fail(types.ErrVersionConflict, "page.rename", "new slug already exists")
		}
		oldSlug := p.Slug
		rewrite := func(content string) string {
			return wiki.RewritePageLinks(content, oldSlug, in.NewSlug)
		}
		p.Slug = in.NewSlug
		p.Content = rewrite(p.Content)
		if _, err = writePageWithEdit(tx, l, p, in.ExpectedVersion, true, nil, pageEdit{agent: true, rename: true, rewrite: rewrite}); err != nil {
			return nil, err
		}
		ids, err := rewriteIncomingPages(tx, l, p.ID, oldSlug, in.NewSlug, rewrite)
		if err != nil {
			return nil, err
		}
		if err = tx.Model(&types.WikiPageIssue{}).Where("knowledge_base_id = ? AND slug = ?", l.ID, oldSlug).Updates(map[string]any{"slug": in.NewSlug, "updated_at": time.Now()}).Error; err != nil {
			return nil, err
		}
		if err = bumpStructure(tx, l); err != nil {
			return nil, err
		}
		if err = refreshBacklinks(tx, l.ID); err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: p.ID, Version: int64(p.Version), PageIDs: append([]string{p.ID}, ids...), State: "saved"}, nil
	})
}

// DeletePage 迁移上游普通链接和别名链接清理规则，以归档保留可恢复正文及修订。
func (s *Store) DeletePage(ctx context.Context, in types.DeletePageInput) (*types.Receipt, error) {
	return s.mutate(ctx, in.RequestID, "page.delete", in.LibraryID, in, func(tx *gorm.DB) (*types.Receipt, error) {
		l, err := maintenanceLibrary(tx, in.LibraryID, in.StructureVersion, in.ExpectedPolicyVersion)
		if err != nil {
			return nil, err
		}
		p, err := maintenancePage(tx, l, in.PageID, in.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		if p.Status == "archived" {
			return nil, fail(types.ErrInvalid, "page.delete", "page is already archived")
		}
		rewrite := func(content string) string {
			return wiki.RewritePageLinks(content, p.Slug, "")
		}
		ids, err := rewriteIncomingPages(tx, l, p.ID, p.Slug, "", rewrite)
		if err != nil {
			return nil, err
		}
		p.Status = "archived"
		if _, err = writePageWithEdit(tx, l, p, in.ExpectedVersion, true, nil, pageEdit{agent: true}); err != nil {
			return nil, err
		}
		if err = bumpStructure(tx, l); err != nil {
			return nil, err
		}
		if err = refreshBacklinks(tx, l.ID); err != nil {
			return nil, err
		}
		return &types.Receipt{ResourceID: p.ID, Version: int64(p.Version), PageIDs: append([]string{p.ID}, ids...), State: "saved"}, nil
	})
}

func rewriteIncomingPages(tx *gorm.DB, l *types.Library, exclude, oldSlug, newSlug string, rewrite func(string) string) ([]string, error) {
	var pages []*types.WikiPage
	// 使用同库单表候选查询；精确Markdown匹配仍由上游重写规则判断，避免通配符改变范围。
	if err := tx.Where("knowledge_base_id = ? AND id <> ? AND (CAST(out_links AS TEXT) LIKE ? ESCAPE '\\' OR parent_slug = ?)", l.ID, exclude, "%"+escapeLikePattern(oldSlug)+"%", oldSlug).Order("id ASC").Find(&pages).Error; err != nil {
		return nil, err
	}
	ids := []string{}
	for _, p := range pages {
		updated := p.Content
		if slices.Contains(p.OutLinks, oldSlug) {
			updated = rewrite(p.Content)
		}
		parentChanged := p.ParentSlug == oldSlug
		if updated == p.Content && !parentChanged {
			continue
		}
		p.Content = updated
		if parentChanged {
			p.ParentSlug = newSlug
		}
		if _, err := writePageWithEdit(tx, l, p, p.Version, true, nil, pageEdit{agent: true, rewrite: rewrite}); err != nil {
			return nil, err
		}
		ids = append(ids, p.ID)
	}
	return ids, nil
}

// validateAgentEvidence 只允许复用既有证据或引用同库已持久原文，摘录摘要不能自证为真。
func validateAgentEvidence(tx *gorm.DB, l *types.Library, p, existing *types.WikiPage) error {
	prior := map[string]bool{}
	if existing != nil {
		for _, ref := range existing.Evidence.Sources {
			prior[digest(ref)] = true
		}
		for _, ref := range existing.Evidence.Code {
			prior[digest(ref)] = true
		}
	}
	for _, ref := range p.Evidence.Sources {
		if prior[digest(ref)] || ref.ClaimKind == "document_context" {
			continue
		}
		if ref.Excerpt == "" {
			return fail(types.ErrCandidateInvalid, "page.agent-write", "quoted evidence must contain actual text")
		}
		var chunks []types.EmbeddingChunkIndex
		if err := tx.Select("content", "locator").Where("library_id = ? AND source_version_id = ? AND content LIKE ? ESCAPE '\\'", l.ID, ref.SourceVersionID, "%"+escapeLikePattern(ref.Excerpt)+"%").Find(&chunks).Error; err != nil {
			return err
		}
		verified := false
		for _, chunk := range chunks {
			if strings.Contains(chunk.Content, ref.Excerpt) && chunk.Locator == ref.Locator {
				verified = true
				break
			}
		}
		if !verified {
			return fail(types.ErrCandidateInvalid, "page.agent-write", "quotation and locator must match persisted source chunks")
		}
	}
	for _, ref := range p.Evidence.Code {
		if !prior[digest(ref)] {
			return fail(types.ErrCandidateInvalid, "page.agent-write", "new code evidence requires trusted verification")
		}
	}
	return nil
}
