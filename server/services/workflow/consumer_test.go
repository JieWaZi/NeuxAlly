package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	workflowfiles "github.com/JieWaZi/NeuxAlly/server/internal/workflow/files"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/NeuxAlly/server/services/execution"
)

// consumerExecutionBoundary 只替代外部 Runtime 配置验证，运行仍经真实 SQLite 和 Compiler。
type consumerExecutionBoundary struct {
	// Service 保留与本测试无关的完整 Core 接口，未使用方法不得调用。
	execution.Service
}

func (consumerExecutionBoundary) ValidateConfiguration(context.Context, execution.ExecutionConfiguration) error {
	return nil
}

// consumerChatBoundary 提供 Runtime 所属真实回合的可控权威查询边界。
type consumerChatBoundary struct {
	// Module 保留与能力校验无关的 Chat 接口，未使用方法不得调用。
	chat.Module
	// actual 是当前权威执行配置和生命周期事实。
	actual models.Execution
	// summary 保存当前回合和获准工作范围。
	summary chat.ChatSummary
}

func (c *consumerChatBoundary) ReadExecution(_ context.Context, chatID, executionID string) (models.Execution, error) {
	if c.actual.ChatID != chatID || c.actual.ID != executionID {
		return models.Execution{}, status_error.ExecutionNotFound
	}
	return c.actual, nil
}
func (c *consumerChatBoundary) GetChatSummary(_ context.Context, chatID string) (chat.ChatSummary, error) {
	if c.summary.Chat.ID != chatID {
		return chat.ChatSummary{}, status_error.ChatNotFound
	}
	return c.summary, nil
}

