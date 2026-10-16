package workspace

import (
	"strings"
	"testing"
)

// 最终交付不能把缺少 Chat 归属标记的任意目录变成受管附件根。
func TestDeliveredAttachmentRequiresActualChatWorkspace(t *testing.T) {
	directory := t.TempDir()
	if _, _, err := ReadDeliveredAttachment(directory, "chat", "delivery", "report.txt"); err == nil {
		t.Fatal("缺失 Chat 根归属标记时交付恢复没有拒绝")
	}
	if _, err := SaveDeliveredAttachment(directory, "chat", "delivery", "report.txt", strings.NewReader("report")); err == nil {
		t.Fatal("缺失归属标记的目录接受了交付")
	}
}
