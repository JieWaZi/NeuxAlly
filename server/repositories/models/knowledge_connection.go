package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() {
	dbhelper.RegisterTable(&KnowledgeConnectionSecret{})
	dbhelper.RegisterTable(&KnowledgeConnectionStorageKey{})
	dbhelper.RegisterTable(&KnowledgeConnectionMutation{})
}

// KnowledgeConnectionSecret 将凭据密文与普通连接展示分开。
type KnowledgeConnectionSecret struct {
	// ID 为绑定连接身份，复制密文到其他身份不能解密。
	ID string `gorm:"primaryKey" json:"-"`
	// Ciphertext 为AES-GCM密文，不返回API。
	Ciphertext string `json:"-"`
}

func (KnowledgeConnectionSecret) TableName() string { return "knowledge_connection_secrets" }

// KnowledgeConnectionStorageKey 遵循既有凭据库的随机密钥保护机制。
type KnowledgeConnectionStorageKey struct {
	// ID 为此数据库固定密钥记录。
	ID string `gorm:"primaryKey" json:"-"`
	// Key 为AES-256随机密钥，不对外返回。
	Key []byte `json:"-"`
}

func (KnowledgeConnectionStorageKey) TableName() string { return "knowledge_connection_storage_keys" }

// KnowledgeConnectionMutation 使连接编辑重试返回原回执，避免重复修改版本。
type KnowledgeConnectionMutation struct {
	// RequestID 为用户请求幂等身份。
	RequestID string `gorm:"primaryKey" json:"-"`
	// Digest 为包括凭据摘要的请求摘要。
	Digest string `json:"-"`
	// ConnectionID 为本次连接身份。
	ConnectionID string `json:"connectionId"`
	// Revision 为本次提交时的版本。
	Revision int64 `json:"revision"`
}

func (KnowledgeConnectionMutation) TableName() string { return "knowledge_connection_mutations" }