func TestConsumerCLIScopesReplayActualDefaultsAndRevocation(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	definition := savedTestWorkflow(t, s, workflowTestSource(t))
	other, err := fixtureDefinition(ctx, s.store, definitionFixture{RequestID: "create-other", Title: "另一个流程"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, _, err := s.SaveSource(ctx, SaveSourceCommand{DefinitionID: other.ID, RequestID: "save", Source: strings.Replace(workflowTestSource(t), "纯计算验收", "另一个流程", 1), SDKVersion: constants.WorkflowSDKVersion})
	if err != nil {
		t.Fatal(err)
	}
	other.CurrentRevisionID = receipt.RevisionID
	s.executions = consumerExecutionBoundary{}
	queries := &consumerChatBoundary{}
	s.chats = queries
	s.state.AgentBindings = s.AgentBindings
	intent := execution.WorkIntent{ClientRequestID: "consumer-create", Chat: execution.ChatTarget{Create: &execution.CreateChatSpec{WakerID: "fixture", WorkingDirectory: t.TempDir()}}, Input: execution.WorkInput{Content: "调用选中的流程"}, Options: execution.WorkOptions{RuntimeID: "runtime", AccountID: "system", Model: "selected", PermissionLevel: constants.PermissionLevelDefault, WorkspaceReadOnly: true}}
	bound, err := s.BindConsumer(ctx, intent, []string{definition.ID})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := s.consumers.GetConsumer(ctx, bound.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.BindConsumer(ctx, intent, []string{definition.ID})
	if err != nil || again.ExecutionID != bound.ExecutionID || len(again.Options.ToolCapabilities[0].Bindings) != len(bound.Options.ToolCapabilities[0].Bindings) {
		t.Fatalf("capability replay changed identity: %+v %v", again, err)
	}
	if _, err := s.BindConsumer(ctx, intent, []string{definition.ID, other.ID}); !errors.Is(err, status_error.BadRequest) {
		t.Fatalf("replay expanded whitelist: %v", err)
	}
	queries.actual = models.Execution{ID: lease.ID, ChatID: lease.ChatID, Status: constants.ExecutionStatusRunning, RuntimeID: "actual-runtime", AccountID: "actual-account", Model: "actual-model", Reasoning: "actual-reasoning", PermissionLevel: constants.PermissionLevelDefault, WorkspaceReadOnly: true}
	approvedDirectory := t.TempDir()
	approvedDirectory, err = workflowfiles.EnsureScopeDirectory(approvedDirectory)
	if err != nil {
		t.Fatal(err)
	}
	queries.summary = chat.ChatSummary{Chat: models.Chat{ID: lease.ChatID, ActiveExecutionID: lease.ID, AdditionalWorkingDirectories: []string{approvedDirectory}}}
	registry := agentcapability.New(nil)
	grant, activate, revoke, err := registry.Prepare(agentcapability.GrantSpec{ExecutionID: lease.ID, Bindings: s.AgentBindings(lease.ID, lease.AuthToken, constants.WorkflowCapabilityScopeConsumer), Validate: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	activate()
	defer revoke()
	endpoint := httptest.NewServer(registry)
	defer endpoint.Close()
	call := func(input map[string]any, failed bool) map[string]any {
		t.Helper()
		params := map[string]any{}
		for key, value := range input {
			if key != "action" && key != "type" && key != "requestId" {
				if key == "workflowId" {
					key = "definitionId"
				}
				params[key] = value
			}
		}
		id, _ := input["action"].(string)
		switch id {
		case "query":
			switch input["type"] {
			case constants.WorkflowQueryWorkflows, constants.WorkflowQueryCapabilities:
				id = "workflow.definitions.list"
			case constants.WorkflowQuerySDK:
				id = "workflow.sdk.get"
			default:
				id = "workflow.results.schema"
			}
		case "run.start":
			id = "workflow.runs.start"
			params["revisionId"] = definition.CurrentRevisionID
			if params["args"] == nil {
				params["args"] = map[string]any{}
			}
		case "run.get":
			id = "workflow.runs.get"
		case "run.cancel":
			id = "workflow.runs.cancel"
		default:
			id = "workflow." + id
		}
		requestID, _ := input["requestId"].(string)
		body, _ := json.Marshal(agentcapability.Request{Operation: id, RequestID: requestID, Params: params})
		request, _ := http.NewRequest(http.MethodPost, endpoint.URL+constants.AgentCLIPath+"/commands", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+grant.Token)
		response, err := endpoint.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var value map[string]any
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
			t.Fatal(err)
		}
		if value["ok"] != !failed {
			t.Fatalf("CLI success mismatch: %+v failed=%t", value, failed)
		}
		return value
	}
	list := call(map[string]any{"action": constants.WorkflowActionQuery, "type": constants.WorkflowQueryWorkflows}, false)
	call(map[string]any{"action": constants.WorkflowActionQuery, "type": constants.WorkflowQuerySDK}, true)
	call(map[string]any{"action": constants.WorkflowActionQuery, "type": constants.WorkflowQueryResultSchema}, true)
	call(map[string]any{"action": constants.WorkflowActionSourceGet}, true)
	items := list["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != definition.ID {
		t.Fatalf("directory leaked unselected definition: %+v", items)
	}
	start := map[string]any{"action": "run.start", "workflowId": definition.ID, "requestId": "run", "args": map[string]any{"text": "ALLY"}}
	first := call(start, false)["data"].(map[string]any)
	second := call(start, false)["data"].(map[string]any)
	runID, _ := first["runId"].(string)
	if first["definitionId"] != definition.ID || first["revisionId"] != definition.CurrentRevisionID || first["name"] != "纯计算验收" || second["name"] != first["name"] {
		t.Fatalf("启动回执缺少不可变运行身份: first=%+v replay=%+v", first, second)
	}
	if runID == "" || runID != second["runId"] {
		t.Fatal("duplicate consumer start created a second run")
	}
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil || run.Defaults.RuntimeID != "actual-runtime" || run.Defaults.AccountID != "actual-account" || run.Defaults.Model != "actual-model" || !run.Defaults.WorkspaceReadOnly || run.ScopeBinding.WorkingDirectory != approvedDirectory || run.Context["triggerSource"] != "agent" {
		t.Fatalf("model escaped actual execution configuration: %+v %v", run, err)
	}
	runTicks(t, s, runID, 12)
	call(map[string]any{"action": "run.get", "runId": runID}, false)
	call(map[string]any{"action": "run.start", "workflowId": other.ID, "requestId": "other"}, true)
	call(map[string]any{"action": "run.start", "workflowId": definition.ID, "requestId": "escalate", "permissionLevel": "full_access"}, true)
	manual, err := s.StartRun(ctx, StartRunCommand{DefinitionID: definition.ID, RequestID: "manual", Args: map[string]any{"text": "manual"}})
	if err != nil {
		t.Fatal(err)
	}
	call(map[string]any{"action": "run.get", "runId": manual.ID}, true)
	call(map[string]any{"action": "run.cancel", "runId": manual.ID, "requestId": "cancel"}, true)
	call(map[string]any{"action": "run.cancel", "runId": runID, "requestId": "cancel"}, false)
	now := time.Now().UnixMilli()
	queries.actual.CompletedAt = &now
	queries.actual.Status = constants.ExecutionStatusCompleted
	call(map[string]any{"action": "run.get", "runId": runID}, true)
	call(map[string]any{"action": constants.WorkflowActionQuery, "type": constants.WorkflowQueryCapabilities}, true)
}

// 本测试捕获绑定丢失定义快照、上下文或接受多个定义的行为回归。
func TestConsumerBindsOneWorkflowReferenceAndInstructions(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	definition := savedTestWorkflow(t, s, workflowTestSource(t))
	definition, _ = s.store.GetDefinition(ctx, definition.ID)
	s.executions = consumerExecutionBoundary{}
	s.chats = &consumerChatBoundary{}
	s.state.AgentBindings = s.AgentBindings
	intent := execution.WorkIntent{ClientRequestID: "reference", Chat: execution.ChatTarget{Create: &execution.CreateChatSpec{WakerID: "fixture", WorkingDirectory: t.TempDir()}}, Input: execution.WorkInput{Content: "评审本次改动"}, Options: execution.WorkOptions{RuntimeID: "runtime", PermissionLevel: constants.PermissionLevelDefault}}
	t.Run("真实定义引用和执行上下文", func(t *testing.T) {
		bound, err := s.BindConsumer(ctx, intent, []string{definition.ID})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(bound.Input)
		var input map[string]any
		if err := json.Unmarshal(encoded, &input); err != nil {
			t.Fatal(err)
		}
		refs, _ := input["workflows"].([]any)
		if len(refs) != 1 || refs[0].(map[string]any)["id"] != definition.ID || refs[0].(map[string]any)["name"] != definition.Title {
			t.Fatalf("绑定输入缺少真实工作流快照: %s", encoded)
		}
		if len(bound.Input.ContextInstructions) == 0 {
			t.Fatal("选中工作流未提供读取契约并通过 CLI 启动等待的执行上下文")
		}
		if bound.Input.Content != "评审本次改动" {
			t.Fatal("工作流执行上下文污染用户正文")
		}
	})
	t.Run("拒绝两个定义", func(t *testing.T) {
		other := savedTestWorkflow(t, s, strings.Replace(workflowTestSource(t), "纯计算验收", "第二个流程", 1))
		intent.ClientRequestID = "two-definitions"
		if _, err := s.BindConsumer(ctx, intent, []string{definition.ID, other.ID}); err == nil {
			t.Fatal("单条聊天消息接受了两个工作流")
		}
	})
}

// 本测试验证重新绑定读取真实改名快照，但受信展示快照不能改变原调用授权身份。
func TestConsumerRenamePreservesBindingIdentity(t *testing.T) {
	ctx := context.Background()
	s, _ := workflowTestService(t)
	definition := savedTestWorkflow(t, s, workflowTestSource(t))
	s.executions = consumerExecutionBoundary{}
	s.chats = &consumerChatBoundary{}
	s.state.AgentBindings = s.AgentBindings
	intent := execution.WorkIntent{ClientRequestID: "rename-binding", Chat: execution.ChatTarget{Create: &execution.CreateChatSpec{WakerID: "fixture", WorkingDirectory: t.TempDir()}}, Input: execution.WorkInput{Content: "评审改动"}, Options: execution.WorkOptions{RuntimeID: "runtime", PermissionLevel: constants.PermissionLevelDefault}}
	first, err := s.BindConsumer(ctx, intent, []string{definition.ID})
	if err != nil {
		t.Fatal(err)
	}
	definition, err = s.store.GetDefinition(ctx, definition.ID)
	if err != nil {
		t.Fatal(err)
	}
	definition.Title = "新的评审名称"
	if err := s.store.UpdateHead(ctx, definition, definition.CurrentRevisionID); err != nil {
		t.Fatal(err)
	}
	// 既有受信入口可能带上旧展示快照；它会被真实定义替换，不属于本次请求选择。
	intent.Input.Workflows = first.Input.Workflows
	again, err := s.BindConsumer(ctx, intent, []string{definition.ID})
	if err != nil || again.ExecutionID != first.ExecutionID || len(again.Input.Workflows) != 1 || again.Input.Workflows[0].Name != "新的评审名称" {
		t.Fatalf("改名快照改变了稳定绑定身份: %+v %v", again.Input.Workflows, err)
	}
	if first.Input.Workflows[0].Name == "新的评审名称" {
		t.Fatal("重新绑定修改了旧快照")
	}
}
