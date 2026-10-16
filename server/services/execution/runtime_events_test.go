package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability/contract"
	eventlog "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	eventstream "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventstream"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/messageview"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
)

func TestRuntimeEventSinkPersistsOnlyCompleteAssistantMessage(t *testing.T) {
	t.Parallel()
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	chatID := "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sink, _ := newRuntimeEventSinkForTest(store, chatID)
	for _, text := range []string{"one", "two"} {
		if err := sink.acceptInvocation(context.Background(), "invocation-1", runtimes.Event{
			Type: runtimes.EventTypeTextDelta,
			Data: marshalData(struct {
				// Text 是本次测试交付的实时文本增量。
				Text string `json:"text"`
			}{Text: text}),
		}); err != nil {
			t.Fatalf("Accept() error = %v", err)
		}
	}
	beforeFlush, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() before flush error = %v", err)
	}
	if len(beforeFlush) != 3 {
		t.Fatalf("records before flush = %d, want 3", len(beforeFlush))
	}
	if err := sink.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	afterFlush, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() after flush error = %v", err)
	}
	if len(afterFlush) != 4 || afterFlush[3].Type != constants.ChatEventTypeAssistantMessage {
		t.Fatalf("records after flush = %+v", afterFlush)
	}
	var persisted eventlog.RuntimeMessageData
	if err := json.Unmarshal(afterFlush[3].Data, &persisted); err != nil {
		t.Fatalf("decode persisted execution message: %v", err)
	}
	if len(persisted.Content) != 1 ||
		persisted.Content[0].Type != runtimes.EventTypeTextDelta ||
		string(persisted.Content[0].Data) != `{"text":"onetwo"}` ||
		persisted.MessageRevision != 5 {
		t.Fatalf("persisted execution message = %#v", persisted)
	}
}

func TestRuntimeEventSinkPublishesCompleteLiveSnapshotBeforePersistence(t *testing.T) {
	t.Parallel()
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	chatID := "abababababababababababababababab"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sink, live := newRuntimeEventSinkForTest(store, chatID)
	for _, text := range []string{"你好", "！"} {
		if err := sink.acceptInvocation(context.Background(), "invocation-1", runtimes.Event{
			Type: runtimes.EventTypeTextDelta,
			Data: marshalData(struct {
				// Text 是验证实时消息视图立即可见的文本增量。
				Text string `json:"text"`
			}{Text: text}),
		}); err != nil {
			t.Fatalf("Accept() error = %v", err)
		}
	}

	var lastLive *messageview.Message
	for range 2 {
		select {
		case event := <-live:
			if !event.Transient || event.Sequence != 0 || event.Message == nil {
				t.Fatalf("live event = %#v", event)
			}
			lastLive = event.Message
		case <-time.After(time.Second):
			t.Fatal("live snapshot was not published immediately")
		}
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records before complete message = %d, want 3", len(records))
	}
	if err := sink.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	records, err = store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() after flush error = %v", err)
	}
	persisted, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatalf("ReplayMessages() error = %v", err)
	}
	if lastLive == nil || len(persisted) != 1 ||
		persisted[0].Content != lastLive.Content ||
		persisted[0].Revision != lastLive.Revision ||
		!reflect.DeepEqual(persisted[0].DisplayParts, lastLive.DisplayParts) {
		t.Fatalf("live snapshot %#v != persisted snapshot %#v", lastLive, persisted)
	}
}

