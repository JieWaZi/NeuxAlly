package execution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/chatstorage"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
)

// agentTestJournal 只替代权威 Chat 身份读取，不替代授权注册表和文件系统。
type agentTestJournal struct {
	// Journal 保留不应在本测试调用的完整日志边界。
	chatservice.Journal
	// chat 是测试推进的实际当前执行身份。
	chat models.Chat
}

func (j *agentTestJournal) LoadExecutionContext(context.Context, string) (chatservice.ExecutionContext, error) {
	return chatservice.ExecutionContext{Chat: j.chat}, nil
}

func TestNativeSessionKeepsContextPathButRotatesAndRevokesCredentials(t *testing.T) {
	registry := agentcapability.New(nil)
	defer registry.Close()
	endpoint := httptest.NewServer(registry)
	defer endpoint.Close()
	layout, err := chatstorage.NewLayout(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	journal := &agentTestJournal{chat: models.Chat{ID: "chat", WakerID: "waker", ActiveExecutionID: "first"}}
	programDirectory := t.TempDir()
	manager := &service{agent: registry, agentEndpoint: endpoint.URL + constants.AgentCLIPath, agentBinary: binary, agentSessions: &agentSessionFiles{slots: map[string]agentSessionSlot{}}, journal: journal, storage: layout, skillDataDirectory: programDirectory, elicitations: newElicitationCoordinator(journal)}
	request := runtimes.RunRequest{Target: runtimes.RuntimeTarget{RuntimeID: "codex", AccountID: "account"}, Session: runtimes.SessionDirective{Mode: runtimes.SessionModeNew}}
	activate, revoke, settle, err := manager.prepareAgent(&request, journal.chat, "first", nil, &[]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	activate()
	for _, directory := range []string{"system-skills", "agent-contexts"} {
		if _, err := os.Stat(filepath.Join(programDirectory, directory)); err != nil {
			t.Fatalf("program resources were not kept in program data: %s %v", directory, err)
		}
		if _, err := os.Stat(filepath.Join(layout.Root(), directory)); !os.IsNotExist(err) {
			t.Fatalf("program resources leaked into managed files: %s %v", directory, err)
		}
	}
	firstPath := request.Target.Environment[constants.AgentCLIContextEnvironment]
	first, err := agentcapability.ReadConnection(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	settle(runtimes.Result{Session: runtimes.SessionResult{OutputID: "native", State: runtimes.SessionStateCreated}})
	revoke()
	journal.chat.ActiveExecutionID = "second"
	secondRequest := runtimes.RunRequest{Target: request.Target, Session: runtimes.SessionDirective{Mode: runtimes.SessionModeResume, ID: "native"}}
	activate, revokeSecond, _, err := manager.prepareAgent(&secondRequest, journal.chat, "second", nil, &[]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer revokeSecond()
	activate()
	if secondRequest.Target.Environment[constants.AgentCLIContextEnvironment] != firstPath {
		t.Fatal("continued native process lost its original environment file path")
	}
	second, err := agentcapability.ReadConnection(firstPath)
	if err != nil || second.Token == first.Token || second.GrantID == first.GrantID {
		t.Fatalf("credential did not rotate: %v", err)
	}
	send := func(token string) int {
		request, _ := http.NewRequest(http.MethodGet, endpoint.URL+constants.AgentCLIPath+"/capabilities", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := endpoint.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if send(first.Token) != 401 || send(second.Token) != 200 {
		t.Fatal("old invocation crossed into a later grant")
	}
	newRequest := runtimes.RunRequest{Target: request.Target, Session: runtimes.SessionDirective{Mode: runtimes.SessionModeNew}}
	_, revokeNew, _, err := manager.prepareAgent(&newRequest, journal.chat, "second", nil, &[]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer revokeNew()
	if newRequest.Target.Environment[constants.AgentCLIContextEnvironment] == firstPath {
		t.Fatal("new native session reused an old process credential file")
	}
}
