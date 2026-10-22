// Package core 提供独立知识库 SDK；不要求模型、HTTP 或 NeuxAlly 宿主。
package core

import "github.com/JieWaZi/neux-knowledge/internal/domain"

// 以下别名直接复用核心领域模型；SDK 调用者无需导入 internal。
type (
	JSON               = types.JSON
	StringArray        = types.StringArray
	SourceRef          = types.SourceRef
	SourceLocator      = types.SourceLocator
	CodeRef            = types.CodeRef
	PageEvidence       = types.PageEvidence
	Library            = types.Library
	SourceVersion      = types.SourceVersion
	KnowledgeProfile   = types.KnowledgeProfile
	PermissionSnapshot = types.PermissionSnapshot
	Page               = types.WikiPage
	Theme              = types.WikiFolder
	Revision           = types.WikiPageRevision
	Job                = types.Job
	Stage              = types.Stage
	Receipt            = types.Receipt
	Publication        = types.Publication
	PageMutation       = types.PageMutation
	ThemeMutation      = types.ThemeMutation
	SourceConclusion   = types.SourceConclusion
	WikiConfig         = types.WikiConfig
	IndexingStrategy   = types.IndexingStrategy
	CreateLibraryInput = types.CreateLibraryInput
	UpdateLibraryInput = types.UpdateLibraryInput
	ImportSourceInput  = types.ImportSourceInput
	ArchiveSourceInput = types.ArchiveSourceInput
	WritePageInput     = types.WritePageInput
	RestorePageInput   = types.RestorePageInput
	WriteThemeInput    = types.WriteThemeInput
	AcceptJobInput     = types.AcceptJobInput
	JobActionInput     = types.JobActionInput
	PageQuery          = types.PageQuery
)

// Error 与 ErrorCode 是独立核心稳定错误，不依赖宿主 HTTP 框架。
type Error = types.Error
type ErrorCode = types.ErrorCode

const (
	ErrInvalid                = types.ErrInvalid
	ErrNotFound               = types.ErrNotFound
	ErrVersionConflict        = types.ErrVersionConflict
	ErrIdempotencyConflict    = types.ErrIdempotencyConflict
	ErrScopeViolation         = types.ErrScopeViolation
	ErrCancelled              = types.ErrCancelled
	ErrStaleAttempt           = types.ErrStaleAttempt
	ErrExecutionNotSuccessful = types.ErrExecutionNotSuccessful
	ErrCandidateInvalid       = types.ErrCandidateInvalid
	ErrManualProtected        = types.ErrManualProtected
	ErrDatabaseBusy           = types.ErrDatabaseBusy
	ErrDatabase               = types.ErrDatabase
	ErrStorage                = types.ErrStorage
)

// FailJobInput 是无页面写入的真实失败结算输入。
type FailJobInput = types.FailJobInput
