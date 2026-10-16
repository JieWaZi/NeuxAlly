package execution

import (
	"context"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	eventlog "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	eventstream "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventstream"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
)

// terminalJournal 记录执行终态的 durable 与公开两阶段交接。
type terminalJournal struct {
	// terminal 是最近一次先持久的执行终态。
	terminal chatservice.TerminalFact
	// published 在注册表交出执行占用后接收公开通知。
	published chan struct{}
}

// ForkChat 不参与终态交接测试。
func (j *terminalJournal) ForkChat(context.Context, chatservice.ForkInput, chatservice.NativeFork) (string, error) {
	panic("unexpected ForkChat call")
}

// LoadExecutionContext 不参与终态交接测试。
func (j *terminalJournal) LoadExecutionContext(context.Context, string) (chatservice.ExecutionContext, error) {
	panic("unexpected LoadExecutionContext call")
}

// CommitStartedTurn 不参与终态交接测试。
func (j *terminalJournal) CommitStartedTurn(context.Context, chatservice.StartedTurn) (chatservice.AcceptedTurn, error) {
	panic("unexpected CommitStartedTurn call")
}

// CommitAcceptedTurn 不参与终态交接测试。
func (j *terminalJournal) CommitAcceptedTurn(context.Context, chatservice.StartedTurn) (chatservice.AcceptedTurn, error) {
	panic("unexpected CommitAcceptedTurn call")
}

// MarkExecutionStarted 不参与终态交接测试。
func (j *terminalJournal) MarkExecutionStarted(context.Context, models.Execution, string) (models.Execution, error) {
	panic("unexpected MarkExecutionStarted call")
}

// ReadRecords 不参与终态交接测试。
func (j *terminalJournal) ReadRecords(string) ([]eventlog.Record, error) {
	panic("unexpected ReadRecords call")
}

// AppendExecutionFacts 不参与终态交接测试。
func (j *terminalJournal) AppendExecutionFacts(context.Context, string, []eventlog.AppendInput) ([]eventlog.Record, error) {
	panic("unexpected AppendExecutionFacts call")
}

// AppendApprovalFact 不参与终态交接测试。
func (j *terminalJournal) AppendApprovalFact(string, eventlog.AppendInput) (eventlog.Record, error) {
	panic("unexpected AppendApprovalFact call")
}

// AppendElicitationFact 不参与终态交接测试。
func (j *terminalJournal) AppendElicitationFact(string, eventlog.AppendInput) (eventlog.Record, error) {
	panic("unexpected AppendElicitationFact call")
}

// PublishApprovalFact 不参与终态交接测试。
func (j *terminalJournal) PublishApprovalFact(context.Context, eventlog.Record, []models.PendingApproval) {
	panic("unexpected PublishApprovalFact call")
}

// PublishElicitationFact 不参与终态交接测试。
func (j *terminalJournal) PublishElicitationFact(context.Context, eventlog.Record, []models.PendingElicitation) {
	panic("unexpected PublishElicitationFact call")
}

// CommitTerminal 记录已经 durable 但尚未公开的终态。
func (j *terminalJournal) CommitTerminal(_ context.Context, fact chatservice.TerminalFact) (chatservice.TerminalCommit, error) {
	j.terminal = fact
	return chatservice.TerminalCommit{}, nil
}

// PublishTerminal 在执行占用交出时公开终态。
func (j *terminalJournal) PublishTerminal(chatservice.TerminalCommit) {
	j.published <- struct{}{}
}

// PublishLive 不参与终态交接测试。
func (j *terminalJournal) PublishLive(string, eventstream.PublicEvent) {
	panic("unexpected PublishLive call")
}

func TestFinishExecutionReleasesReservationAndPublishesDurableTerminal(t *testing.T) {
	chatID := "0123456789abcdef0123456789abcdef"
	executionID := "11111111111111111111111111111111"
	registry := newExecutionRegistry()
	registry.start(context.Background())
	t.Cleanup(registry.close)
	journal := &terminalJournal{published: make(chan struct{}, 1)}
	executionCore := &service{journal: journal, executions: registry}
	if _, err := registry.reserve(chatID, executionID); err != nil {
		t.Fatalf("first reserve() error = %v", err)
	}

	executionCore.finishExecution(
		models.Chat{ID: chatID},
		models.Execution{ID: executionID, ChatID: chatID},
		constants.ExecutionStatusCompleted,
		"",
		"",
		0,
		"",
		nil,
		0,
		false,
	)
	select {
	case <-journal.published:
	default:
		t.Fatal("durable terminal was not published")
	}
	if journal.terminal.Status != constants.ExecutionStatusCompleted {
		t.Fatalf("terminal status = %q", journal.terminal.Status)
	}
	if _, err := registry.reserve(
		chatID,
		"22222222222222222222222222222222",
	); err != nil {
		t.Fatalf("reserve after visible terminal event error = %v", err)
	}
	registry.release(chatID, "22222222222222222222222222222222")
}
