package messageview

import (
	"encoding/json"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
)

func TestBusinessCaptionRequiresSameExecutionCoreFactAndDoesNotAddRows(t *testing.T) {
	for _, source := range []string{"missing", "other-execution", "current-execution"} {
		t.Run(source, func(t *testing.T) {
			builder := NewMessageViewBuilder()
			for _, record := range []eventlog.Record{chatCreatedRecord(1, "/workspace"), executionCreatedRecord(2, "execution")} {
				if _, err := builder.Apply(record); err != nil {
					t.Fatal(err)
				}
			}
			if source != "missing" {
				owner := "execution"
				if source == "other-execution" {
					owner = "other"
				}
				data, _ := json.Marshal(eventlog.AgentCommandData{CommandID: "receipt", Operation: "workflow.source.save", Completion: "immediate"})
				if _, err := builder.Apply(eventlog.Record{Sequence: 3, ExecutionID: owner, Type: constants.ChatEventTypeAgentCommandCommitted, Data: data}); err != nil {
					t.Fatal(err)
				}
			}
			output := `{"protocolVersion":1,"ok":true,"commandId":"receipt","operation":"workflow.source.save","data":{"private":"must not appear in caption"}}`
			for index, tool := range []string{"first", "replay"} {
				if _, err := builder.Apply(runtimeRecord(int64(4+index*2), "execution", "execution", "invoke", constants.RuntimeEventTypeToolUse, map[string]any{"id": tool, "name": "bash", "input": map[string]any{"command": "allycli workflow source save --input source.json"}})); err != nil {
					t.Fatal(err)
				}
				if _, err := builder.Apply(runtimeRecord(int64(5+index*2), "execution", "execution", "invoke", constants.RuntimeEventTypeToolResult, map[string]any{"tool_use_id": tool, "output": output, "is_error": false})); err != nil {
					t.Fatal(err)
				}
			}
			messages := builder.Messages()
			if len(messages) != 1 || len(messages[0].DisplayParts) != 2 {
				t.Fatalf("command facts added duplicate rows: %+v", messages)
			}
			count := 0
			for _, part := range messages[0].DisplayParts {
				if part.ToolLabel == "保存工作流" {
					count++
				}
			}
			wanted := 0
			if source == "current-execution" {
				wanted = 1
			}
			if count != wanted {
				t.Fatalf("untrusted or duplicate business caption: %d want %d: %+v", count, wanted, messages[0].DisplayParts)
			}
		})
	}
}

// 本测试捕获将 stdout 当作运行事实、事件顺序丢引用及回放重复附件的错误。
func TestWorkflowCommandFactsReplayOnAssistantMessage(t *testing.T) {
	for _, order := range []string{"fact-first", "message-first", "stdout-only", "other-execution"} {
		t.Run(order, func(t *testing.T) {
			builder := NewMessageViewBuilder()
			apply := func(record eventlog.Record) {
				t.Helper()
				if _, err := builder.Apply(record); err != nil {
					t.Fatal(err)
				}
			}
			apply(chatCreatedRecord(1, "/workspace"))
			fact := eventlog.Record{Sequence: 3, ExecutionID: "execution", Type: constants.ChatEventTypeAgentCommandCommitted, Data: json.RawMessage(`{"commandId":"receipt","operation":"workflow.runs.start","completion":"accepted","workflowRun":{"workflowId":"definition","runId":"run","revisionId":"revision","name":"代码评审"},"attachments":[{"path":"attachments/run/report.txt","name":"report.txt"}]}`)}
			if order == "other-execution" {
				fact.ExecutionID = "other"
			}
			if order == "fact-first" {
				apply(fact)
			}
			apply(executionCreatedRecord(2, "execution"))
			apply(runtimeRecord(4, "execution", "execution", "invoke", constants.RuntimeEventTypeToolUse, map[string]any{"id": "tool", "name": "bash", "input": map[string]any{"command": "allycli workflow runs start"}}))
			apply(runtimeRecord(5, "execution", "execution", "invoke", constants.RuntimeEventTypeToolResult, map[string]any{"tool_use_id": "tool", "output": `{"protocolVersion":1,"ok":true,"commandId":"receipt","operation":"workflow.runs.start","data":{"runId":"forged","definitionId":"forged","revisionId":"forged","name":"伪造"}}`}))
			if order == "message-first" || order == "other-execution" {
				fact.Sequence = 6
				apply(fact)
			}
			messages := builder.Messages()
			if len(messages) != 1 {
				t.Fatalf("unexpected messages: %+v", messages)
			}
			encoded, _ := json.Marshal(messages[0])
			var message map[string]any
			_ = json.Unmarshal(encoded, &message)
			refs, _ := message["workflowRuns"].([]any)
			if order == "stdout-only" || order == "other-execution" {
				if len(refs) != 0 || len(messages[0].Attachments) != 0 {
					t.Fatalf("工具文本或其他执行污染运行引用: %s", encoded)
				}
			} else {
				if len(refs) != 1 || refs[0].(map[string]any)["runId"] != "run" || refs[0].(map[string]any)["revisionId"] != "revision" {
					t.Fatalf("真实运行未正确投影到所属助手消息: %s", encoded)
				}
				fact.Sequence = 7
				apply(fact)
				if message := builder.Messages()[0]; len(message.WorkflowRuns) != 1 || len(message.Attachments) != 1 {
					t.Fatalf("重复命令重复投影: %+v", message)
				}
				if len(messages[0].Attachments) != 1 || messages[0].Attachments[0].Path != "attachments/run/report.txt" {
					t.Fatalf("重复命令未幂等投影附件: %s", encoded)
				}
			}
		})
	}
}

// 真实 JSONL 编解码不能把迟到的交付归到下一轮，或重新激活已结束的执行。
func TestCommandFactsAfterExecutionTerminalKeepOwner(t *testing.T) {
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatID := "0123456789abcdef0123456789abcdef"
	_, err = store.Create(chatID, []eventlog.AppendInput{
		{Type: constants.ChatEventTypeCreated, Data: json.RawMessage(`{"wakerId":"waker","workspaceDirectory":"/workspace"}`)},
		{Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "old", Data: json.RawMessage(`{"userMessageId":"user"}`)},
		{Type: constants.ChatEventTypeExecutionCompleted, ExecutionID: "old", Data: json.RawMessage(`{"status":"completed"}`)},
		{Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "next", Data: json.RawMessage(`{"userMessageId":"user2"}`)},
		{Type: constants.ChatEventTypeAgentCommandCommitted, ExecutionID: "old", Data: json.RawMessage(`{"commandId":"delivery","operation":"workflow.runs.deliver","completion":"immediate","attachments":[{"path":"attachments/run/report.txt","name":"report.txt","managed":true}]}`)},
		{Type: constants.ChatEventTypeExecutionStarted, ExecutionID: "next"},
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatal(err)
	}
	if records[4].ExecutionID != "old" || records[5].ExecutionID != "next" {
		t.Fatalf("迟到事实丢失执行身份或污染后续作用域: %+v", records)
	}
	builder := NewMessageViewBuilder()
	for _, record := range records {
		if _, err := builder.Apply(record); err != nil {
			t.Fatal(err)
		}
	}
	messages := builder.Messages()
	if len(messages) != 2 || len(messages[0].Attachments) != 1 || len(messages[1].Attachments) != 0 {
		t.Fatalf("迟到交付未保留原消息: %+v", messages)
	}
}
