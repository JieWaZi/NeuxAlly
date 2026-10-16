package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcli"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/execution"
	"github.com/JieWaZi/NeuxAlly/server/services/workflow"
)

// consumerCLIRuntime 仅替代模型协议，命令调用及最终答复均进入真实 Core。
type consumerCLIRuntime struct {
	// creationScopeRuntime 复用环境目录与请求捕获，不复用业务实现。
	creationScopeRuntime
	// summaries 控制当前模型何时结束总结，其他聊天保持活跃供授权验证。
	summaries chan string
}

// Run 向真实执行接纳边界交付启动与最终文本，等待由测试驱动的模型结果。
func (r *consumerCLIRuntime) Run(ctx context.Context, request runtimes.RunRequest, sink runtimes.EventSink) runtimes.Result {
	id := util.New()
	started, _ := json.Marshal(runtimes.InvocationStartedData{InvocationID: id})
	if err := sink.Accept(ctx, runtimes.Event{Type: runtimes.EventTypeInvocationStarted, Data: started}); err != nil {
		return runtimes.Result{ExitCode: 1}
	}
	r.requests <- request
	select {
	case text := <-r.summaries:
		content, _ := json.Marshal(map[string]any{"text": text})
		if err := sink.Accept(ctx, runtimes.Event{Type: runtimes.EventTypeTextDelta, Data: content}); err != nil {
			return runtimes.Result{ExitCode: 1}
		}
	case <-ctx.Done():
	}
	return runtimes.Result{ProtocolHealthy: true, Facts: runtimes.InvocationFacts{InvocationID: id, RuntimeID: "codex", BaseRuntimeID: "codex"}}
}

