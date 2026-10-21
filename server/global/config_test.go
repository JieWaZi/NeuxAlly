package global

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcommand"
	"github.com/JieWaZi/NeuxAlly/server/internal/builtinrolepreset"
	builtinskills "github.com/JieWaZi/NeuxAlly/server/internal/skills/builtin"
	workflowdsl "github.com/JieWaZi/NeuxAlly/server/internal/workflow/dsl"
	workflowstate "github.com/JieWaZi/NeuxAlly/server/internal/workflow/state"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/NeuxAlly/server/services/workflow"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func TestLoadSystemSkillStaysOutsideEditablePartnerCatalog(t *testing.T) {
	database, err := dbhelper.NewDB(conf.DB{
		Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "builtin-skills.db"), Migrate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	previousSkills, previousTransactions := SkillControllerManager, TransactionControllerManager
	t.Cleanup(func() {
		SkillControllerManager, TransactionControllerManager = previousSkills, previousTransactions
	})
	SkillControllerManager = controllers.NewSkillController(database.DB)
	TransactionControllerManager = controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB))
	configuration := NewConfiguration()
	configuration.DataDirectory = t.TempDir()
	ctx := context.Background()
	if err := loadBuiltinSkills(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	enabled, err := SkillControllerManager.ListEnabled(ctx, builtinrolepreset.PersonalAssistantWakerID)
	if err != nil || len(enabled) != 0 {
		t.Fatalf("system skill leaked into editable partner state: %+v %v", enabled, err)
	}
	directory, err := builtinskills.SystemDirectory(configuration.DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(filepath.Join(directory, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

}

func TestPrepareKeepsSQLiteDefaultsAndCreatesToken(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ALLY_DATA_DIR", dataDir)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ALLY_MANAGED_FILES_DIR", "")
	configuration := NewConfiguration()
	if err := Prepare(configuration); err != nil {
		t.Fatalf("prepare configuration: %v", err)
	}
	if configuration.DB.Type != conf.Sqlite {
		t.Fatalf("database type = %q, want sqlite", configuration.DB.Type)
	}
	if configuration.DB.DBName != filepath.Join(dataDir, "ally.db") {
		t.Fatalf("database path = %q", configuration.DB.DBName)
	}
	if configuration.DataDirectory != dataDir || configuration.ManagedFilesDirectory != filepath.Join(home, constants.ManagedFilesDirectoryName) {
		t.Fatalf("storage roots were not separated: data=%q managed=%q", configuration.DataDirectory, configuration.ManagedFilesDirectory)
	}
	info, err := os.Stat(configuration.ManagedFilesDirectory)
	if err != nil || !info.IsDir() {
		t.Fatalf("managed root was not prepared: %v", err)
	}
	if configuration.DB.MaxOpenConns != 0 || configuration.DB.MaxIdleConns != 0 {
		t.Fatalf(
			"connection pool should remain unset: open=%d idle=%d",
			configuration.DB.MaxOpenConns,
			configuration.DB.MaxIdleConns,
		)
	}
	if configuration.Security.LocalToken == "" {
		t.Fatal("local token was not generated")
	}
}

func TestPrepareManagedFilesOverrideKeepsDatabaseLocation(t *testing.T) {
	dataDir, managedDir := t.TempDir(), filepath.Join(t.TempDir(), "work-files")
	t.Setenv("ALLY_DATA_DIR", dataDir)
	t.Setenv("ALLY_MANAGED_FILES_DIR", managedDir)
	configuration := NewConfiguration()
	if err := Prepare(configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.ManagedFilesDirectory != managedDir || configuration.DataDirectory != dataDir || configuration.DB.DBName != filepath.Join(dataDir, "ally.db") {
		t.Fatalf("managed override changed database root: %+v", configuration)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "chat"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLY_MANAGED_FILES_DIR", filepath.Join(managedDir, "chat", "nested"))
	if err := Prepare(NewConfiguration()); err == nil {
		t.Fatal("unusable managed root was accepted")
	}
}

func TestPrepareRejectsNonLoopbackHost(t *testing.T) {
	configuration := NewConfiguration()
	configuration.Server.Host = "0.0.0.0"
	if err := Prepare(configuration); err == nil {
		t.Fatal("expected non-loopback host to be rejected")
	}
}

func TestRunLoadSequenceRollsBackOnceAfterFailure(t *testing.T) {
	wantFailure := errors.New("load failed")
	executed := []string{}
	rollbackCount := 0
	err := runLoadSequence(
		func() { rollbackCount++ },
		func() error {
			executed = append(executed, "database")
			return nil
		},
		func() error {
			executed = append(executed, "memory")
			return wantFailure
		},
		func() error {
			executed = append(executed, "runtime")
			return nil
		},
	)
	if !errors.Is(err, wantFailure) {
		t.Fatalf("load error = %v, want %v", err, wantFailure)
	}
	if rollbackCount != 1 {
		t.Fatalf("rollback count = %d, want 1", rollbackCount)
	}
	if len(executed) != 2 || executed[0] != "database" || executed[1] != "memory" {
		t.Fatalf("executed steps = %v", executed)
	}
}

func TestRunLoadSequenceDoesNotRollBackAfterSuccess(t *testing.T) {
	rollbackCount := 0
	err := runLoadSequence(
		func() { rollbackCount++ },
		func() error { return nil },
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("run load sequence: %v", err)
	}
	if rollbackCount != 0 {
		t.Fatalf("rollback count = %d, want 0", rollbackCount)
	}
}

// legacyConsumerChat 仅替代当前聊天执行读取，旧运行、回执与初始化均使用真实临时 SQLite。
type legacyConsumerChat struct {
	// Module 保留未使用的聊天接口，意外调用立即失败。
	chat.Module
	// chatID 是当前已授权聊天身份。
	chatID string
}

func (c *legacyConsumerChat) ReadExecution(_ context.Context, chatID, executionID string) (models.Execution, error) {
	if chatID != c.chatID || executionID != "next" {
		return models.Execution{}, status_error.ExecutionNotFound
	}
	return models.Execution{ID: "next", ChatID: c.chatID, Status: constants.ExecutionStatusRunning}, nil
}
func (c *legacyConsumerChat) GetChatSummary(_ context.Context, chatID string) (chat.ChatSummary, error) {
	if chatID != c.chatID {
		return chat.ChatSummary{}, status_error.ChatNotFound
	}
	return chat.ChatSummary{Chat: models.Chat{ID: c.chatID, ActiveExecutionID: "next"}}, nil
}

// 旧表没有新关联列与取消原因，初始化必须搬移旧 Context 事实，且不能从可变名称补回执。
func TestLoadDBMigratesLegacyConsumerRunsAndReceipts(t *testing.T) {
	ctx := context.Background()
	configuration := NewConfiguration()
	configuration.DataDirectory = t.TempDir()
	configuration.DB = conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "legacy.db")}
	database, err := dbhelper.NewDB(configuration.DB)
	if err != nil {
		t.Fatal(err)
	}
	db := database.DB
	// 与基线 WorkflowRun 相同的列形态；明确不使用新模型先建表再填空列。
	legacySchema := `CREATE TABLE workflow_runs (worker_configurations text NOT NULL,root_run_id text,id varchar(32),definition_id varchar(32) NOT NULL,revision_id varchar(32) NOT NULL,revision_ordinal integer,status varchar(32) NOT NULL,args text,defaults text NOT NULL,limits text,scope_binding text NOT NULL,context text NOT NULL,root_fiber_id varchar(32) NOT NULL,parent_run_id varchar(32) NOT NULL DEFAULT '',joined numeric,depth integer,result text,failure text,version integer NOT NULL,lease_owner varchar(32) NOT NULL DEFAULT '',fence integer NOT NULL,lease_until integer NOT NULL,started_at integer,deadline_at integer,completed_at integer,history_deleted_at integer,created_at integer,updated_at integer,PRIMARY KEY(id))`
	if err := db.Exec(legacySchema).Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&models.WorkflowRun{}, "consumer_lease_id") {
		t.Fatal("fixture already has new column")
	}
	if err := db.AutoMigrate(&models.WorkflowConsumerLease{}, &models.WorkflowDefinition{}, &models.WorkflowRevision{}, &models.AgentCommandReceipt{}); err != nil {
		t.Fatal(err)
	}
	lease := models.WorkflowConsumerLease{ID: "original", ChatID: "chat", DefinitionIDs: []string{"definition"}, AuthToken: "original-token", DeadlineAt: 1}
	if err := db.Create(&lease).Error; err != nil {
		t.Fatal(err)
	}
	next := lease
	next.ID = "next"
	next.AuthToken = "next-token"
	next.DeadlineAt = time.Now().Add(time.Hour).UnixMilli()
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	invalid := lease
	invalid.ID = "wrong-definition"
	invalid.DefinitionIDs = []string{"another-definition"}
	if err := db.Create(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	var manifest workflowdsl.Manifest
	if err := json.Unmarshal([]byte(`{"meta":{"title":"原不可变标题","outputSchema":{"type":"object"}}}`), &manifest); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.WorkflowDefinition{ID: "definition", Title: "已改当前名称", CurrentRevisionID: "revision"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.WorkflowRevision{ID: "revision", DefinitionID: "definition", Manifest: manifest}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 260; i++ {
		id := fmt.Sprintf("run-%03d", i)
		origin := "original"
		if i == 257 {
			origin = "missing"
		}
		if i == 258 {
			origin = "wrong-definition"
		}
		flowContext := map[string]any{"runId": id, "workflowId": "definition", "revisionId": "revision", "triggerSource": "agent", "consumerLeaseId": origin, "largeInteger": json.RawMessage(`9007199254740993`), "unknown": json.RawMessage(`{"exact":18446744073709551615,"nested":[1,null,"keep"]}`)}
		if i == 259 {
			flowContext["workflowId"] = "forged"
		}
		run := models.WorkflowRun{ID: id, DefinitionID: "definition", RevisionID: "revision", Context: flowContext, Status: constants.WorkflowRunStatusQueued, Defaults: models.WorkflowExecutionDefaults{}, WorkerConfigurations: map[string]models.WorkflowExecutionDefaults{}, RootFiberID: "fiber", Version: 1}
		if i == 0 {
			run.Status = constants.WorkflowRunStatusSucceeded
			run.Result = map[string]any{}
		}
		if err := db.Omit("ConsumerLeaseID", "CancellationReason").Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	commandID := agentcommand.ID("workflow.consumer:chat", "workflow.runs.start", "old-start")
	if err := db.Create(&models.AgentCommandReceipt{ID: commandID, Digest: "original-digest", Data: map[string]any{"runId": "run-000", "definitionId": "definition", "revisionId": "revision"}}).Error; err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Error(err)
		}
	})
	for initialization := 0; initialization < 2; initialization++ {
		if err := loadDB(ctx, configuration); err != nil {
			t.Fatal(err)
		}
		migrated, err := WorkflowRunControllerManager.GetRun(ctx, "run-000")
		if err != nil {
			t.Fatal(err)
		}
		if migrated.ConsumerLeaseID != "original" {
			t.Fatalf("legacy run lost consumer after initialization: %+v", migrated)
		}
		var raw string
		if err := DB.Model(&models.WorkflowRun{}).Select("context").Where("id = ?", "run-000").Scan(&raw).Error; err != nil {
			t.Fatal(err)
		}
		var exact map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &exact); err != nil {
			t.Fatal(err)
		}
		if string(exact["largeInteger"]) != "9007199254740993" || string(exact["unknown"]) != `{"exact":18446744073709551615,"nested":[1,null,"keep"]}` {
			t.Fatalf("migration changed unrelated JSON: %s", raw)
		}
		if _, exists := migrated.Context["consumerLeaseId"]; exists {
			t.Fatal("legacy context retained second authoritative association")
		}
		for _, id := range []string{"run-257", "run-258", "run-259"} {
			run, err := WorkflowRunControllerManager.GetRun(ctx, id)
			if err != nil || run.ConsumerLeaseID != "" {
				t.Fatalf("invalid original ownership migrated: %+v %v", run, err)
			}
		}
		if initialization == 0 {
			if err := Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	consumers := controllers.NewWorkflowConsumerController(DB)
	commands := controllers.NewAgentCommandController(DB)
	chats := &legacyConsumerChat{chatID: "chat"}
	service := workflow.New(workflow.Config{Store: WorkflowControllerManager, Runs: WorkflowRunControllerManager, Consumers: consumers, Commands: commands, Transactions: TransactionControllerManager, Chats: chats, State: workflowstate.New(WorkflowControllerManager, WorkflowRunControllerManager, TransactionControllerManager)})
	bindings := map[string]agentcapability.Binding{}
	for _, binding := range service.AgentBindings("next", "next-token", constants.WorkflowCapabilityScopeConsumer) {
		bindings[binding.Operation] = binding
	}
	result, err := bindings["workflow.runs.get"].Call(ctx, agentcapability.Request{Operation: "workflow.runs.get", Params: map[string]any{"runId": "run-000"}})
	if err != nil {
		t.Fatalf("same-chat authorized legacy read failed: %v", err)
	}
	if result.Data.(map[string]any)["run"].(models.WorkflowRun).ID != "run-000" {
		t.Fatal("wrong run")
	}
	receipt, err := bindings["workflow.runs.start"].Receipt(ctx, "old-start")
	if err != nil || receipt.CommandID != commandID || receipt.ExecutionID != "original" || receipt.Data.(map[string]any)["name"] != "原不可变标题" {
		t.Fatalf("old receipt lost trusted immutable reference: %+v %v", receipt, err)
	}
	stored, found, err := commands.Find(ctx, commandID)
	if err != nil || !found || stored.Data["name"] != nil {
		t.Fatal("recovery rewrote immutable original receipt")
	}
	// 新回合权限仍不能跨聊天，旧消费者过期不应阻断同聊天历史读取。
	if err := DB.Model(&models.WorkflowConsumerLease{}).Where("id = ?", "next").Update("chat_id", "other-chat").Error; err != nil {
		t.Fatal(err)
	}
	chats.chatID = "other-chat"
	if _, err := bindings["workflow.runs.get"].Call(ctx, agentcapability.Request{Operation: "workflow.runs.get", Params: map[string]any{"runId": "run-000"}}); !errors.Is(err, status_error.WorkflowNotFound) {
		t.Fatalf("foreign chat read legacy run: %v", err)
	}
	if _, err := bindings["workflow.runs.start"].Receipt(ctx, "old-start"); !errors.Is(err, status_error.AgentReceiptNotFound) {
		t.Fatalf("foreign chat recovered receipt: %v", err)
	}
	if err := service.CancelConsumerExecution(ctx, "chat", "original"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 257; i++ {
		run, err := WorkflowRunControllerManager.GetRun(ctx, fmt.Sprintf("run-%03d", i))
		if err != nil || run.Status != constants.WorkflowRunStatusCancelling {
			t.Fatalf("linked cancel missed migrated run %d: %+v %v", i, run, err)
		}
	}
}
