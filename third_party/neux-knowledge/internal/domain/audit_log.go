// 来源：Tencent/WeKnora 固定提交 8776a2d92d11ad3e76865d51cf3192e37e2a9976。
// 原完整许可见 upstream/WeKnora.LICENSE，迁移边界见 upstream/manifest.json。
package types

import (
	"context"
	"time"
)

// AuditAction 沿用上游点分命名的知识库活动类别。
type AuditAction string

const (
	AuditActionKBCreated               AuditAction = "kb.created"
	AuditActionKBUpdated               AuditAction = "kb.updated"
	AuditActionKBDeleted               AuditAction = "kb.deleted"
	AuditActionKnowledgeCreated        AuditAction = "knowledge.created"
	AuditActionKnowledgeUpdated        AuditAction = "knowledge.updated"
	AuditActionKnowledgeDeleted        AuditAction = "knowledge.deleted"
	AuditActionKnowledgeReparseStarted AuditAction = "knowledge.reparse_started"
	AuditActionKnowledgeParseCanceled  AuditAction = "knowledge.parse_canceled"
	AuditActionWikiContentChanged      AuditAction = "wiki.content_changed"
)

// AuditOutcome separates asynchronous acceptance from terminal business
// results and middleware-level rejections. The UI can therefore avoid
// presenting a queued operation as already completed.
type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "success"
	// Accepted means the durable request was created and asynchronous work was
	// submitted, but the background operation has not reached a terminal state.
	AuditOutcomeAccepted AuditOutcome = "accepted"
	AuditOutcomeDenied   AuditOutcome = "denied"
	AuditOutcomeFailed   AuditOutcome = "failed"
	AuditOutcomePartial  AuditOutcome = "partial"
	AuditOutcomeCanceled AuditOutcome = "canceled"
)

// AuditLog 保留上游不可变事件、作用域与自增游标模型，适配本机单库权威。
// RequestID 与动作、结果联合唯一，使活动与业务回执在同一事务内幂等。
type AuditLog struct {
	// ID 是递增事件游标，按倒序分页。
	ID uint64 `json:"id" gorm:"primaryKey;autoIncrement;index:idx_activity_scope_desc,priority:3,sort:desc"`
	// RequestID 绑定实际已提交的幂等写入。
	RequestID string `json:"request_id" gorm:"not null;uniqueIndex:idx_activity_request_event,priority:1"`
	// ActorUserID 是真实发起人；系统触发且没有用户时为空。
	ActorUserID string `json:"actor_user_id" gorm:"index:idx_activity_actor"`
	// ActorRole 保留宿主已验证的角色，不推断管理员身份。
	ActorRole string `json:"actor_role"`
	// Operation 绑定实际写入用例，与请求、动作及结果共同幂等。
	Operation string `json:"operation" gorm:"not null;uniqueIndex:idx_activity_request_event,priority:2"`
	// Action 是沿用上游合同的动作类别。
	Action AuditAction `json:"action" gorm:"not null;uniqueIndex:idx_activity_request_event,priority:3"`
	// ScopeType 固定知识库作用域。
	ScopeType string `json:"scope_type" gorm:"index:idx_activity_scope_desc,priority:1"`
	// ScopeID 是该活动所属知识库。
	ScopeID string `json:"scope_id" gorm:"index:idx_activity_scope_desc,priority:2"`
	// TargetType 表示实际变更的库、原件或整理任务。
	TargetType string `json:"target_type"`
	// TargetID 是实际目标资源身份。
	TargetID string `json:"target_id"`
	// Outcome 区分持久接纳与真实终态。
	Outcome AuditOutcome `json:"outcome" gorm:"not null;uniqueIndex:idx_activity_request_event,priority:4"`
	// Details 仅保存白名单内的有界元数据，不保存正文、凭据或诊断原文。
	Details JSON `json:"details" gorm:"type:json"`
	// CreatedAt 是提交事件的实际时间，分页仍以 ID 为准。
	CreatedAt time.Time `json:"created_at"`
}

// TableName 固定本地活动表，不依赖 GORM 复数推断。
func (AuditLog) TableName() string { return "knowledge_activity_logs" }

// ActivityQuery 沿用上游游标、页大小与精确过滤；库范围由入口固定。
type ActivityQuery struct {
	// AfterID 是上一页最后事件标识，零表示从最新事件开始。
	AfterID uint64
	// Limit 默认五十，最大一百。
	Limit int
	// Action 精确匹配动作；空值不限制。
	Action AuditAction
	// Outcome 精确匹配结果；空值不限制。
	Outcome AuditOutcome
	// ActorUserID 精确匹配真实用户标识；空值不限制。
	ActorUserID string
}

// ActivityActor 由受信宿主注入并随任务接纳冻结，不能从请求正文取得。
type ActivityActor struct {
	// UserID 是宿主认证主体；无真实用户时为空。
	UserID string `json:"userId,omitempty"`
	// Role 是宿主验证的角色；本机未设角色时为空。
	Role string `json:"role,omitempty"`
	// Trigger 标识 user 或 system 触发来源；省略时按实际身份派生。
	Trigger string `json:"trigger,omitempty"`
}

type activityActorContextKey struct{}

// WithActivityActor 将真实发起身份附在本次受信调用上下文。
func WithActivityActor(ctx context.Context, actor ActivityActor) context.Context {
	return context.WithValue(ctx, activityActorContextKey{}, actor)
}

// ActivityActorFromContext 返回实际发起身份，缺省上下文属于系统触发。
func ActivityActorFromContext(ctx context.Context) ActivityActor {
	actor, _ := ctx.Value(activityActorContextKey{}).(ActivityActor)
	if actor.Trigger == "" {
		if actor.UserID != "" {
			actor.Trigger = "user"
		} else {
			actor.Trigger = "system"
		}
	}
	if actor.UserID == "" {
		actor.Role = ""
	}
	return actor
}
