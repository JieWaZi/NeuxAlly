package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

func TestLiveSummaryRecoversUnpublishedNativeQuestionWithoutCancellingExecution(t *testing.T) {
	ctx := context.Background()
	_, manager := newRecoveryTestService(t, t.TempDir())
	chatID := "91919191919191919191919191919192"
	executionID := "92929292929292929292929292929293"
	workspace := t.TempDir()
	if _, err := manager.CreateDormantChats(ctx, []DormantChat{{Chat: models.Chat{ID: chatID, WakerID: "worker", Title: "实际提问", WorkspaceDirectory: workspace, AdditionalWorkingDirectories: []string{workspace}}}}); err != nil {
		t.Fatal(err)
	}
	records, err := manager.eventCommitter.store.Append(chatID, []eventlog.AppendInput{
		{Type: constants.ChatEventTypeUserMessageCreated, MessageID: "message", Data: recoveryTestData(t, eventlog.MessageCreatedData{Role: constants.MessageRoleUser, Actor: "workflow", Content: "虚构材料"})},
		{Type: constants.ChatEventTypeExecutionCreated, ExecutionID: executionID, Data: recoveryTestData(t, eventlog.ExecutionCreatedData{UserMessageID: "message", RuntimeID: "codex"})},
		{Type: constants.ChatEventTypeElicitationRequested, ExecutionID: executionID, Data: recoveryTestData(t, eventlog.ElicitationRequestedData{ElicitationID: "pending", Mode: "form", Message: "允许提交节点结果？", RequestedSchema: recoveryTestData(t, map[string]any{"type": "object", "properties": map[string]any{}})})},
	})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := manager.GetChatSummary(ctx, chatID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Chat.ActiveExecutionID != executionID || !summary.Chat.WaitingElicitation || len(summary.Chat.PendingElicitations) != 1 || summary.Chat.LatestSequence != records[len(records)-1].Sequence {
		t.Fatalf("live native question lost or running execution cancelled: %+v", summary.Chat)
	}
	again, err := manager.GetChatSummary(ctx, chatID)
	if err != nil || len(again.Chat.PendingElicitations) != 1 {
		t.Fatalf("repeated summary duplicated the request: %+v %v", again, err)
	}
}

// liveProjectionRaceTransaction 在查询写回前让另一个提交者推进真实 SQLite 游标。
type liveProjectionRaceTransaction struct {
	// TransactionController 执行原有数据库事务，保留真实条件更新与回滚行为。
	controllers.TransactionController
	// beforeTransaction 控制后台提交与查询写回的确定性交错。
	beforeTransaction func(context.Context) error
}

// Transaction 先完成竞争提交，再执行查询原本的投影事务。
func (c liveProjectionRaceTransaction) Transaction(ctx context.Context, fn func(context.Context) error) error {
	if err := c.beforeTransaction(ctx); err != nil {
		return err
	}
	return c.TransactionController.Transaction(ctx, fn)
}

func TestLiveSummarySucceedsDuringProjectionRaces(t *testing.T) {
	for _, scenario := range []struct {
		// name 区分持续追加与逐条追平两种真实提交交错。
		name string
		// appendAfterAdvance 表示后台追平后继续追加下一条权威事件。
		appendAfterAdvance bool
		// stopAfterFirst 表示只抢先提交一个事件，剩余事件必须由查询补齐。
		stopAfterFirst bool
	}{
		{name: "writer_keeps_appending", appendAfterAdvance: true},
		{name: "writer_advances_one_event_at_a_time"},
		{name: "query_completes_remaining_tail", stopAfterFirst: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			_, manager := newRecoveryTestService(t, t.TempDir())
			chatID := "93939393939393939393939393939393"
			workspace := t.TempDir()
			if _, err := manager.CreateDormantChats(ctx, []DormantChat{{Chat: models.Chat{
				ID: chatID, WakerID: "worker", Title: "before", WorkspaceDirectory: workspace,
				AdditionalWorkingDirectories: []string{workspace},
			}}}); err != nil {
				t.Fatal(err)
			}
			var target eventlog.Record
			for index := 0; index < 8; index++ {
				records, err := manager.logStore.Append(chatID, []eventlog.AppendInput{{
					Type: constants.ChatEventTypeTitleUpdated,
					Data: recoveryTestData(t, eventlog.ChatTitleUpdatedData{Title: fmt.Sprintf("title-%d", index)}),
				}})
				if err != nil {
					t.Fatal(err)
				}
				target = records[len(records)-1]
			}
			writes := 0
			manager.queryIndexes.transactionController = liveProjectionRaceTransaction{
				TransactionController: manager.queryIndexes.transactionController,
				beforeTransaction: func(ctx context.Context) error {
					if scenario.stopAfterFirst && writes > 0 {
						return nil
					}
					indexed, err := manager.queryIndexes.chatController.Get(ctx, chatID)
					if err != nil {
						return err
					}
					limit := 1
					if scenario.appendAfterAdvance {
						limit = 0
					}
					tail, err := manager.logStore.ReadAfter(chatID, indexed.LatestSequence, limit)
					if err != nil || len(tail) == 0 {
						return err
					}
					last := tail[len(tail)-1]
					var title eventlog.ChatTitleUpdatedData
					if err := json.Unmarshal(last.Data, &title); err != nil {
						return err
					}
					if err := manager.queryIndexes.chatController.AdvanceProjection(ctx, chatID, indexed.LatestSequence, map[string]any{
						"latest_sequence": last.Sequence, "title": title.Title, "updated_at": last.RecordedAt,
					}); err != nil {
						return err
					}
					writes++
					if scenario.appendAfterAdvance {
						_, err = manager.logStore.Append(chatID, []eventlog.AppendInput{{
							Type: constants.ChatEventTypeTitleUpdated,
							Data: recoveryTestData(t, eventlog.ChatTitleUpdatedData{Title: "next-title"}),
						}})
					}
					return err
				},
			}
			readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			summary, err := manager.GetChatSummary(readCtx, chatID)
			if err != nil {
				t.Fatalf("GetChatSummary during concurrent commits returned %v, want a readable chat", err)
			}
			if summary.Chat.LatestSequence < target.Sequence || summary.Chat.Title != "title-7" {
				t.Fatalf("summary lost captured authority: %+v, target sequence %d", summary.Chat, target.Sequence)
			}
			if (scenario.appendAfterAdvance || scenario.stopAfterFirst) && writes != 1 {
				t.Fatalf("query chased new writes %d times instead of completing its captured tail", writes)
			}
			if !scenario.appendAfterAdvance && !scenario.stopAfterFirst && writes != 8 {
				t.Fatalf("expected eight competing cursor advances, got %d", writes)
			}
		})
	}
}

