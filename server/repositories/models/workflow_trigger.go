package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() { dbhelper.RegisterTable(&WorkflowTriggerReceipt{}) }

// WorkflowTriggerReceipt 固定整份自动运行配置的保存身份，避免重试重复建立触发器。
type WorkflowTriggerReceipt struct {
	// WorkflowID 与请求身份共同唯一定位一次保存。
	WorkflowID string `gorm:"size:32;primaryKey"`
	// RequestID 是客户端在未知保存结果时复用的请求身份。
	RequestID string `gorm:"size:120;primaryKey"`
	// Digest 是规范化完整请求的摘要，禁止同键修改内容。
	Digest string
	// Fingerprint 是保存后的配置并发版本。
	Fingerprint string
	// TaskIDs 是本次保存涉及的触发器身份，不包含密钥明文。
	TaskIDs []string `gorm:"serializer:json;type:text"`
}

func (WorkflowTriggerReceipt) TableName() string { return "workflow_trigger_receipts" }
