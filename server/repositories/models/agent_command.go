package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() { dbhelper.RegisterTable(&AgentCommandReceipt{}) }

// AgentCommandReceipt 只保存主数据库领域命令的原子接纳事实。
type AgentCommandReceipt struct {
	// ID 绑定服务端作用域、精确操作和调用方请求身份。
	ID string `gorm:"type:varchar(64);primaryKey"`
	// Digest 是规范输入摘要，不保存输入原文或凭据。
	Digest string `gorm:"type:varchar(64);not null"`
	// Data 保存首次提交的最小业务回执。
	Data map[string]any `gorm:"serializer:json;type:text;not null"`
	// CreatedAt 是与领域变更一起提交的 Unix 毫秒。
	CreatedAt int64 `gorm:"autoCreateTime:milli"`
}

func (AgentCommandReceipt) TableName() string { return "agent_command_receipts" }
