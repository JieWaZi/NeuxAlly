package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcli"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcommand"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	"github.com/JieWaZi/NeuxAlly/server/internal/notification/events"
	"github.com/JieWaZi/NeuxAlly/server/internal/workspace"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
)

func TestAgentRunFreezesFilesFromCLICWDAndConflictsOnChangedBytes(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	s.dataDirectory = t.TempDir()
	source := `export const meta={sdk:'ally.workflow/1',title:'文件交付',description:'交付本次报告',inputSchema:{type:'object',properties:{files:{type:'array',title:'报告',description:'选择本次报告。',items:{type:'string',format:'file'}},note:{type:'string',title:'备注',description:'填写备注。'}},required:['files']},outputSchema:{type:'string',format:'file'},phases:[{id:'deliver',title:'交付',detail:'交付报告文件',canExit:true}]};phase('deliver');return args.files[0];`
	definition := savedTestWorkflow(t, s, source)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "子目录 with spaces")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{filepath.Join(root, "report.txt"): "wrong root file", filepath.Join(sub, "report.txt"): "actual child report"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lease := models.WorkflowConsumerLease{ID: "execution", ChatID: "chat", AuthToken: "consumer-token", DefinitionIDs: []string{definition.ID}, DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := s.consumers.CreateConsumer(ctx, lease); err != nil {
		t.Fatal(err)
	}
	s.executions = consumerExecutionBoundary{}
	s.chats = &consumerChatBoundary{actual: models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning, PermissionLevel: constants.PermissionLevelDefault}, summary: chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID, AdditionalWorkingDirectories: []string{root}}}}
	registry := agentcapability.New(nil)
	defer registry.Close()
	grant, activate, revoke, err := registry.Prepare(agentcapability.GrantSpec{ExecutionID: lease.ID, FileRoots: []string{root}, Bindings: s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer), Validate: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	activate()
	endpoint := httptest.NewServer(registry)
	defer endpoint.Close()
	connection, err := agentcapability.WriteConnection(root, "connection.json", agentcapability.Connection{ProtocolVersion: constants.AgentCLIProtocolVersion, Endpoint: endpoint.URL + constants.AgentCLIPath, CoreEpoch: grant.Epoch, GrantID: grant.ID, Token: grant.Token})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	call := func(arguments []string, params any) (int, agentcapability.Response) {
		t.Helper()
		var input, output, failure bytes.Buffer
		if params != nil {
			if err := json.NewEncoder(&input).Encode(params); err != nil {
				t.Fatal(err)
			}
		}
		code := agentcli.Execute(ctx, arguments, agentcli.IO{In: &input, Out: &output, Err: &failure, ContextFile: connection})
		body := output.Bytes()
		if code != 0 {
			body = failure.Bytes()
		}
		var response agentcapability.Response
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("invalid CLI response: %s %s %v", &output, &failure, err)
		}
		return code, response
	}
	arguments := []string{"workflow", "runs", "start", "--request-id", "report", "--input", "-"}
	params := map[string]any{"definitionId": definition.ID, "revisionId": definition.CurrentRevisionID, "args": map[string]any{"files": []string{"report.txt"}, "note": "report.txt"}}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("not authorized"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := map[string]any{"definitionId": definition.ID, "revisionId": definition.CurrentRevisionID, "args": map[string]any{"files": []string{"report.txt", outside}}}
	code, rejected := call([]string{"workflow", "runs", "start", "--request-id", "outside", "--input", "-"}, invalid)
	if code == 0 || rejected.Error == nil || rejected.Error.Code != status_error.WorkflowFileInvalid.Key() {
		t.Fatalf("input escaped the grant roots: %+v", rejected)
	}
	code, missing := call([]string{"receipts", "get", "--operation", "workflow.runs.start", "--request-id", "outside"}, nil)
	if code == 0 || missing.Error == nil || missing.Error.Code != status_error.AgentReceiptNotFound.Key() {
		t.Fatalf("rejected input produced a receipt: %+v", missing)
	}
	code, first := call(arguments, params)
	if code != 0 {
		t.Fatalf("file admission failed: %+v", first.Error)
	}
	runID, _ := first.Data.(map[string]any)["runId"].(string)
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	args := run.Args.(map[string]any)
	frozen := args["files"].([]any)[0].(string)
	if body, err := os.ReadFile(frozen); err != nil || string(body) != "actual child report" || args["note"] != "report.txt" {
		t.Fatalf("incorrect input resolution: args=%v body=%q err=%v", args, body, err)
	}
	code, replay := call(arguments, params)
	if code != 0 || !replay.Replayed || replay.CommandID != first.CommandID {
		t.Fatalf("unchanged files did not replay: %+v", replay)
	}
	currentArguments := []string{"workflow", "runs", "start", "--request-id", "current", "--input", "-"}
	currentParams := map[string]any{"definitionId": definition.ID, "revisionId": "", "args": params["args"]}
	code, current := call(currentArguments, currentParams)
	if code != 0 {
		t.Fatalf("current revision admission failed: %+v", current)
	}
	changedSource := strings.Replace(source, "交付本次报告", "交付更新后的报告", 1)
	saved, _, err := s.SaveSource(ctx, SaveSourceCommand{DefinitionID: definition.ID, ExpectedRevisionID: definition.CurrentRevisionID, RequestID: "new-head", Source: changedSource, SDKVersion: constants.WorkflowSDKVersion})
	if err != nil || saved.RevisionID == definition.CurrentRevisionID {
		t.Fatalf("new revision was not created: %+v %v", saved, err)
	}
	code, currentReplay := call(currentArguments, currentParams)
	if code != 0 || !currentReplay.Replayed || currentReplay.CommandID != current.CommandID || currentReplay.Data.(map[string]any)["revisionId"] != definition.CurrentRevisionID {
		t.Fatalf("current revision replay followed a new head: %+v", currentReplay)
	}
	if err := os.WriteFile("report.txt", []byte("changed report"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, conflict := call(arguments, params)
	if code == 0 || conflict.Error == nil || conflict.Error.Code != status_error.AgentCommandRequestConflict.Key() {
		t.Fatalf("changed file replayed original run: %+v", conflict)
	}
	if body, err := os.ReadFile(frozen); err != nil || string(body) != "actual child report" {
		t.Fatalf("original input was replaced: %q %v", body, err)
	}
	if err := os.Remove("report.txt"); err != nil {
		t.Fatal(err)
	}
	code, receipt := call([]string{"receipts", "get", "--operation", "workflow.runs.start", "--request-id", "report"}, nil)
	if code != 0 || receipt.CommandID != first.CommandID {
		t.Fatalf("receipt depended on deleted source file: %+v", receipt)
	}
	entries, err := os.ReadDir(s.dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "workflow-input-staging-") {
			t.Fatal("temporary input snapshot leaked")
		}
	}
}

// pausedAgentDefinition 在读取首个版本快照后暂停，确定性覆盖跨版本并发接纳。
type pausedAgentDefinition struct {
	// WorkflowController 保留真实 SQLite 仓储行为。
	controllers.WorkflowController
	// taken 保证只暂停第一份定义快照。
	taken atomic.Bool
	// paused 通知测试首个调用已经取得旧版本。
	paused chan struct{}
	// resume 允许首个调用继续处理已经取得的快照。
	resume <-chan struct{}
}

func (s *pausedAgentDefinition) GetDefinition(ctx context.Context, id string) (models.WorkflowDefinition, error) {
	definition, err := s.WorkflowController.GetDefinition(ctx, id)
	if err == nil && s.taken.CompareAndSwap(false, true) {
		close(s.paused)
		select {
		case <-s.resume:
		case <-ctx.Done():
			return models.WorkflowDefinition{}, status_error.DatabaseOperationCancelled
		}
	}
	return definition, err
}

func TestAgentConcurrentCurrentRevisionAdmissionReplaysWinner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, _ := workflowTestService(t)
	s.dataDirectory = t.TempDir()
	source := workflowTestSource(t)
	definition := savedTestWorkflow(t, s, source)
	lease := models.WorkflowConsumerLease{ID: "execution", ChatID: "chat", AuthToken: "token", DefinitionIDs: []string{definition.ID}, DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := s.consumers.CreateConsumer(ctx, lease); err != nil {
		t.Fatal(err)
	}
	s.executions = consumerExecutionBoundary{}
	s.chats = &consumerChatBoundary{actual: models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning, PermissionLevel: constants.PermissionLevelDefault}, summary: chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID, AdditionalWorkingDirectories: []string{t.TempDir()}}}}
	var start agentcapability.Binding
	for _, binding := range s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer) {
		if binding.Operation == "workflow.runs.start" {
			start = binding
		}
	}
	request := agentcapability.Request{Operation: start.Operation, RequestID: "same", Params: map[string]any{"definitionId": definition.ID, "revisionId": "", "args": map[string]any{"text": "same"}}}
	resume := make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	defer release()
	paused := &pausedAgentDefinition{WorkflowController: s.store, paused: make(chan struct{}), resume: resume}
	s.store = paused
	firstResults, firstErrors := make(chan agentcapability.Result, 1), make(chan error, 1)
	go func() {
		value, err := start.Call(ctx, request)
		firstResults <- value
		firstErrors <- err
	}()
	select {
	case <-paused.paused:
	case <-ctx.Done():
		t.Fatal("first admission did not read its revision")
	}
	saved, _, err := s.SaveSource(ctx, SaveSourceCommand{DefinitionID: definition.ID, ExpectedRevisionID: definition.CurrentRevisionID, RequestID: "revision", Source: strings.Replace(source, "纯计算验收", "第二版", 1), SDKVersion: constants.WorkflowSDKVersion})
	if err != nil || saved.RevisionID == definition.CurrentRevisionID {
		t.Fatalf("new revision was not created: %+v %v", saved, err)
	}
	second, err := start.Call(ctx, request)
	release()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case first := <-firstResults:
		if err := <-firstErrors; err != nil || first.CommandID != second.CommandID || !first.Replayed || first.Data.(map[string]any)["revisionId"] != saved.RevisionID {
			t.Fatalf("old revision admission did not replay the winner: first=%+v second=%+v err=%v", first, second, err)
		}
	case <-ctx.Done():
		t.Fatal("first admission did not finish")
	}
}

