package types

import "fmt"

// ErrorCode 是独立 SDK 的稳定错误分类。
type ErrorCode string

const (
	ErrInvalid                ErrorCode = "invalid"
	ErrNotFound               ErrorCode = "not_found"
	ErrVersionConflict        ErrorCode = "version_conflict"
	ErrIdempotencyConflict    ErrorCode = "idempotency_conflict"
	ErrScopeViolation         ErrorCode = "scope_violation"
	ErrSourceUnavailable      ErrorCode = "source_unavailable"
	ErrCancelled              ErrorCode = "cancelled"
	ErrStaleAttempt           ErrorCode = "stale_attempt"
	ErrExecutionNotSuccessful ErrorCode = "execution_not_successful"
	ErrCandidateInvalid       ErrorCode = "candidate_invalid"
	ErrManualProtected        ErrorCode = "manual_protected"
	ErrDatabaseBusy           ErrorCode = "database_busy"
	ErrDatabase               ErrorCode = "database_error"
	ErrStorage                ErrorCode = "storage_error"
)

// Error 保存稳定类别与原始技术原因。
type Error struct {
	Code      ErrorCode // 可供宿主映射的稳定错误代码。
	Operation string    // 失败领域操作。
	Detail    string    // 技术诊断，不充当界面文案。
	Cause     error     // 原始底层原因。
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %s: %v", e.Code, e.Operation, e.Detail, e.Cause)
	}
	return fmt.Sprintf("%s: %s: %s", e.Code, e.Operation, e.Detail)
}
func (e *Error) Unwrap() error { return e.Cause }