func TestRuntimeEventSinkKeepsFileChangesEqualBetweenLiveAndReplay(t *testing.T) {
	t.Parallel()
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	chatID := "bcbcbcbcbcbcbcbcbcbcbcbcbcbcbcbc"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sink, live := newRuntimeEventSinkForTest(store, chatID)
	for _, event := range []runtimes.Event{
		{
			Type: runtimes.EventTypeToolUse,
			Data: marshalData(map[string]any{
				"name": "file_modify", "status": "in_progress", "toolCallId": "edit-1",
				"content": []any{map[string]any{
					"type": "diff", "path": "/workspace/project/main.go",
					"oldText": "old\n", "newText": "new\n",
				}},
			}),
		},
		{
			Type: runtimes.EventTypeToolResult,
			Data: marshalData(map[string]any{
				"name": "file_modify", "status": "completed", "toolCallId": "edit-1",
				"content": []any{map[string]any{
					"type": "diff", "path": "/workspace/project/main.go",
					"oldText": "old\n", "newText": "new\nmore\n",
				}},
			}),
		},
	} {
		if err := sink.acceptInvocation(context.Background(), "invocation-1", event); err != nil {
			t.Fatalf("Accept(%s) error = %v", event.Type, err)
		}
	}

	var lastLive *messageview.Message
	liveCount := 0
	for liveCount < 2 {
		select {
		case event := <-live:
			if event.Message == nil {
				t.Fatalf("live event = %#v", event)
			}
			lastLive = event.Message
			liveCount++
		case <-time.After(time.Second):
			t.Fatal("live file change snapshot was not published")
		}
	}
	if err := sink.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	persisted, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatalf("ReplayMessages() error = %v", err)
	}
	if lastLive == nil || len(persisted) != 1 ||
		!reflect.DeepEqual(persisted[0].FileChanges, lastLive.FileChanges) ||
		len(persisted[0].FileChanges) != 1 ||
		persisted[0].FileChanges[0].AddedLines != 2 ||
		persisted[0].FileChanges[0].DeletedLines != 1 {
		t.Fatalf("live file changes %#v != persisted %#v", lastLive, persisted)
	}
}

