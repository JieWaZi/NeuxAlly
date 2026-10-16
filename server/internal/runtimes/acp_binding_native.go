package runtimes

import (
	"context"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/accountconfig"
	acpgemini "github.com/JieWaZi/acp-go/pkg/gemini"
	acpgrok "github.com/JieWaZi/acp-go/pkg/grok"
	acpopencode "github.com/JieWaZi/acp-go/pkg/opencode"
	acp "github.com/coder/acp-go-sdk"
)

// managedNativeBinding 由适配器冻结权限，每次调用重建进程并通过原生历史恢复。
func managedNativeBinding(id, name string) acpRuntimeBinding {
	binding := nativeACPRuntimeBinding()
	binding.nativeVersionMetadataOnly = true
	binding.baseRuntimeID = id
	binding.displayName = name
	binding.supportsMCPTransport = allMCPTransports
	binding.permissionLevels = func([]acp.SessionConfigOption, []acp.SessionMode) []PermissionLevel {
		return []PermissionLevel{PermissionLevelDefault, PermissionLevelAuto, PermissionLevelFullAccess}
	}
	binding.modeID = func(policy ExecutionPolicy, _ *acp.SessionModeState) (acp.SessionModeId, error) {
		if policy.WorkspaceReadOnly {
			return "", status_error.RuntimeProtocolIncompatible
		}
		return "", nil
	}
	binding.validateSelection = cursorValidateSelection
	return binding
}

// openCodeACPRuntimeBinding 只在受管账号边界转换真实 provider/model ID。
func openCodeACPRuntimeBinding() acpRuntimeBinding {
	binding := managedNativeBinding(RuntimeIDOpenCode, "OpenCode")
	binding.resolveSelections = func(_ *embeddedACPAgent, _ acp.SessionId, request RunRequest) RunRequest {
		if request.Target.Isolated && request.Target.AccountID != "" {
			request.Policy.Model = accountconfig.NativeModelID(RuntimeIDOpenCode, request.Policy.Model)
		}
		return request
	}
	binding.newAgent = func(ctx context.Context, config acpAdapterConfig) (acp.Agent, error) {
		return acpopencode.NewAgent(ctx, acpopencode.Config{Logger: acpAdapterLogger(ctx), OpenCodePath: config.Executable, PrefixArgs: cloneStrings(config.PrefixArgs), Environment: cloneStrings(config.Environment), WorkingDirectory: config.WorkingDirectory, PermissionMode: config.PermissionMode})
	}
	return binding
}

// geminiACPRuntimeBinding 使用账号受管根保存 owned-child 的稳定身份和原生历史。
func geminiACPRuntimeBinding() acpRuntimeBinding {
	binding := managedNativeBinding(RuntimeIDGemini, "Gemini CLI")
	binding.stateDirectory = func(request acpRunRequest) string { return request.Prepared.environment.get("GEMINI_CLI_HOME") }
	binding.modeID = func(policy ExecutionPolicy, _ *acp.SessionModeState) (acp.SessionModeId, error) {
		if policy.WorkspaceReadOnly {
			return "", status_error.RuntimeProtocolIncompatible
		}
		if policy.PermissionLevel == PermissionLevelFullAccess {
			return "yolo", nil
		}
		return "default", nil
	}
	binding.newAgent = func(ctx context.Context, config acpAdapterConfig) (acp.Agent, error) {
		return acpgemini.NewAgent(ctx, acpgemini.Config{Logger: acpAdapterLogger(ctx), GeminiPath: config.Executable, PrefixArgs: cloneStrings(config.PrefixArgs), Environment: cloneStrings(config.Environment), WorkingDirectory: config.WorkingDirectory, StateDirectory: config.StateDirectory, PermissionMode: config.PermissionMode})
	}
	return binding
}

// grokACPRuntimeBinding 由原生会话 metadata 安装 ask、auto 或 yolo 权限。
func grokACPRuntimeBinding() acpRuntimeBinding {
	binding := managedNativeBinding(RuntimeIDGrok, "Grok Build")
	binding.newAgent = func(ctx context.Context, config acpAdapterConfig) (acp.Agent, error) {
		return acpgrok.NewAgent(ctx, acpgrok.Config{Logger: acpAdapterLogger(ctx), GrokPath: config.Executable, PrefixArgs: cloneStrings(config.PrefixArgs), Environment: cloneStrings(config.Environment), WorkingDirectory: config.WorkingDirectory, PermissionMode: config.PermissionMode})
	}
	return binding
}
