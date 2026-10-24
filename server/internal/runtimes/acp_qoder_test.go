package runtimes

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	acp "github.com/coder/acp-go-sdk"
)

// TestQoderRegistryAndPermissions 覆盖原生目录、传输、严格权限菜单与前置参数保护。
func TestQoderRegistryAndPermissions(t *testing.T) {
	definitions, err := newRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	definition := definitions[RuntimeIDQoder]
	if definition.name != "Qoder" || definition.instructionFileName != "AGENTS.md" || definition.skillDirectoryName != ".qoder" || definition.configHomeEnvironmentVariable != "QODER_CONFIG_DIR" || !reflect.DeepEqual(definition.binaryCandidates, []string{"qoder", "qodercli"}) || len(definition.models) != 0 {
		t.Fatalf("incorrect native definition: %+v", definition)
	}
	if !slices.Contains(BaseRuntimeIDs(), RuntimeIDQoder) {
		t.Fatal("connector matrix omitted Qoder")
	}
	binding := defaultACPRuntimeBindings()[RuntimeIDQoder]
	if !binding.nativeProcess || binding.retainedProcess {
		t.Fatal("Qoder process must be isolated per invocation")
	}
	for _, transport := range []ConnectorTransport{ConnectorTransportStdio, ConnectorTransportStreamableHTTP, ConnectorTransportSSE} {
		if supported, _ := ConnectorCompatibility(RuntimeIDQoder, transport); !supported {
			t.Fatalf("missing transport %s", transport)
		}
	}
	for _, bypass := range []string{constants.RuntimeQoderBypassModeID, constants.RuntimeQoderYoloModeID} {
		modes := &acp.SessionModeState{AvailableModes: []acp.SessionMode{{Id: "default"}, {Id: acp.SessionModeId(bypass)}}}
		if got := binding.permissionLevels(nil, modes.AvailableModes); !reflect.DeepEqual(got, []PermissionLevel{PermissionLevelDefault, PermissionLevelFullAccess}) {
			t.Fatalf("invented permissions: %v", got)
		}
		for _, level := range []PermissionLevel{PermissionLevelDefault, PermissionLevelFullAccess} {
			want := acp.SessionModeId("default")
			if level == PermissionLevelFullAccess {
				want = acp.SessionModeId(bypass)
			}
			if got, err := binding.modeID(ExecutionPolicy{PermissionLevel: level}, modes); err != nil || got != want {
				t.Fatalf("mode = %s, %v", got, err)
			}
		}
		for _, policy := range []ExecutionPolicy{{PermissionLevel: PermissionLevelAuto}, {PermissionLevel: PermissionLevelFullAccess, WorkspaceReadOnly: true}} {
			if _, err := binding.modeID(policy, modes); err == nil {
				t.Fatalf("unsupported policy accepted: %+v", policy)
			}
		}
	}
	autoModes := &acp.SessionModeState{AvailableModes: []acp.SessionMode{{Id: "default"}, {Id: "auto"}, {Id: "yolo"}}}
	if got := binding.permissionLevels(nil, autoModes.AvailableModes); !reflect.DeepEqual(got, []PermissionLevel{PermissionLevelDefault, PermissionLevelAuto, PermissionLevelFullAccess}) {
		t.Fatalf("advertised auto mode lost: %v", got)
	}
	if got, err := binding.modeID(ExecutionPolicy{PermissionLevel: PermissionLevelAuto}, autoModes); err != nil || got != "auto" {
		t.Fatalf("advertised auto mode = %s, %v", got, err)
	}
	if _, err := binding.modeID(ExecutionPolicy{PermissionLevel: PermissionLevelFullAccess}, &acp.SessionModeState{AvailableModes: []acp.SessionMode{{Id: "default"}}}); err == nil {
		t.Fatal("missing bypass mode accepted")
	}
	if _, err := binding.modeID(ExecutionPolicy{}, nil); err == nil {
		t.Fatal("missing mode catalog accepted")
	}
	for flag := range protectedPrefixArguments[RuntimeIDQoder] {
		if LaunchOverrideArgumentsAllowed(RuntimeIDQoder, []string{flag}) || LaunchOverrideArgumentsAllowed(RuntimeIDQoder, []string{flag + "=override"}) {
			t.Fatalf("unsafe override accepted: %s", flag)
		}
	}
	if !LaunchOverrideArgumentsAllowed(RuntimeIDQoder, []string{"--debug"}) {
		t.Fatal("diagnostic flag rejected")
	}
}

// TestQoderReappliesPermissionOnResume 确保续聊重建进程后回落默认权限，不继承历史完全访问。
func TestQoderReappliesPermissionOnResume(t *testing.T) {
	backend := newACPBackend()
	defer backend.Close(context.Background())
	agents := []*fakeCodexACPAgent{}
	replaceACPAgentFactory(backend, RuntimeIDQoder, func(_ context.Context, _ acpAdapterConfig) (acp.Agent, error) {
		agent := &fakeCodexACPAgent{sessionID: "qoder", selectedConfigs: make(map[acp.SessionConfigId]acp.SessionConfigValueId), simplePrompt: true}
		agents = append(agents, agent)
		return &qoderFixtureAgent{fakeCodexACPAgent: agent}, nil
	})
	request := matrixACPRunRequest(t, RuntimeIDQoder, SessionDirective{Mode: SessionModeNew})
	for _, level := range []PermissionLevel{PermissionLevelFullAccess, PermissionLevelDefault} {
		request.Request.Policy.Execution.PermissionLevel = level
		result := backend.Run(context.Background(), request)
		if result.Failure != nil {
			t.Fatalf("%s: %+v", level, result)
		}
		request.Request.Session = SessionDirective{Mode: SessionModeResume, ID: result.Session.OutputID}
	}
	if len(agents) != 2 || agents[0].selectedMode != "bypassPermissions" || agents[1].selectedMode != "default" {
		t.Fatal("resumed permission did not follow current policy")
	}
}

// qoderFixtureAgent 提供 Qoder 官方文档声明的两种 ACP 模式。
type qoderFixtureAgent struct {
	// fakeCodexACPAgent 复用标准 ACP 请求记录和回执。
	*fakeCodexACPAgent
}

// NewSession 在标准会话回执中补充 Qoder 权限目录。
func (agent *qoderFixtureAgent) NewSession(ctx context.Context, request acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	response, err := agent.fakeCodexACPAgent.NewSession(ctx, request)
	response.Modes = &acp.SessionModeState{AvailableModes: []acp.SessionMode{{Id: "default", Name: "Default"}, {Id: "bypassPermissions", Name: "Bypass"}}}
	return response, err
}

// ResumeSession 使恢复同样显式携带当前权限目录。
func (agent *qoderFixtureAgent) ResumeSession(ctx context.Context, request acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	response, err := agent.fakeCodexACPAgent.ResumeSession(ctx, request)
	response.Modes = &acp.SessionModeState{AvailableModes: []acp.SessionMode{{Id: "default", Name: "Default"}, {Id: "bypassPermissions", Name: "Bypass"}}}
	return response, err
}