func TestExecutionTerminalRevisionStaysAheadOfTransientOnlyEvents(t *testing.T) {
	t.Parallel()
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	chatID := "acacacacacacacacacacacacacacacac"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sink, _ := newRuntimeEventSinkForTest(store, chatID)
	for index := 0; index < 20; index++ {
		if err := sink.acceptInvocation(context.Background(), "invocation-1", runtimes.Event{
			Type: runtimes.EventTypeUsage,
			Data: marshalData(map[string]any{"turn": map[string]any{"output_tokens": index + 1}}),
		}); err != nil {
			t.Fatalf("Accept(usage %d) error = %v", index, err)
		}
	}
	if _, err := store.Append(chatID, []eventlog.AppendInput{{
		Type:        constants.ChatEventTypeExecutionCompleted,
		ExecutionID: "execution-1",
		Data: marshalData(eventlog.ExecutionFinishedData{
			Status:          constants.ExecutionStatusCompleted,
			Usage:           sink.Usage(),
			MessageRevision: sink.MessageRevision(),
		}),
	}}); err != nil {
		t.Fatalf("Append(execution completed) error = %v", err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatalf("ReplayMessages() error = %v", err)
	}
	if len(messages) != 1 || messages[0].Status != constants.ExecutionStatusCompleted ||
		messages[0].Revision <= sink.MessageRevision() {
		t.Fatalf("terminal message = %+v, transient revision = %d", messages, sink.MessageRevision())
	}
}

func TestRuntimeEventSinkPersistsToolMessagesAndKeepsOnlyLatestUsage(t *testing.T) {
	t.Parallel()
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	chatID := "dededededededededededededededede"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sink, _ := newRuntimeEventSinkForTest(store, chatID)
	for _, event := range []runtimes.Event{
		{Type: runtimes.EventTypeStatus, Data: json.RawMessage(`{"state":"thinking"}`)},
		{Type: runtimes.EventTypeRaw, Data: json.RawMessage(`{"type":"system","subtype":"thinking_tokens"}`)},
		{Type: runtimes.EventTypeToolUse, Data: marshalData(map[string]any{"id": "tool-1", "name": "Bash"})},
		{Type: runtimes.EventTypeToolResult, Data: marshalData(map[string]any{"tool_use_id": "tool-1", "is_error": false})},
		{Type: runtimes.EventTypeUsage, Data: json.RawMessage(`{"turn":{"output_tokens":10}}`)},
		{Type: runtimes.EventTypeUsage, Data: json.RawMessage(`{"turn":{"output_tokens":12}}`)},
	} {
		if err := sink.acceptInvocation(context.Background(), "invocation-1", event); err != nil {
			t.Fatalf("Accept(%s) error = %v", event.Type, err)
		}
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(records) != 5 ||
		records[3].Type != constants.ChatEventTypeAssistantMessage ||
		records[4].Type != constants.ChatEventTypeToolMessage {
		t.Fatalf("persisted tool records = %+v", records)
	}
	if usage := string(sink.Usage()); usage != `{"turn":{"output_tokens":12}}` {
		t.Fatalf("Usage() = %s, want latest usage", usage)
	}
	if _, err := store.Append(chatID, []eventlog.AppendInput{{
		Type:        constants.ChatEventTypeExecutionCompleted,
		ExecutionID: "execution-1",
		Data: marshalData(eventlog.ExecutionFinishedData{
			Status: constants.ExecutionStatusCompleted,
			Usage:  sink.Usage(),
		}),
	}}); err != nil {
		t.Fatalf("Append(execution completed) error = %v", err)
	}
	records, err = store.ReadAll(chatID)
	if err != nil {
		t.Fatalf("ReadAll() after terminal error = %v", err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatalf("ReplayMessages() error = %v", err)
	}
	if len(messages) != 1 || messages[0].RunMetrics == nil ||
		messages[0].RunMetrics.OutputTokens != 12 {
		t.Fatalf("replayed usage = %+v", messages)
	}
}

func chatCreatedInput() eventlog.AppendInput {
	return eventlog.AppendInput{
		Type: constants.ChatEventTypeCreated,
		Data: marshalData(eventlog.ChatCreatedData{
			WakerID:            "waker-1",
			WorkspaceDirectory: "/workspace/project",
			Title:              "测试对话",
		}),
	}
}

func activeChatInputs() []eventlog.AppendInput {
	return []eventlog.AppendInput{
		chatCreatedInput(),
		{
			Type:        constants.ChatEventTypeExecutionCreated,
			ExecutionID: "execution-1",
			Data:        marshalData(eventlog.ExecutionCreatedData{}),
		},
		{
			Type:         constants.ChatEventTypeExecutionStarted,
			ExecutionID:  "execution-1",
			InvocationID: "invocation-1",
		},
	}
}

func TestStartedTurnUsesExecutionAsAssistantIdentityWithoutPlaceholderFact(t *testing.T) {
	t.Parallel()

	chatID := "12121212121212121212121212121212"
	executionID := "34343434343434343434343434343434"
	inputs := startedMessageEvents(
		messageChat{
			isNew: true,
			chat: models.Chat{
				ID:                 chatID,
				WakerID:            "56565656565656565656565656565656",
				WorkspaceDirectory: t.TempDir(),
				Title:              "测试对话",
			},
		},
		pendingMessage{
			message: userMessage{Text: "开始"},
			runtime: runtimeSelection{RuntimeID: "codex"},
		},
		models.Execution{
			ID:            executionID,
			ChatID:        chatID,
			UserMessageID: "78787878787878787878787878787878",
			RuntimeID:     "codex",
		},
		"90909090909090909090909090909090",
		nil,
		nil,
		nil,
		nil,
	)
	inputs = append(inputs, eventlog.AppendInput{
		Type:         constants.ChatEventTypeExecutionStarted,
		ExecutionID:  executionID,
		InvocationID: "90909090909090909090909090909090",
	})
	if len(inputs) != 4 {
		t.Fatalf("accepted first turn facts = %d, want 4", len(inputs))
	}
	for _, input := range inputs {
		if string(input.Type) == "message."+"assistant_created" {
			t.Fatal("accepted turn must not persist an empty assistant placeholder")
		}
	}

	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	records, err := store.Create(chatID, inputs)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	messageIDs := make([]string, 0, 2)
	for _, record := range records {
		if record.MessageID != "" {
			messageIDs = append(messageIDs, record.MessageID)
		}
	}
	if len(messageIDs) != 2 || messageIDs[1] != executionID {
		t.Fatalf("message identities = %q, want user ID then execution ID %q", messageIDs, executionID)
	}
}

// runtimeEventTestJournal 为 Runtime 聚合测试提供最小权威事实端口。
type runtimeEventTestJournal struct {
	// store 保存测试中的权威 JSONL 记录。
	store *eventlog.Store
	// live 接收不入 JSONL 的 transient 消息快照。
	live chan eventstream.PublicEvent
}

// ForkChat 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) ForkChat(context.Context, chatservice.ForkInput, chatservice.NativeFork) (string, error) {
	panic("unexpected ForkChat call")
}

// LoadExecutionContext 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) LoadExecutionContext(context.Context, string) (chatservice.ExecutionContext, error) {
	panic("unexpected LoadExecutionContext call")
}

// CommitStartedTurn 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) CommitStartedTurn(context.Context, chatservice.StartedTurn) (chatservice.AcceptedTurn, error) {
	panic("unexpected CommitStartedTurn call")
}

