package chats

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	workflowstate "github.com/JieWaZi/NeuxAlly/server/internal/workflow/state"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/NeuxAlly/server/services/execution"
	"github.com/JieWaZi/NeuxAlly/server/services/workflow"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

// cancelChatBoundary 提供原执行和下一轮的权威身份。
type cancelChatBoundary struct {
	// Module 保留无关聊天端口。
	chat.Module
	// active 是当前仍运行的执行。
	active string
}

func (b *cancelChatBoundary) GetChatSummary(context.Context, string) (chat.ChatSummary, error) {
	return chat.ChatSummary{Chat: models.Chat{ID: "chat", ActiveExecutionID: b.active}}, nil
}
func (b *cancelChatBoundary) ReadExecution(_ context.Context, chatID, id string) (models.Execution, error) {
	if chatID != "chat" || (id != "old" && id != "new" && id != "ordinary") {
		return models.Execution{}, status_error.ExecutionNotFound
	}
	status := constants.ExecutionStatusCanceled
	if b.active == id {
		status = constants.ExecutionStatusRunning
	}
	return models.Execution{ID: id, ChatID: chatID, Status: status}, nil
}

// cancelExecutionBoundary 模拟运行注册表按身份取消，终态旧执行不会命中新执行。
type cancelExecutionBoundary struct {
	// Service 保留无关执行端口。
	execution.Service
	// chats 是共享当前执行事实。
	chats *cancelChatBoundary
	// stopped 保存真正命中的执行。
	stopped []string
}

func (b *cancelExecutionBoundary) CancelExecution(_ string, id string) error {
	if b.chats.active == id {
		b.stopped = append(b.stopped, id)
		b.chats.active = ""
	}
	return nil
}

// cancelRunFailure 在真实运行读取处注入一次可恢复的数据库错误。
type cancelRunFailure struct {
	// WorkflowRunController 复用真实 SQLite 查询和事务。
	controllers.WorkflowRunController
	// fail 控制原运行的故障窗口。
	fail bool
}

func (r *cancelRunFailure) GetRun(ctx context.Context, id string) (models.WorkflowRun, error) {
	if r.fail && id == "old-run" {
		return models.WorkflowRun{}, status_error.DatabaseError
	}
	return r.WorkflowRunController.GetRun(ctx, id)
}

func TestCancelChatExecutionRetriesOriginalIdentityAfterNextTurn(t *testing.T) {
	ctx := context.Background()
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "cancel.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	runs := controllers.NewWorkflowRunController(database.DB)
	fault := &cancelRunFailure{WorkflowRunController: runs, fail: true}
	consumers := controllers.NewWorkflowConsumerController(database.DB)
	store := controllers.NewWorkflowController(database.DB)
	transactions := controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB))
	originalChat, originalExecution, originalWorkflow := global.ChatService, global.ExecutionService, global.WorkflowService
	t.Cleanup(func() {
		global.ChatService, global.ExecutionService, global.WorkflowService = originalChat, originalExecution, originalWorkflow
	})
	chats := &cancelChatBoundary{active: "old"}
	executions := &cancelExecutionBoundary{chats: chats}
	global.ChatService = chats
	global.ExecutionService = executions
	global.WorkflowService = workflow.New(workflow.Config{Consumers: consumers, Runs: fault, Store: store, Transactions: transactions, State: workflowstate.New(store, fault, transactions)})
	create := func(id string) {
		t.Helper()
		if err := consumers.CreateConsumer(ctx, models.WorkflowConsumerLease{ID: id, ChatID: "chat", DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}); err != nil {
			t.Fatal(err)
		}
		if err := runs.CreateRun(ctx, models.WorkflowRun{ID: id + "-run", ConsumerLeaseID: id, Status: constants.WorkflowRunStatusQueued, Version: 1}, models.WorkflowFiber{ID: id + "-fiber", RunID: id + "-run", Status: constants.WorkflowStepStatusReady}); err != nil {
			t.Fatal(err)
		}
	}
	create("old")
	var request CancelChatExecution
	if err := json.Unmarshal([]byte(`{"ChatID":"chat","ExecutionID":"old"}`), &request); err != nil {
		t.Fatal(err)
	}
	if _, err := request.Output(&gin.Context{}); !errors.Is(err, status_error.DatabaseError) {
		t.Fatalf("传播故障未报告: %v", err)
	}
	if len(executions.stopped) != 1 || executions.stopped[0] != "old" {
		t.Fatalf("传播失败仍须停止原智能体: %v", executions.stopped)
	}
	create("new")
	chats.active = "new"
	fault.fail = false
	if _, err := request.Output(&gin.Context{}); err != nil {
		t.Fatal(err)
	}
	oldRun, _ := runs.GetRun(ctx, "old-run")
	newRun, _ := runs.GetRun(ctx, "new-run")
	if oldRun.Status != constants.WorkflowRunStatusCancelling || newRun.Status != constants.WorkflowRunStatusQueued || chats.active != "new" || len(executions.stopped) != 1 {
		t.Fatalf("原请求重试串入下一轮: old=%s new=%s active=%s stopped=%v", oldRun.Status, newRun.Status, chats.active, executions.stopped)
	}
	// 错误归属不得撤销授权或停止智能体。
	request.ChatID = "other"
	if _, err := request.Output(&gin.Context{}); err == nil {
		t.Fatal("跨聊天执行身份未拒绝")
	}
	request.ChatID = "chat"
	if err := json.Unmarshal([]byte(`{"ExecutionID":"ordinary"}`), &request); err != nil {
		t.Fatal(err)
	}
	chats.active = "ordinary"
	if _, err := request.Output(&gin.Context{}); err != nil || chats.active != "" {
		t.Fatalf("普通聊天停止失败: %v", err)
	}
}
