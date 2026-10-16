package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability/contract"
	eventlog "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	builtinskills "github.com/JieWaZi/NeuxAlly/server/internal/skills/builtin"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
)

// agentCommandFacts 将已持久化命令事实交给同执行实时投影，无需轮询日志。
type agentCommandFacts struct {
	// mutex 保护命令提交与 Runtime 增量并发交付。
	mutex sync.Mutex
	// pending 只保留实时投影尚未消费的权威事实。
	pending []eventlog.Record
}

func (f *agentCommandFacts) record(ctx context.Context, journal chatservice.Journal, chatID, executionID string, op contract.Operation, result agentcapability.Result) error {
	records, err := journal.AppendExecutionFacts(ctx, chatID, []eventlog.AppendInput{{Type: constants.ChatEventTypeAgentCommandCommitted, ExecutionID: executionID, Data: marshalData(eventlog.AgentCommandData{CommandID: result.CommandID, Operation: op.ID, Completion: op.Completion})}})
	if err == nil && f != nil {
		f.mutex.Lock()
		f.pending = append(f.pending, records...)
		f.mutex.Unlock()
	}
	return err
}

func (f *agentCommandFacts) take() []eventlog.Record {
	if f == nil {
		return nil
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	result := f.pending
	f.pending = nil
	return result
}

// agentSessionSlot 对应一个真实原生会话与其独占连接文件目录。
type agentSessionSlot struct {
	// sessionID 是可靠的原生会话身份。
	sessionID string
	// directory 是该会话进程环境中保持稳定的目录。
	directory string
}

// agentSessionFiles 串行保护原生会话的连接文件映射，不保存业务授权。
type agentSessionFiles struct {
	// mutex 保护会话槽位和文件准备。
	mutex sync.Mutex
	// slots 按 Chat、Runtime 和账号保存当前真实会话。
	slots map[string]agentSessionSlot
}

func agentIdentity(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func (m *service) prepareAgent(request *runtimes.RunRequest, chat models.Chat, executionID string, capabilities []ToolCapability, allowedDomains *[]string, facts *agentCommandFacts) (func(), func(), func(runtimes.Result), error) {
	if m.agent == nil {
		return nil, nil, nil, nil
	}
	binary, err := filepath.EvalSymlinks(m.agentBinary)
	if err != nil || !filepath.IsAbs(binary) {
		return nil, nil, nil, status_error.AgentConnectionUnavailable
	}
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
		return nil, nil, nil, status_error.AgentConnectionUnavailable
	}
	// 系统技能与执行授权描述保留在程序数据目录，不随业务工作文件切换根目录。
	root, err := filepath.EvalSymlinks(m.skillDataDirectory)
	if err != nil {
		return nil, nil, nil, status_error.AgentContextStorageError
	}
	skill, err := builtinskills.SystemDirectory(root)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, item := range request.Skills {
		if item.Name == constants.AgentCLISkillName {
			return nil, nil, nil, status_error.SkillContentInvalid
		}
	}
	request.Skills = append(request.Skills, runtimes.Skill{Name: constants.AgentCLISkillName, Path: skill})
	request.Instructions = append(request.Instructions, runtimes.Instruction{Name: "软件能力", Content: "For NeuxAlly-provided operations, read the ally skill and use allycli capabilities to discover the current authorized commands. Read only the relevant guide and live operation contract."})
	roots := []string{}
	for _, candidate := range append([]string{request.Workspace.Path}, chat.AdditionalWorkingDirectories...) {
		if actual, err := filepath.EvalSymlinks(candidate); err == nil {
			roots = append(roots, actual)
		}
	}
	bindings := []agentcapability.Binding{}
	if m.agentBindings != nil {
		for _, binding := range m.agentBindings(chat, executionID, roots) {
			domain, _, _ := strings.Cut(binding.Operation, ".")
			if allowedDomains == nil || slices.Contains(*allowedDomains, domain) {
				bindings = append(bindings, binding)
			}
		}
	}
	if allowedDomains == nil || slices.Contains(*allowedDomains, "interaction") {
		bindings = append(bindings, m.questionBindings(chat.ID, executionID)...)
	}
	for _, capability := range capabilities {
		bindings = append(bindings, capability.Bindings...)
	}
	grant, activate, revoke, err := m.agent.Prepare(agentcapability.GrantSpec{ExecutionID: executionID, Bindings: bindings, FileRoots: roots, Record: func(ctx context.Context, op contract.Operation, result agentcapability.Result) error {
		return facts.record(ctx, m.journal, chat.ID, executionID, op, result)
	}, Validate: func(ctx context.Context) error {
		current, err := m.journal.LoadExecutionContext(ctx, chat.ID)
		if err != nil {
			return err
		}
		if current.Chat.WakerID != chat.WakerID || current.Chat.ActiveExecutionID != executionID {
			return status_error.AgentContextUnauthorized
		}
		return nil
	}})
	if err != nil {
		return nil, nil, nil, err
	}
	key := agentIdentity(chat.ID + ":" + request.Target.RuntimeID + ":" + request.Target.AccountID)
	m.agentSessions.mutex.Lock()
	slot := m.agentSessions.slots[key]
	if request.Session.Mode == runtimes.SessionModeNew || slot.directory == "" || slot.sessionID != request.Session.ID {
		slot = agentSessionSlot{sessionID: request.Session.ID, directory: filepath.Join(root, "agent-contexts", key, util.New())}
	}
	path, err := agentcapability.WriteConnection(slot.directory, "connection.json", agentcapability.Connection{ProtocolVersion: constants.AgentCLIProtocolVersion, Endpoint: m.agentEndpoint, CoreEpoch: grant.Epoch, GrantID: grant.ID, Token: grant.Token})
	if err == nil {
		m.agentSessions.slots[key] = slot
	}
	m.agentSessions.mutex.Unlock()
	if err != nil {
		revoke()
		return nil, nil, nil, err
	}
	request.UserInputState = m.elicitations.inputState(executionID)
	if request.Target.Environment == nil {
		request.Target.Environment = map[string]string{}
	}
	request.Target.Environment[constants.AgentCLIContextEnvironment] = path
	request.Target.Environment[constants.AgentCLIBinaryEnvironment] = binary
	inheritedPath := request.Target.Environment["PATH"]
	if inheritedPath == "" {
		inheritedPath = os.Getenv("PATH")
	}
	request.Target.Environment["PATH"] = filepath.Dir(binary) + string(filepath.ListSeparator) + inheritedPath
	settle := func(result runtimes.Result) {
		if sessionID := reliableRuntimeSessionID(result); sessionID != "" {
			m.agentSessions.mutex.Lock()
			defer m.agentSessions.mutex.Unlock()
			if current := m.agentSessions.slots[key]; current.directory == slot.directory {
				current.sessionID = sessionID
				m.agentSessions.slots[key] = current
			}
		}
	}
	return activate, func() { revoke(); m.elicitations.finishQuestions(chat.ID, executionID) }, settle, nil
}