func TestLiveSummaryPreservesProjectionFailureAndCancellation(t *testing.T) {
	for _, failure := range []status_error.StatusError{status_error.DatabaseError, status_error.OperationCanceled} {
		t.Run(fmt.Sprint(failure.Code()), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, manager := newRecoveryTestService(t, t.TempDir())
			chatID := "94949494949494949494949494949494"
			workspace := t.TempDir()
			if _, err := manager.CreateDormantChats(ctx, []DormantChat{{Chat: models.Chat{
				ID: chatID, WakerID: "worker", Title: "before", WorkspaceDirectory: workspace,
				AdditionalWorkingDirectories: []string{workspace},
			}}}); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.logStore.Append(chatID, []eventlog.AppendInput{{
				Type: constants.ChatEventTypeTitleUpdated,
				Data: recoveryTestData(t, eventlog.ChatTitleUpdatedData{Title: "after"}),
			}}); err != nil {
				t.Fatal(err)
			}
			manager.queryIndexes.transactionController = liveProjectionRaceTransaction{
				TransactionController: manager.queryIndexes.transactionController,
				beforeTransaction: func(context.Context) error {
					if failure == status_error.OperationCanceled {
						cancel()
						return status_error.Conflict
					}
					return failure
				},
			}
			if _, err := manager.GetChatSummary(ctx, chatID); !hasStatusErrorCode(err, failure.Code()) {
				t.Fatalf("GetChatSummary() error = %v, want %v", err, failure)
			}
		})
	}
}

// 迟到交付经过实际 Journal、JSONL 重读和历史索引后，原消息保留附件且新回合继续活跃。
func TestCommandFactsAfterTerminalRecoverOriginalMessage(t *testing.T) {
	ctx := context.Background()
	_, manager := newRecoveryTestService(t, t.TempDir())
	chatID := "83838383838383838383838383838383"
	directory := t.TempDir()
	if _, err := manager.CreateDormantChats(ctx, []DormantChat{{Chat: models.Chat{ID: chatID, WakerID: "worker", WorkspaceDirectory: directory, AdditionalWorkingDirectories: []string{directory}, Title: "交付恢复"}}}); err != nil {
		t.Fatal(err)
	}
	inputs := []eventlog.AppendInput{
		{Type: constants.ChatEventTypeUserMessageCreated, MessageID: "u1", Data: recoveryTestData(t, eventlog.MessageCreatedData{Role: constants.MessageRoleUser, Content: "生成报告"})},
		{Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "old", Data: recoveryTestData(t, eventlog.ExecutionCreatedData{UserMessageID: "u1", RuntimeID: "codex"})},
		{Type: constants.ChatEventTypeExecutionCompleted, ExecutionID: "old", Data: recoveryTestData(t, eventlog.ExecutionFinishedData{Status: constants.ExecutionStatusCompleted})},
		{Type: constants.ChatEventTypeUserMessageCreated, MessageID: "u2", Data: recoveryTestData(t, eventlog.MessageCreatedData{Role: constants.MessageRoleUser, Content: "继续其他任务"})},
		{Type: constants.ChatEventTypeExecutionCreated, ExecutionID: "next", Data: recoveryTestData(t, eventlog.ExecutionCreatedData{UserMessageID: "u2", RuntimeID: "codex"})},
	}
	if _, err := manager.eventCommitter.store.Append(chatID, inputs); err != nil {
		t.Fatal(err)
	}
	fact := eventlog.AppendInput{Type: constants.ChatEventTypeAgentCommandCommitted, ExecutionID: "old", Data: recoveryTestData(t, eventlog.AgentCommandData{CommandID: "delivery", Operation: "workflow.runs.deliver", Completion: "immediate", Attachments: []eventlog.AttachmentReference{{Name: "report.txt", Path: "attachments/report/report.txt", Managed: true}}})}
	for range 2 {
		if _, err := manager.AppendExecutionFacts(ctx, chatID, []eventlog.AppendInput{fact}); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := manager.GetChatSummary(ctx, chatID)
	if err != nil || summary.Chat.ActiveExecutionID != "next" {
		t.Fatalf("迟到事实重新激活旧执行: %+v %v", summary, err)
	}
	history, err := manager.ReadMessageHistory(ctx, chatID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	attached := 0
	for _, message := range history.Resources {
		if len(message.Attachments) > 0 {
			attached++
			if message.ExecutionID != "old" || len(message.Attachments) != 1 || message.Status != constants.ExecutionStatusCompleted {
				t.Fatalf("附件归属或终态被污染: %+v", message)
			}
		}
	}
	if attached != 1 {
		t.Fatalf("历史查询丢失或重复交付: %+v", history.Resources)
	}
}
