package chats

import (
	"encoding/json"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/go-playground/validator/v10"
)

// 本测试验证首发和续发公开请求均限制一个工作流，且不接纳伪造的引用快照。
func TestChatWorkflowInputAllowsOneDefinitionAndIgnoresForgedFacts(t *testing.T) {
	validate := validator.New()
	for _, create := range []bool{false, true} {
		for _, count := range []int{0, 1, 2} {
			message := MessageRequest{Text: "评审改动", RuntimeID: "codex", PermissionLevel: constants.PermissionLevelDefault}
			for index := 0; index < count; index++ {
				message.WorkflowIDs = append(message.WorkflowIDs, "definition")
			}
			var request any = message
			if create {
				request = CreateChatRequest{ClientRequestID: "first", WakerID: "waker", MessageRequest: message}
			}
			err := validate.Struct(request)
			if (err != nil) != (count > 1) {
				t.Errorf("create=%t count=%d validation=%v", create, count, err)
			}
		}
	}
	var request MessageRequest
	if err := json.Unmarshal([]byte(`{"text":"评审改动","workflowIds":["real-id"],"workflows":[{"id":"forged","name":"伪造"}],"workflowRuns":[{"runId":"forged"}]}`), &request); err != nil {
		t.Fatal(err)
	}
	input, _ := request.workInput()
	encoded, _ := json.Marshal(input)
	var fields map[string]any
	_ = json.Unmarshal(encoded, &fields)
	if fields["workflows"] != nil || fields["workflowRuns"] != nil {
		t.Fatalf("公开请求可伪造可信事实: %s", encoded)
	}
}
