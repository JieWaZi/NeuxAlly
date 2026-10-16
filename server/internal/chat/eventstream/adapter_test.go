package eventstream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	eventlog "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
)

func TestAdapterAppliesRecordsIncrementallyAndKeepsStableMessageRevision(t *testing.T) {
	t.Parallel()

	adapter := NewAdapter()
	records := fixtureRecords(t)
	for index, record := range records {
		event, err := adapter.Apply(record)
		if err != nil {
			t.Fatalf("Apply(record %d) error = %v", index, err)
		}
		if event.Sequence != record.Sequence {
			t.Fatalf("event.Sequence = %d, want %d", event.Sequence, record.Sequence)
		}
		if event.Message != nil && event.Message.Revision != record.Sequence {
			t.Fatalf("event.Message.Revision = %d, want %d", event.Message.Revision, record.Sequence)
		}
	}
	if events := adapter.EventsAfter(1, 1); len(events) != 1 ||
		events[0].Message == nil || events[0].Message.ExecutionID != "execution-1" {
		t.Fatalf("execution-created event = %#v, want assistant execution snapshot", events)
	}

	events := adapter.EventsAfter(2, 2)
	if len(events) != 2 || events[0].Sequence != 3 || events[1].Sequence != 4 {
		t.Fatalf("EventsAfter(2, 2) = %#v, want sequences 3 and 4", events)
	}
	if events[0].Message == nil || len(events[0].Message.DisplayParts) != 1 {
		t.Fatalf("tool-use message snapshot = %#v", events[0].Message)
	}
	partID := events[0].Message.DisplayParts[0].ID
	if partID == "" || events[1].Message == nil || events[1].Message.DisplayParts[0].ID != partID {
		t.Fatalf("tool part id did not remain stable across snapshots: %#v %#v", events[0].Message, events[1].Message)
	}
}

func TestAdapterPublicEventsExcludeRuntimePayloadAndTechnicalExitCode(t *testing.T) {
	t.Parallel()

	adapter, err := Replay(fixtureRecords(t))
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	encoded, err := json.Marshal(adapter.EventsAfter(0, 0))
	if err != nil {
		t.Fatalf("Marshal(events) error = %v", err)
	}
	payload := string(encoded)
	var envelopes []map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &envelopes); err != nil {
		t.Fatalf("Unmarshal(events) error = %v", err)
	}
	for index, envelope := range envelopes {
		for _, key := range []string{
			"chatId",
			"messageId",
			"executionId",
			"invocationId",
			"runtimeEventType",
		} {
			if _, exists := envelope[key]; exists {
				t.Fatalf("public event %d exposed redundant envelope field %q", index, key)
			}
		}
	}
	for _, forbidden := range []string{
		"private command output",
		"exitCode",
		"resultCount",
		"\"data\"",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("public events leaked %q in %s", forbidden, payload)
		}
	}
	for _, required := range []string{
		"\"type\":\"assistant.message\"",
		"\"toolStatus\":\"failed\"",
		"\"text\":\"最终正文\"",
	} {
		if !strings.Contains(payload, required) {
			t.Fatalf("public events missing %q in %s", required, payload)
		}
	}
}

func TestAdapterTerminalEventIncludesCompleteMessageSnapshot(t *testing.T) {
	t.Parallel()

	records := fixtureRecords(t)
	finished, err := eventlog.MarshalData(eventlog.ExecutionFinishedData{
		Status:       constants.ExecutionStatusFailed,
		ErrorCode:    "runtime_failed",
		ErrorMessage: "执行失败",
	})
	if err != nil {
		t.Fatalf("MarshalData(finished) error = %v", err)
	}
	records = append(records, eventlog.Record{
		SchemaVersion: 1,
		ChatID:        "chat-1",
		Sequence:      6,
		RecordedAt:    600,
		Type:          constants.ChatEventTypeExecutionFailed,
		ExecutionID:   "execution-1",
		Data:          finished,
	})

	adapter, err := Replay(records)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	events := adapter.EventsAfter(5, 0)
	if len(events) != 1 {
		t.Fatalf("terminal events = %d, want 1", len(events))
	}
	message := events[0].Message
	if message == nil ||
		message.ID != "execution-1" ||
		message.ExecutionID != "execution-1" ||
		message.Status != constants.ExecutionStatusFailed ||
		message.ErrorCode != "runtime_failed" ||
		message.ErrorMessage != "执行失败" {
		t.Fatalf("terminal message snapshot = %#v", message)
	}
}

