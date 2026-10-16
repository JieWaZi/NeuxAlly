// Package util 提供跨领域入口复用的轻量规范化方法。
package util

import (
	"github.com/google/uuid"
	"strings"
)

// NormalizeName 清理用户可见名称的首尾空白并保留原始大小写。
func NormalizeName(value string) string {
	return strings.TrimSpace(value)
}

// New 返回 32 位无连字符 UUID 字符串。
func New() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
