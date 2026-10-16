package chats

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListChatMessagesDefaultsAndRejectsInvalidCursor(t *testing.T) {
	request := ListChatMessages{ChatID: "chat-1"}
	if err := request.Validate(&gin.Context{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if request.Limit != 50 {
		t.Fatalf("default limit = %d, want 50", request.Limit)
	}

	invalid := ListChatMessages{
		ChatID:         "chat-1",
		BeforeSequence: -1,
	}
	if err := invalid.Validate(&gin.Context{}); err == nil {
		t.Fatal("negative beforeSequence should be rejected")
	}
}