func TestAdapterPublishesOnlySafeApprovalFields(t *testing.T) {
	t.Parallel()
	line := 0
	data, err := eventlog.MarshalData(eventlog.ApprovalRequestedData{
		ApprovalID: "approval-1",
		ToolCallID: "tool-1",
		Title:      "读取文件",
		Kind:       "read",
		Command:    "curl https://chatgpt.com/",
		Locations: []eventlog.ApprovalLocationData{{
			Path: "/workspace/main.go",
			Line: &line,
		}},
		Options: []eventlog.ApprovalOptionData{{
			ID: "allow-once", Name: "Allow once", Kind: "allow_once",
		}},
	})
	if err != nil {
		t.Fatalf("MarshalData(approval) error = %v", err)
	}
	event, err := NewAdapter().Apply(eventlog.Record{
		SchemaVersion: 1,
		ChatID:        "conversation-1",
		Sequence:      1,
		RecordedAt:    100,
		Type:          constants.ChatEventTypeApprovalRequested,
		ExecutionID:   "execution-1",
		Data:          data,
	})
	if err != nil || event.ApprovalRequest == nil ||
		event.ApprovalRequest.ApprovalID != "approval-1" ||
		event.ApprovalRequest.Command != "curl https://chatgpt.com/" ||
		event.ApprovalRequest.Locations[0].Line == nil ||
		*event.ApprovalRequest.Locations[0].Line != 0 {
		t.Fatalf("approval event = %#v, %v", event, err)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal(event) error = %v", err)
	}
	for _, forbidden := range []string{"rawInput", "requestId", "headers", "environment"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("approval event leaked %q in %s", forbidden, encoded)
		}
	}
}

func fixtureRecords(t *testing.T) []eventlog.Record {
	t.Helper()
	return []eventlog.Record{
		messageRecord(t, 1),
		executionRecord(t, 2),
		executionMessageRecord(t, 3, constants.ChatEventTypeAssistantMessage, "tool_use", map[string]any{
			"id":      "tool-command",
			"name":    "Bash",
			"command": "cat /workspace/project/secret.txt",
		}),
		executionMessageRecord(t, 4, constants.ChatEventTypeToolMessage, "tool_result", map[string]any{
			"tool_use_id": "tool-command",
			"exit_code":   7,
			"output":      "private command output",
		}),
		executionMessageRecord(t, 5, constants.ChatEventTypeAssistantMessage, "text_delta", map[string]any{"text": "最终正文"}),
	}
}

func messageRecord(t *testing.T, sequence int64) eventlog.Record {
	t.Helper()
	data, err := eventlog.MarshalData(eventlog.MessageCreatedData{Role: constants.MessageRoleUser})
	if err != nil {
		t.Fatalf("MarshalData(message) error = %v", err)
	}
	return eventlog.Record{
		SchemaVersion: 1,
		ChatID:        "chat-1",
		Sequence:      sequence,
		RecordedAt:    sequence * 100,
		Type:          constants.ChatEventTypeUserMessageCreated,
		MessageID:     "user-1",
		Data:          data,
	}
}

func executionRecord(t *testing.T, sequence int64) eventlog.Record {
	t.Helper()
	data, err := eventlog.MarshalData(eventlog.ExecutionCreatedData{
		UserMessageID: "user-1",
	})
	if err != nil {
		t.Fatalf("MarshalData(execution) error = %v", err)
	}
	return eventlog.Record{
		SchemaVersion: 1,
		ChatID:        "chat-1",
		Sequence:      sequence,
		RecordedAt:    sequence * 100,
		Type:          constants.ChatEventTypeExecutionCreated,
		MessageID:     "execution-1",
		ExecutionID:   "execution-1",
		Data:          data,
	}
}

func executionMessageRecord(
	t *testing.T,
	sequence int64,
	recordType constants.ChatEventType,
	contentType constants.RuntimeEventType,
	payload map[string]any,
) eventlog.Record {
	t.Helper()
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(payload) error = %v", err)
	}
	data, err := eventlog.MarshalData(eventlog.RuntimeMessageData{
		Content: []eventlog.RuntimeMessageContent{{
			Type: contentType,
			Data: rawPayload,
		}},
		MessageRevision: sequence,
	})
	if err != nil {
		t.Fatalf("MarshalData(runtime) error = %v", err)
	}
	return eventlog.Record{
		SchemaVersion: 1,
		ChatID:        "chat-1",
		Sequence:      sequence,
		RecordedAt:    sequence * 100,
		Type:          recordType,
		MessageID:     "execution-1",
		ExecutionID:   "execution-1",
		InvocationID:  "invocation-1",
		Data:          data,
	}
}