// CommitAcceptedTurn 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) CommitAcceptedTurn(context.Context, chatservice.StartedTurn) (chatservice.AcceptedTurn, error) {
	panic("unexpected CommitAcceptedTurn call")
}

// MarkExecutionStarted 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) MarkExecutionStarted(context.Context, models.Execution, string) (models.Execution, error) {
	panic("unexpected MarkExecutionStarted call")
}

// ReadRecords 返回建立实时投影所需的当前事实。
func (j *runtimeEventTestJournal) ReadRecords(chatID string) ([]eventlog.Record, error) {
	return j.store.ReadAll(chatID)
}

// AppendExecutionFacts 同步追加完整 Runtime 消息事实。
func (j *runtimeEventTestJournal) AppendExecutionFacts(_ context.Context, chatID string, facts []eventlog.AppendInput) ([]eventlog.Record, error) {
	return j.store.Append(chatID, facts)
}

// AppendApprovalFact 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) AppendApprovalFact(string, eventlog.AppendInput) (eventlog.Record, error) {
	panic("unexpected AppendApprovalFact call")
}

// PublishApprovalFact 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) PublishApprovalFact(context.Context, eventlog.Record, []models.PendingApproval) {
	panic("unexpected PublishApprovalFact call")
}

// PublishElicitationFact 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) PublishElicitationFact(context.Context, eventlog.Record, []models.PendingElicitation) {
	panic("unexpected PublishElicitationFact call")
}

// AppendElicitationFact 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) AppendElicitationFact(string, eventlog.AppendInput) (eventlog.Record, error) {
	panic("unexpected AppendElicitationFact call")
}

// CommitTerminal 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) CommitTerminal(context.Context, chatservice.TerminalFact) (chatservice.TerminalCommit, error) {
	panic("unexpected CommitTerminal call")
}

// PublishTerminal 不参与 Runtime 事件聚合测试。
func (j *runtimeEventTestJournal) PublishTerminal(chatservice.TerminalCommit) {
	panic("unexpected PublishTerminal call")
}

// PublishLive 交付 transient 消息快照供测试断言。
func (j *runtimeEventTestJournal) PublishLive(_ string, event eventstream.PublicEvent) {
	j.live <- event
}

// newRuntimeEventSinkForTest 组合 Runtime 事件管线的最小 Journal 端口。
func newRuntimeEventSinkForTest(
	store *eventlog.Store,
	chatID string,
) (*runtimeEventSink, <-chan eventstream.PublicEvent) {
	live := make(chan eventstream.PublicEvent, 256)
	journal := &runtimeEventTestJournal{store: store, live: live}
	return &runtimeEventSink{
		journal:     journal,
		chatID:      chatID,
		executionID: "execution-1",
	}, live
}

