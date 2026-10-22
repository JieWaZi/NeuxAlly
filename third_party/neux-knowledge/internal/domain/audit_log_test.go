// 来源：Tencent/WeKnora 8776a2d internal/types/audit_log_test.go 的命名空间与不重复断言。
// 只保留当前知识库公开活动范围，不迁全局管理或第三方存储动作。
package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditActionDotNamespaceConvention(t *testing.T) {
	all := []AuditAction{AuditActionKBCreated, AuditActionKBUpdated, AuditActionKBDeleted, AuditActionKnowledgeCreated, AuditActionKnowledgeUpdated, AuditActionKnowledgeDeleted, AuditActionKnowledgeReparseStarted, AuditActionKnowledgeParseCanceled, AuditActionWikiContentChanged}
	seen := map[AuditAction]bool{}
	for _, a := range all {
		s := string(a)
		area, event, ok := strings.Cut(s, ".")
		assert.True(t, ok, "action %q must contain exactly one dot separator", s)
		assert.NotEmpty(t, area, "action %q has empty area", s)
		assert.NotEmpty(t, event, "action %q has empty event", s)
		if seen[a] {
			t.Fatalf("collision: %q", a)
		}
		seen[a] = true
	}
}