// deliveryReadFailure 在交付安全打开前的真实运行仓储读取处注入故障。
type deliveryReadFailure struct {
	// WorkflowRunController 保留真实 SQLite 读取。
	controllers.WorkflowRunController
	// reads 记录当前请求到达的读取次数。
	reads int
}

func (r *deliveryReadFailure) GetRun(ctx context.Context, id string) (models.WorkflowRun, error) {
	r.reads++
	if r.reads == 3 {
		return models.WorkflowRun{}, status_error.DatabaseError
	}
	return r.WorkflowRunController.GetRun(ctx, id)
}

// 显式交付固化真实字节，重复回执不重跑，其他运行同名文件各有独立身份。
func TestAgentDeliversDeclaredFilesToAssistantMessage(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	s.dataDirectory = t.TempDir()
	source := `export const meta={sdk:'ally.workflow/1',title:'文件交付',description:'交付报告',inputSchema:{type:'object',properties:{path:{type:'string',title:'路径',description:'已有文件路径'}},required:['path']},outputSchema:{type:'object',properties:{report:{type:'string',format:'file'},note:{type:'string'}},required:['report','note']},phases:[{id:'deliver',title:'交付',detail:'交付报告',canExit:true}]};phase('deliver');return {report:args.path,note:args.path};`
	definition := savedTestWorkflow(t, s, source)
	root := t.TempDir()
	workspaceDir := t.TempDir()
	if err := workspace.EnsureExecutionWorkspace(workspaceDir, "chat", "waker", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "report.txt")
	if err := os.WriteFile(path, []byte("real bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	lease := models.WorkflowConsumerLease{ID: "delivery-execution", ChatID: "chat", AuthToken: "token", DefinitionIDs: []string{definition.ID}, DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := s.consumers.CreateConsumer(ctx, lease); err != nil {
		t.Fatal(err)
	}
	s.chats = &consumerChatBoundary{actual: models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning}, summary: chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID, WorkspaceDirectory: workspaceDir, AdditionalWorkingDirectories: []string{root}}}}
	bindings := s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer)
	var deliver *agentcapability.Binding
	for i := range bindings {
		if bindings[i].Operation == "workflow.runs.deliver" {
			deliver = &bindings[i]
		}
	}
	if deliver == nil {
		t.Fatal("未向获准聊天提供显式最终文件交付命令")
	}
	var lastRequest string
	previous := ""
	for _, requestID := range []string{"first", "second"} {
		run, err := s.StartRun(ctx, StartRunCommand{DefinitionID: definition.ID, RequestID: requestID, Args: map[string]any{"path": path}, ConsumerLeaseID: lease.ID, ApprovedScope: &models.WorkflowScopeBinding{WorkingDirectory: root}})
		if err != nil {
			t.Fatal(err)
		}
		runTicks(t, s, run.ID, 6)
		lastRequest = requestID
		request := agentcapability.Request{Operation: "workflow.runs.deliver", RequestID: requestID, Params: map[string]any{"runId": run.ID}}
		if requestID == "second" {
			original := s.runs
			fault := &deliveryReadFailure{WorkflowRunController: original}
			s.runs = fault
			if _, err := deliver.Call(ctx, request); !errors.Is(err, status_error.DatabaseError) {
				t.Fatalf("真实读取故障被成功空回执吞掉: reads=%d err=%v", fault.reads, err)
			}
			s.runs = original
		}
		if requestID == "first" {
			original := s.commands
			s.commands = deliveryReceiptFailure{Store: original}
			if _, err := deliver.Call(ctx, request); err == nil {
				t.Fatal("注入回执写入失败未生效")
			}
			s.commands = original
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		result, err := deliver.Call(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result.Data)
		var data struct {
			// Attachments 保存真实交付引用。
			Attachments []eventlog.AttachmentReference `json:"attachments"`
			// Files 保存声明文件描述。
			Files []OutputFile `json:"files"`
		}
		if err := json.Unmarshal(encoded, &data); err != nil {
			t.Fatal(err)
		}
		if requestID == "first" {
			if err := os.WriteFile(path, []byte("real bytes"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if result.ExecutionID != lease.ID {
			t.Fatal("交付回执没有原执行归属")
		}
		if len(data.Attachments) != 1 || len(data.Files) != 1 || data.Files[0].Size != 10 || !data.Attachments[0].Managed {
			t.Fatalf("声明文件未准确交付: %s", encoded)
		}
		current := data.Attachments[0].Path
		if current == previous {
			t.Fatal("同名不同运行覆盖同一附件")
		}
		previous = current
		if body, err := os.ReadFile(filepath.Join(workspaceDir, current)); err != nil || string(body) != "real bytes" {
			t.Fatalf("未固化真实字节: %s %v", body, err)
		}
		again, err := deliver.Call(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		replayed, _ := json.Marshal(again.Data)
		if string(encoded) != string(replayed) {
			t.Fatalf("重复交付改变回执: %s %s", encoded, replayed)
		}
		receipt, err := deliver.Receipt(ctx, requestID)
		if err != nil {
			t.Fatal(err)
		}
		recovered, _ := json.Marshal(receipt.Data)
		if string(encoded) != string(recovered) {
			t.Fatal("无法恢复原交付回执")
		}
	}
	// 新回合恢复原回执时仍返回原运行执行，不把附件归入当前消息。
	next := lease
	next.ID = "next-execution"
	if err := s.consumers.CreateConsumer(ctx, next); err != nil {
		t.Fatal(err)
	}
	boundary := s.chats.(*consumerChatBoundary)
	boundary.actual.ID = next.ID
	boundary.summary.Chat.ActiveExecutionID = next.ID
	for _, binding := range s.AgentBindings(next.ID, next.AuthToken, constants.WorkflowCapabilityScopeConsumer) {
		if binding.Operation == "workflow.runs.deliver" {
			restored, err := binding.Receipt(ctx, lastRequest)
			if err != nil || restored.ExecutionID != lease.ID {
				t.Fatalf("旧交付被归入新回合: %+v %v", restored, err)
			}
		}
	}
	// 同一旧回执在另一聊天没有可查询身份。
	next.ID = "other-execution"
	next.ChatID = "other-chat"
	if err := s.consumers.CreateConsumer(ctx, next); err != nil {
		t.Fatal(err)
	}
	boundary.actual.ID = next.ID
	boundary.actual.ChatID = next.ChatID
	boundary.summary.Chat.ID = next.ChatID
	boundary.summary.Chat.ActiveExecutionID = next.ID
	for _, binding := range s.AgentBindings(next.ID, next.AuthToken, constants.WorkflowCapabilityScopeConsumer) {
		if binding.Operation == "workflow.runs.deliver" {
			if _, err := binding.Receipt(ctx, lastRequest); err == nil {
				t.Fatal("其他聊天读取了原回执")
			}
		}
	}

}

// 交付不接受普通字符串，越界软链接和缺失文件只报告不可用，GET 不创建聊天文件。
func TestAgentDeliveryRejectsEscapingFiles(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	root := t.TempDir()
	directory := t.TempDir()
	if err := workspace.EnsureExecutionWorkspace(directory, "chat", "waker", nil); err != nil {
		t.Fatal(err)
	}
	source := `export const meta={sdk:'ally.workflow/1',title:'文件安全',description:'只交付声明文件',inputSchema:{type:'object',properties:{path:{type:'string',title:'路径',description:'文件路径'}},required:['path']},outputSchema:{type:'object',properties:{file:{type:'string',format:'file'},ordinary:{type:'string'}},required:['file','ordinary']},phases:[{id:'deliver',title:'交付',detail:'交付报告',canExit:true}]};phase('deliver');return {file:args.path,ordinary:args.path};`
	definition := savedTestWorkflow(t, s, source)
	lease := models.WorkflowConsumerLease{ID: "escape", ChatID: "chat", AuthToken: "token", DefinitionIDs: []string{definition.ID}, DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := s.consumers.CreateConsumer(ctx, lease); err != nil {
		t.Fatal(err)
	}
	s.chats = &consumerChatBoundary{actual: models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning}, summary: chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID, WorkspaceDirectory: directory}}}
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	bindings := s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer)
	for _, path := range []string{link, filepath.Join(root, "missing.txt")} {
		run, err := s.StartRun(ctx, StartRunCommand{DefinitionID: definition.ID, RequestID: filepath.Base(path), ConsumerLeaseID: lease.ID, Args: map[string]any{"path": path}, ApprovedScope: &models.WorkflowScopeBinding{WorkingDirectory: root}})
		if err != nil {
			t.Fatal(err)
		}
		for _, binding := range bindings {
			if binding.Operation == "workflow.runs.deliver" {
				if _, err := binding.Call(ctx, agentcapability.Request{Operation: binding.Operation, RequestID: "active-" + run.ID, Params: map[string]any{"runId": run.ID}}); err == nil {
					t.Fatal("活跃运行被交付")
				}
			}
		}
		runTicks(t, s, run.ID, 6)
		for _, binding := range bindings {
			if binding.Operation == "workflow.runs.get" || binding.Operation == "workflow.runs.deliver" {
				result, err := binding.Call(ctx, agentcapability.Request{Operation: binding.Operation, RequestID: run.ID, Params: map[string]any{"runId": run.ID}})
				if err != nil {
					t.Fatal(err)
				}
				encoded, _ := json.Marshal(result.Data)
				var data map[string]any
				_ = json.Unmarshal(encoded, &data)
				files := data["files"].([]any)
				if len(files) != 1 || files[0].(map[string]any)["available"] != false {
					t.Fatalf("越界或缺失文件被交付或普通字符串被识别: %s", encoded)
				}
				if attachments, ok := data["attachments"].([]any); ok && len(attachments) != 0 {
					t.Fatal("产生不安全附件")
				}
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(directory, "attachments")); err != nil || len(entries) != 1 || entries[0].Name() != "generated" {
		t.Fatalf("读取或无效交付产生受管副作用: %v %v", entries, err)
	}
}

// deliveryReceiptFailure 模拟文件固化后数据库提交失败，不替代文件系统与原查询。
type deliveryReceiptFailure struct {
	// Store 继续读取真实 SQLite 回执。
	agentcommand.Store
}

func (deliveryReceiptFailure) Create(context.Context, agentcommand.Receipt) error {
	return status_error.DatabaseError
}

// advanceAfterRunRead 在观察已读取运行快照后推进真实协调者，确定性覆盖双读竞态。
type advanceAfterRunRead struct {
	// WorkflowRunController 保留真实 SQLite 行为。
	controllers.WorkflowRunController
	// once 只在第一份观察快照形成后推进运行。
	once sync.Once
	// advance 通过真实 Runner 提交终态，不直接改写数据库。
	advance func()
}

// ListOperations 在资源组装期间触发真实运行终态提交。
func (r *advanceAfterRunRead) ListOperations(ctx context.Context, id string) ([]models.WorkflowOperationRequest, error) {
	value, err := r.WorkflowRunController.ListOperations(ctx, id)
	if err == nil {
		r.once.Do(r.advance)
	}
	return value, err
}

// TestAgentWaitCompletionMatchesReturnedRunSnapshot 防止终态标志混入后续版本。
func TestAgentWaitCompletionMatchesReturnedRunSnapshot(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	s.events = events.New()
	defer s.events.Close()
	definition := savedTestWorkflow(t, s, workflowTestSource(t))
	lease := models.WorkflowConsumerLease{ID: "wait-execution", ChatID: "wait-chat", AuthToken: "token", DefinitionIDs: []string{definition.ID}, DeadlineAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := s.consumers.CreateConsumer(ctx, lease); err != nil {
		t.Fatal(err)
	}
	s.chats = &consumerChatBoundary{actual: models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning}, summary: chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID}}}
	run, err := s.StartRun(ctx, StartRunCommand{DefinitionID: definition.ID, ConsumerLeaseID: lease.ID, RequestID: "snapshot", Args: map[string]any{"text": "snapshot"}})
	if err != nil {
		t.Fatal(err)
	}
	original := s.runs
	s.runs = &advanceAfterRunRead{WorkflowRunController: original, advance: func() {
		terminal := runTicks(t, s, run.ID, 8)
		if terminal.Status != constants.WorkflowRunStatusSucceeded {
			t.Fatalf("real runner did not finish: %+v", terminal)
		}
	}}
	for _, binding := range s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer) {
		if binding.Operation != "workflow.runs.wait" {
			continue
		}
		observed, err := binding.Call(ctx, agentcapability.Request{Operation: binding.Operation, Params: map[string]any{"runId": run.ID, "timeoutMs": 1}})
		if err != nil {
			t.Fatal(err)
		}
		data := observed.Data.(map[string]any)
		snapshot := data["resource"].(map[string]any)["run"].(models.WorkflowRun)
		if data["completed"] != false || snapshot.Status == constants.WorkflowRunStatusSucceeded {
			t.Fatalf("completed must describe the returned pre-terminal snapshot: %+v", data)
		}
		current, err := original.GetRun(ctx, run.ID)
		if err != nil || current.Status != constants.WorkflowRunStatusSucceeded {
			t.Fatalf("interleaving did not reach real terminal: %+v %v", current, err)
		}
		return
	}
	t.Fatal("missing wait binding")
}