func TestCommandFactsKeepLiveAndDurableToolCaptionsEqual(t *testing.T) {
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatID := "99999999999999999999999999999999"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatal(err)
	}
	sink, live := newRuntimeEventSinkForTest(store, chatID)
	sink.agentFacts = &agentCommandFacts{}
	send := func(kind runtimes.EventType, data map[string]any) {
		t.Helper()
		if err := sink.acceptInvocation(context.Background(), "invocation-1", runtimes.Event{Type: kind, Data: marshalData(data)}); err != nil {
			t.Fatal(err)
		}
	}
	send(runtimes.EventTypeToolUse, map[string]any{"id": "tool", "name": "bash", "input": map[string]any{"command": "allycli workflow source save --input source.json"}})
	op, _ := contract.Find("workflow.source.save")
	if err := sink.agentFacts.record(context.Background(), sink.journal, chatID, "execution-1", op, agentcapability.Result{CommandID: "receipt"}); err != nil {
		t.Fatal(err)
	}
	send(runtimes.EventTypeToolResult, map[string]any{"tool_use_id": "tool", "output": `{"protocolVersion":1,"ok":true,"commandId":"receipt","operation":"workflow.source.save"}`, "is_error": false})
	send(runtimes.EventTypeTextDelta, map[string]any{"text": "done"})
	var lastLive *messageview.Message
	for index := range 3 {
		event := <-live
		lastLive = event.Message
		if index == 0 {
			continue
		}
		for _, part := range event.Message.DisplayParts {
			if part.ToolCallID == "tool" && part.ToolLabel != "保存工作流" {
				t.Fatalf("live business caption lost: %+v", part)
			}
		}
	}
	if err := sink.Flush(); err != nil {
		t.Fatal(err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || !reflect.DeepEqual(messages[0].DisplayParts, lastLive.DisplayParts) {
		t.Fatalf("live/durable diverged: live=%+v durable=%+v", lastLive, messages)
	}
}

// 本测试捕获命令记录丢失可信 Core 回执或实时快照与日志重放不同步。
func TestWorkflowCommandFactsKeepLiveAndDurableReferencesEqual(t *testing.T) {
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatID := "98989898989898989898989898989898"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatal(err)
	}
	sink, live := newRuntimeEventSinkForTest(store, chatID)
	sink.agentFacts = &agentCommandFacts{}
	send := func(text string) {
		t.Helper()
		if err := sink.acceptInvocation(context.Background(), "invocation-1", runtimes.Event{Type: runtimes.EventTypeTextDelta, Data: marshalData(map[string]any{"text": text})}); err != nil {
			t.Fatal(err)
		}
	}
	send("正在启动")
	<-live
	op, _ := contract.Find("workflow.runs.start")
	result := agentcapability.Result{CommandID: "start-receipt", Data: map[string]any{"runId": "actual-run", "definitionId": "definition", "revisionId": "revision", "name": "代码评审"}}
	if err := sink.agentFacts.record(context.Background(), sink.journal, chatID, "execution-1", op, result); err != nil {
		t.Fatal(err)
	}
	send("等待结果")
	snapshot := (<-live).Message
	if err := sink.Flush(); err != nil {
		t.Fatal(err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []*messageview.Message{snapshot, &messages[0]} {
		encoded, _ := json.Marshal(message)
		var public map[string]any
		_ = json.Unmarshal(encoded, &public)
		refs, _ := public["workflowRuns"].([]any)
		if len(refs) != 1 || refs[0].(map[string]any)["runId"] != "actual-run" {
			t.Fatalf("Core运行事实未进入实时/持久消息: %s", encoded)
		}
	}
	if messages[0].Revision != snapshot.Revision {
		t.Fatalf("实时和重放消息修订不一致: %d != %d", messages[0].Revision, snapshot.Revision)
	}
}

// 新回合查询旧回执时，可信运行身份控制持久消息登记，当前执行不能替代原归属。
func TestDeliveryCommandFactUsesOriginalExecution(t *testing.T) {
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatID := "73737373737373737373737373737373"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatal(err)
	}
	sink, _ := newRuntimeEventSinkForTest(store, chatID)
	if _, err := store.Append(chatID, []eventlog.AppendInput{{Type: constants.ChatEventTypeExecutionCompleted, ExecutionID: "execution-1", Data: marshalData(eventlog.ExecutionFinishedData{Status: constants.ExecutionStatusCompleted})}, {Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "next", Data: marshalData(eventlog.ExecutionCreatedData{UserMessageID: "next-user"})}}); err != nil {
		t.Fatal(err)
	}
	op, _ := contract.Find("workflow.runs.deliver")
	facts := &agentCommandFacts{}
	result := agentcapability.Result{CommandID: "delivery", ExecutionID: "execution-1", Data: map[string]any{"runId": "run", "definitionId": "definition", "revisionId": "revision", "name": "报告", "attachments": []eventlog.AttachmentReference{{Name: "report.txt", Path: "attachments/run/report.txt", Managed: true}}}}
	if err := facts.record(context.Background(), sink.journal, chatID, "next", op, result); err != nil {
		t.Fatal(err)
	}
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatal(err)
	}
	attached := 0
	for _, message := range messages {
		if len(message.Attachments) > 0 {
			attached++
			if message.ExecutionID != "execution-1" || len(message.WorkflowRuns) != 1 {
				t.Fatalf("新回合覆盖原交付身份: %+v", message)
			}
		}
	}
	if attached != 1 {
		t.Fatal("原消息没有交付")
	}
}

