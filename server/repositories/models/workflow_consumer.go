package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() { dbhelper.RegisterTable(&WorkflowConsumerLease{}) }

// WorkflowConsumerLease 记录用户为一个真实聊天回合选择的工作流能力。
type WorkflowConsumerLease struct {
	// ID 同时是唯一 Execution 身份和 scoped 能力身份。
	ID string `gorm:"type:varchar(32);primaryKey" json:"id"`
	// ChatID 是唯一获准使用本能力的权威 Chat。
	ChatID string `gorm:"type:varchar(32);not null;index" json:"chatId"`
	// DefinitionIDs 是用户明确选择的定义白名单，模型不能添加。
	DefinitionIDs []string `gorm:"serializer:json;type:text;not null" json:"definitionIds"`
	// PayloadDigest 固定本回合输入、选择和能力配置，防止首次请求重放扩大范围。
	PayloadDigest string `json:"-"`
	// AuthToken 是仅交给该回合 Runtime 的认证凭据。
	AuthToken string `json:"-"`
	// DeadlineAt 限定能力可以调用的最长时间，实际 Execution 终态会更早撤销。
	DeadlineAt int64 `json:"deadlineAt"`
	// CreatedAt 是签发能力的 Unix 毫秒时间。
	CreatedAt int64 `gorm:"autoCreateTime:milli" json:"createdAt"`
}

// TableName 返回使用者能力持久表名。
func (WorkflowConsumerLease) TableName() string { return "workflow_consumer_leases" }