// 检验 CLI 参数、授权、运行协调、生产命令事实和附件链路；只替代外部模型与环境探测。
func TestConsumerCLIExecutesAndDeliversThroughRealCore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime := &consumerCLIRuntime{creationScopeRuntime: creationScopeRuntime{requests: make(chan runtimes.RunRequest, 2)}, summaries: make(chan string)}
	s, _ := openCoreWorkflowPorts(t, t.TempDir(), runtime, fixtureEnvironment{})
	root := t.TempDir()
	path := filepath.Join(root, "report.txt")
	if err := os.WriteFile(path, []byte("verified report bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	source := `export const meta={sdk:'ally.workflow/1',title:'聊天交付验收',description:'返回内容及声明文件',inputSchema:{type:'object',properties:{task:{type:'string'},path:{type:'string'}},required:['task','path']},outputSchema:{type:'object',properties:{summary:{type:'string'},report:{type:'string',format:'file'},plainPath:{type:'string'}},required:['summary','report','plainPath']},phases:[{id:'report',title:'报告',detail:'交付确认结果',canExit:true}]};phase('report');return {summary:args.task.toUpperCase(),report:args.path,plainPath:args.path};`
	definition := savedTestWorkflow(t, s, source)
	other, err := createDefinition(ctx, workflowAPIDependencies(s), CreateDefinitionCommand{RequestID: "other-definition", Title: "未选择流程"})
	if err != nil {
		t.Fatal(err)
	}
	otherRevision, diagnostics, err := s.SaveSource(ctx, workflow.SaveSourceCommand{DefinitionID: other.ID, RequestID: "other-save", Source: source, SDKVersion: constants.WorkflowSDKVersion})
	if err != nil {
		t.Fatalf("other runnable workflow: %v %+v", err, diagnostics)
	}
	other.CurrentRevisionID = otherRevision.RevisionID
	startChat := func(requestID string) (execution.AcceptedExecution, runtimes.RunRequest) {
		t.Helper()
		intent, err := s.BindConsumer(ctx, execution.WorkIntent{ClientRequestID: requestID, Chat: execution.ChatTarget{Create: &execution.CreateChatSpec{WakerID: "fixture-waker", WorkingDirectory: root}}, Input: execution.WorkInput{Actor: "user", Content: "review this change"}, Options: execution.WorkOptions{RuntimeID: "codex", AccountID: "system", Model: "fixture", PermissionLevel: constants.PermissionLevelDefault}}, []string{definition.ID})
		if err != nil {
			t.Fatal(err)
		}
		accepted, err := s.Executions.Start(ctx, intent)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case request := <-runtime.requests:
			return accepted, request
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		return execution.AcceptedExecution{}, runtimes.RunRequest{}
	}
	accepted, request := startChat("first-chat")
	call := func(request runtimes.RunRequest, args []string, params any, wantOK bool) agentcapability.Response {
		t.Helper()
		var input, output, failure bytes.Buffer
		if params != nil {
			if err := json.NewEncoder(&input).Encode(params); err != nil {
				t.Fatal(err)
			}
			args = append(append([]string(nil), args...), "--input", "-")
		}
		code := agentcli.Execute(ctx, args, agentcli.IO{In: &input, Out: &output, Err: &failure, ContextFile: request.Target.Environment[constants.AgentCLIContextEnvironment]})
		body := output.Bytes()
		if code != 0 {
			body = failure.Bytes()
		}
		var response agentcapability.Response
		if err := json.Unmarshal(body, &response); err != nil || response.OK != wantOK || (code == 0) != wantOK {
			t.Fatalf("CLI %v: code=%d out=%s err=%s decode=%v", args, code, &output, &failure, err)
		}
		return response
	}
	data := func(response agentcapability.Response) map[string]any {
		t.Helper()
		value, ok := response.Data.(map[string]any)
		if !ok {
			t.Fatalf("missing data: %+v", response)
		}
		return value
	}
	described := data(call(request, []string{"workflow", "definitions", "describe", "--definition-id", definition.ID}, nil, true))
	if described["revisionId"] != definition.CurrentRevisionID || described["inputSchema"] == nil || described["outputSchema"] == nil {
		t.Fatalf("missing immutable contract: %+v", described)
	}
	params := map[string]any{"definitionId": definition.ID, "revisionId": described["revisionId"], "args": map[string]any{"task": "review this change", "path": path}}
	forbidden := map[string]any{"definitionId": other.ID, "revisionId": other.CurrentRevisionID, "args": params["args"]}
	rejected := call(request, []string{"workflow", "runs", "start", "--request-id", "unselected"}, forbidden, false)
	if rejected.Error == nil || rejected.Error.Code != status_error.AgentOperationForbidden.Key() {
		t.Fatalf("unselected definition failed for the wrong reason: %+v", rejected)
	}
	unselectedRuns, err := s.Runs.ListRuns(ctx, other.ID, 0, "", 10)
	if err != nil || len(unselectedRuns) != 0 {
		t.Fatalf("unselected definition started a run: %+v %v", unselectedRuns, err)
	}
	// 暂停真实后台调度，使观察期限确定性先到期；随后恢复同一协调者推进原 Run。
	s.Close()
	started := call(request, []string{"workflow", "runs", "start", "--request-id", "stable-start"}, params, true)
	runID := data(started)["runId"].(string)
	replay := call(request, []string{"workflow", "runs", "start", "--request-id", "stable-start"}, params, true)
	receipt := call(request, []string{"receipts", "get", "--operation", "workflow.runs.start", "--request-id", "stable-start"}, nil, true)
	if !replay.Replayed || replay.CommandID != started.CommandID || receipt.CommandID != started.CommandID || data(replay)["runId"] != runID {
		t.Fatal("start did not preserve its receipt")
	}
	waitArgs := []string{"workflow", "runs", "wait", "--run-id", runID, "--timeout", "1ms"}
	waiting := data(call(request, waitArgs, nil, true))
	waitingRun := waiting["resource"].(map[string]any)["run"].(map[string]any)
	if waiting["completed"] != false || waitingRun["id"] != runID || waitingRun["status"] == "cancelled" {
		t.Fatalf("observation changed run: %+v", waiting)
	}
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	completed := data(call(request, []string{"workflow", "runs", "wait", "--run-id", runID, "--timeout", "10s"}, nil, true))
	if completed["completed"] != true {
		t.Fatalf("run did not finish: %+v", completed)
	}
	result := data(call(request, []string{"workflow", "runs", "get", "--run-id", runID}, nil, true))
	run := result["run"].(map[string]any)
	if run["id"] != runID || run["status"] != "succeeded" || run["result"].(map[string]any)["summary"] != "REVIEW THIS CHANGE" || len(result["files"].([]any)) != 1 {
		t.Fatalf("lost content or declared file: %+v", result)
	}
	history, err := s.Chats.ReadMessageHistory(ctx, accepted.Chat.ID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range history.Resources {
		if len(message.Attachments) != 0 {
			t.Fatal("get/wait registered files before deliver")
		}
	}
	delivered := call(request, []string{"workflow", "runs", "deliver", "--run-id", runID, "--request-id", "stable-delivery"}, nil, true)
	attachments := data(delivered)["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("wrong final attachments: %+v", delivered)
	}
	attachment := attachments[0].(map[string]any)
	if attachment["managed"] != true || attachment["name"] != "report.txt" {
		t.Fatalf("not a managed chat file: %+v", attachment)
	}
	// 删除原产物后恢复同一回执，证明交付字节已固化且恢复不重新读取或执行。
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		restored := call(request, []string{"receipts", "get", "--operation", "workflow.runs.deliver", "--request-id", "stable-delivery"}, nil, true)
		if restored.CommandID != delivered.CommandID || data(restored)["attachments"].([]any)[0].(map[string]any)["path"] != attachment["path"] {
			t.Fatal("delivery receipt identity changed")
		}
	}
	repeated := call(request, []string{"workflow", "runs", "deliver", "--run-id", runID, "--request-id", "stable-delivery"}, nil, true)
	if !repeated.Replayed {
		t.Fatal("delivery was not replayed")
	}
	history, err = s.Chats.ReadMessageHistory(ctx, accepted.Chat.ID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	attached := 0
	for _, message := range history.Resources {
		if len(message.Attachments) == 0 {
			continue
		}
		attached++
		if message.ExecutionID != accepted.Execution.ID || len(message.Attachments) != 1 || len(message.WorkflowRuns) != 1 || message.WorkflowRuns[0].RunID != runID || message.Attachments[0].Path != attachment["path"] {
			t.Fatalf("original message lost run/file identity: %+v", message)
		}
	}
	if attached != 1 {
		t.Fatalf("registered %d attachment messages", attached)
	}
	// 经正式消息中的相对附件路径读取真实字节；桌面文件读取另由 Electron 测试覆盖。
	summary, err := s.Chats.GetChatSummary(ctx, accepted.Chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(summary.Chat.WorkspaceDirectory, attachment["path"].(string)))
	if err != nil || string(content) != "verified report bytes" {
		t.Fatalf("registered file bytes: %q %v", content, err)
	}
	runs, err := s.Runs.ListRuns(ctx, definition.ID, 0, "", 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("replay/observation started additional runs: %+v %v", runs, err)
	}

	// 右侧入口写真实用户停止原因，等待仍通过原 CLI 查询并由原 Runner 收敛。
	s.Close()
	second := data(call(request, []string{"workflow", "runs", "start", "--request-id", "cancelled-run"}, params, true))["runId"].(string)
	if err := s.CancelRunWithReason(ctx, second, &models.WorkflowCancellationReason{Actor: constants.WorkflowCancellationActorUser, Source: constants.WorkflowCancellationSourceRun, RequestedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	stopped := data(call(request, []string{"workflow", "runs", "wait", "--run-id", second, "--timeout", "10s"}, nil, true))
	stoppedRun := stopped["resource"].(map[string]any)["run"].(map[string]any)
	reason := stoppedRun["cancellationReason"].(map[string]any)
	if stopped["completed"] != true || stoppedRun["status"] != "cancelled" || reason["actor"] != "user" || reason["source"] != "run" || reason["requestedAt"].(float64) <= 0 {
		t.Fatalf("lost user cancellation: %+v", stopped)
	}
	// 原智能体总结与正式附件同时保留到终态及 JSONL 历史中。
	runtime.summaries <- "REVIEW THIS CHANGE"
	for {
		current, readErr := s.Chats.ReadExecution(ctx, accepted.Chat.ID, accepted.Execution.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if current.Status == constants.ExecutionStatusCompleted {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	history, err = s.Chats.ReadMessageHistory(ctx, accepted.Chat.ID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	mixed := false
	for _, message := range history.Resources {
		if message.ExecutionID == accepted.Execution.ID && message.Role == constants.MessageRoleAssistant {
			mixed = message.Content == "REVIEW THIS CHANGE" && len(message.Attachments) == 1 && len(message.WorkflowRuns) == 2 && message.WorkflowRuns[0].RunID == runID && message.WorkflowRuns[1].RunID == second
		}
	}
	if !mixed {
		t.Fatalf("summary/file replay lost mixed delivery: %+v", history.Resources)
	}

	_, otherRequest := startChat("other-chat")
	call(otherRequest, []string{"workflow", "runs", "get", "--run-id", runID}, nil, false)
	call(otherRequest, []string{"workflow", "runs", "deliver", "--run-id", runID, "--request-id", "foreign"}, nil, false)
	call(otherRequest, []string{"receipts", "get", "--operation", "workflow.runs.deliver", "--request-id", "stable-delivery"}, nil, false)

}