// 旧 start、deliver 回执及补交付必须只更新原消息，后续新回合文本仍可实时输出。
func TestOldWorkflowFactsDoNotInterruptCurrentExecution(t *testing.T) {
	for _, scenario := range []struct {
		// name 区分旧回执查询和旧运行首次补交付。
		name string
		// operation 是服务端登记的工作流操作。
		operation string
		// replayed 标识是否来自已存在的领域回执。
		replayed bool
	}{{"start-receipt", "workflow.runs.start", true}, {"deliver-receipt", "workflow.runs.deliver", true}, {"late-delivery", "workflow.runs.deliver", false}} {
		operation := scenario.operation
		t.Run(scenario.name, func(t *testing.T) {
			store, err := eventlog.NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			chatID := "74747474747474747474747474747474"
			if _, err := store.Create(chatID, activeChatInputs()); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Append(chatID, []eventlog.AppendInput{{Type: constants.ChatEventTypeExecutionCompleted, ExecutionID: "execution-1", Data: marshalData(eventlog.ExecutionFinishedData{Status: constants.ExecutionStatusCompleted})}, {Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "next", Data: marshalData(eventlog.ExecutionCreatedData{UserMessageID: "next-user"})}}); err != nil {
				t.Fatal(err)
			}
			sink, live := newRuntimeEventSinkForTest(store, chatID)
			sink.executionID = "next"
			sink.agentFacts = &agentCommandFacts{}
			send := func(text string) {
				t.Helper()
				if err := sink.acceptInvocation(context.Background(), "new-invocation", runtimes.Event{Type: runtimes.EventTypeTextDelta, Data: marshalData(map[string]any{"text": text})}); err != nil {
					t.Fatalf("current text interrupted: %v", err)
				}
			}
			send("before")
			<-live
			op, _ := contract.Find(operation)
			result := agentcapability.Result{CommandID: "old-receipt", Replayed: scenario.replayed, ExecutionID: "execution-1", Data: map[string]any{"runId": "old-run", "definitionId": "definition", "revisionId": "revision", "name": "原版本", "attachments": []eventlog.AttachmentReference{{Name: "report.txt", Path: "attachments/report.txt", Managed: true}}}}
			if err := sink.agentFacts.record(context.Background(), sink.journal, chatID, "next", op, result); err != nil {
				t.Fatal(err)
			}
			send("after")
			snapshot := (<-live).Message
			if snapshot.Content != "beforeafter" || len(snapshot.WorkflowRuns) != 0 || len(snapshot.Attachments) != 0 {
				t.Fatalf("old fact leaked: %+v", snapshot)
			}
			if err := sink.Flush(); err != nil {
				t.Fatal(err)
			}
			records, err := store.ReadAll(chatID)
			if err != nil {
				t.Fatal(err)
			}
			messages, err := messageview.ReplayMessages(records)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, message := range messages {
				if message.ExecutionID == "execution-1" && len(message.WorkflowRuns) == 1 {
					found = true
					if operation == "workflow.runs.deliver" && len(message.Attachments) != 1 {
						t.Fatal("old attachment lost")
					}
				}
			}
			if !found {
				t.Fatal("original workflow reference lost")
			}
		})
	}
}

// failedFactJournal 只注入日志写入故障，其余事实仍由真实日志读写。
type failedFactJournal struct {
	// Journal 复用原事件日志与实时发布端口。
	chatservice.Journal
	// failures 是尚未触发的日志写入故障次数。
	failures int
}

func (j *failedFactJournal) AppendExecutionFacts(ctx context.Context, chatID string, inputs []eventlog.AppendInput) ([]eventlog.Record, error) {
	if j.failures > 0 {
		j.failures--
		return nil, status_error.ChatLogError
	}
	return j.Journal.AppendExecutionFacts(ctx, chatID, inputs)
}

// 调用者必须能从首次失败响应恢复原回执，连续登记故障不能伪报附件成功。
func TestRegistrationFailureGuidesOriginalMessageRecovery(t *testing.T) {
	store, err := eventlog.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatID := "75757575757575757575757575757575"
	if _, err := store.Create(chatID, activeChatInputs()); err != nil {
		t.Fatal(err)
	}
	sink, _ := newRuntimeEventSinkForTest(store, chatID)
	journal := &failedFactJournal{Journal: sink.journal, failures: 2}
	facts := &agentCommandFacts{}
	registry := agentcapability.New(nil)
	defer registry.Close()
	calls := 0
	result := agentcapability.Result{CommandID: "persisted-receipt", ExecutionID: "execution-1", Data: map[string]any{"runId": "run", "definitionId": "definition", "revisionId": "revision", "name": "报告", "attachments": []eventlog.AttachmentReference{{Name: "report.txt", Path: "attachments/report.txt", Managed: true}}}}
	grant, activate, revoke, err := registry.Prepare(agentcapability.GrantSpec{ExecutionID: "execution-1", Validate: func(context.Context) error { return nil }, Bindings: []agentcapability.Binding{{Operation: "workflow.runs.deliver", Call: func(context.Context, agentcapability.Request) (agentcapability.Result, error) {
		calls++
		return result, nil
	}, Receipt: func(context.Context, string) (agentcapability.Result, error) { return result, nil }}}, Record: func(ctx context.Context, op contract.Operation, value agentcapability.Result) error {
		return facts.record(ctx, journal, chatID, "execution-1", op, value)
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	activate()
	send := func(method, path, body string) agentcapability.Response {
		t.Helper()
		request := httptest.NewRequest(method, constants.AgentCLIPath+path, strings.NewReader(body))
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set("Authorization", "Bearer "+grant.Token)
		recorder := httptest.NewRecorder()
		registry.ServeHTTP(recorder, request)
		var response agentcapability.Response
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	response := send(http.MethodPost, "/commands", `{"operation":"workflow.runs.deliver","requestId":"original-delivery","params":{"runId":"run"}}`)
	for range 2 {
		if response.OK || response.Error == nil || response.Error.RetryAdvice != "query_receipt" || response.Data != nil || response.CommandID != "persisted-receipt" {
			t.Fatalf("caller cannot recover registration: %+v", response)
		}
		records, err := store.ReadAll(chatID)
		if err != nil {
			t.Fatal(err)
		}
		messages, err := messageview.ReplayMessages(records)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range messages {
			if len(message.Attachments) > 0 {
				t.Fatal("failed write reported attachment")
			}
		}
		next, ok := response.Error.Details["nextStep"].(map[string]any)
		if !ok {
			t.Fatal("missing receipt instruction")
		}
		response = send(http.MethodGet, "/receipts?operation="+next["operation"].(string)+"&requestId="+next["requestId"].(string), "")
	}
	if !response.OK {
		t.Fatalf("recovery failed: %+v", response)
	}
	send(http.MethodGet, "/receipts?operation=workflow.runs.deliver&requestId=original-delivery", "")
	records, err := store.ReadAll(chatID)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := messageview.ReplayMessages(records)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, message := range messages {
		if len(message.Attachments) > 0 {
			count++
			if message.ExecutionID != "execution-1" || len(message.Attachments) != 1 {
				t.Fatalf("incorrect original delivery: %+v", message)
			}
		}
	}
	if count != 1 || calls != 1 {
		t.Fatalf("duplicate delivery/run: attachments=%d calls=%d", count, calls)
	}
}
