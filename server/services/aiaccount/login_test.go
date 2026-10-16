package aiaccount

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/accountconfig"
	"github.com/JieWaZi/NeuxAlly/server/internal/codexoauth"
	"github.com/JieWaZi/NeuxAlly/server/internal/notification/events"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func subscriptionFixture(t *testing.T) (*Service, models.AIAccount, codexCredential) {
	t.Helper()
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "accounts.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	service := New(controllers.NewAIAccountController(database.DB))
	service.Start(context.Background())
	t.Cleanup(service.Close)
	account := models.AIAccount{ID: "0123456789abcdef0123456789abcdef", Name: "subscription", AuthMode: constants.AccountAuthModeSubscription, Endpoints: []models.AIAccountEndpoint{{Protocol: constants.AccountProtocolOpenAIResponses, BaseURL: constants.AIAccountOpenAIURL}}}
	credential := codexCredential{Tokens: subscriptionTokens("user", "refresh-original"), LastRefresh: time.Now().UTC().Add(-time.Minute)}
	encoded, _ := json.Marshal(credential)
	if err := service.accounts.Create(context.Background(), account, string(encoded)); err != nil {
		t.Fatal(err)
	}
	account, err = service.accounts.Get(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	return service, account, credential
}

func subscriptionTokens(subject, refresh string) codexoauth.Tokens {
	payload, _ := json.Marshal(map[string]any{"sub": subject, "exp": time.Now().Add(time.Hour).Unix(), "https://api.openai.com/auth": map[string]string{"chatgpt_account_id": "workspace"}})
	token := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	return codexoauth.Tokens{IDToken: token, AccessToken: token, RefreshToken: refresh}
}

func TestSubscriptionRecoversNativeRefreshBeforeNextWorkspace(t *testing.T) {
	service, account, _ := subscriptionFixture(t)
	ctx := context.Background()
	workspace := t.TempDir()
	_, release, err := service.Prepare(ctx, account.ID, constants.RuntimeIDCodex, workspace, "")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟官方 CLI 在一次执行中轮换了 refresh token。
	tokens := subscriptionTokens("user", "refresh-from-cli")
	live := codexLiveAuth{LastRefresh: time.Now().UTC(), Tokens: codexLiveTokens{IDToken: tokens.IDToken, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, AccountID: tokens.AccountID()}}
	data, _ := json.Marshal(live)
	if err := accountconfig.WriteFile(filepath.Join(accountconfig.Home(workspace, constants.RuntimeIDCodex), "auth.json"), data); err != nil {
		t.Fatal(err)
	}
	release()
	next := t.TempDir()
	_, nextRelease, err := service.Prepare(ctx, account.ID, constants.RuntimeIDCodex, next, "")
	if err != nil {
		t.Fatal(err)
	}
	defer nextRelease()
	data, err = os.ReadFile(filepath.Join(accountconfig.Home(next, constants.RuntimeIDCodex), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual codexLiveAuth
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Tokens.RefreshToken != "refresh-from-cli" {
		t.Fatal("new workspace reused stale refresh token")
	}
}

func TestSubscriptionProjectionFailureKeepsCredentialWithoutFalseOwnership(t *testing.T) {
	service, account, credential := subscriptionFixture(t)
	workspace := t.TempDir()
	home, err := accountconfig.EnsureHome(workspace, constants.RuntimeIDCodex)
	if err != nil {
		t.Fatal(err)
	}
	// 目录占位使原子替换失败，数据库仍应保留完整凭据且不认领错误的文件。
	if err := os.Mkdir(filepath.Join(home, "auth.json"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := service.prepareSubscription(ctx, &account, workspace, mustEncodeCredential(t, credential)); !errors.Is(err, status_error.AIAccountWorkspaceError) {
		t.Fatalf("expected projection failure, got %v", err)
	}
	saved, err := service.accounts.Get(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LiveAuthPath != "" || !saved.HasCredential {
		t.Fatal("failed projection claimed a native file or lost credentials")
	}
	if _, release, err := service.Prepare(ctx, account.ID, constants.RuntimeIDCodex, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
}

func mustEncodeCredential(t *testing.T, credential codexCredential) string {
	t.Helper()
	data, err := json.Marshal(credential)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCancelLoginInterruptsPollAndPreservesPreviousAccount(t *testing.T) {
	service, account, previous := subscriptionFixture(t)
	polling := make(chan struct{})
	serverDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/deviceauth/usercode":
			_, _ = w.Write([]byte(`{"device_auth_id":"device","user_code":"CODE","interval":1}`))
		case "/api/accounts/deviceauth/token":
			_, _ = io.Copy(io.Discard, r.Body)
			close(polling)
			select {
			case <-r.Context().Done():
			case <-serverDone:
			}
		default:
			t.Errorf("unexpected request after cancellation: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	defer close(serverDone)
	service.oauth = &codexoauth.Client{HTTP: server.Client(), Origin: server.URL}
	login, err := service.StartLogin(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-polling:
	case <-time.After(3 * time.Second):
		t.Fatal("Core did not start OAuth job without page polling")
	}
	service.CancelLogin(account.ID, login.ID)
	snapshot, err := service.GetLogin(context.Background(), account.ID, login.ID)
	if err != nil || snapshot.ErrorCode == "" {
		t.Fatalf("canceled job missing safe state: %v %v", snapshot, err)
	}
	secret, err := service.accounts.Credential(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret != mustEncodeCredential(t, previous) {
		t.Fatal("cancelled login replaced original credentials")
	}
}

func TestReauthenticationRejectsDifferentIdentity(t *testing.T) {
	service, account, previous := subscriptionFixture(t)
	tokens := subscriptionTokens("another-user", "other-refresh")
	jobContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.jobs[account.ID] = &loginJob{ctx: jobContext, cancel: cancel, revision: account.Revision, tokens: &tokens, view: Login{ID: "login", ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}}
	if _, err := service.advanceLogin(context.Background(), account.ID, service.jobs[account.ID]); !errors.Is(err, status_error.AIAccountLoginFailed) {
		t.Fatalf("different identity accepted: %v", err)
	}
	secret, err := service.accounts.Credential(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret != mustEncodeCredential(t, previous) {
		t.Fatal("failed reauthentication replaced credentials")
	}
}

func TestCoreLoginCompletesAndPublishesWithoutRendererPolling(t *testing.T) {
	service, account, _ := subscriptionFixture(t)
	hub := events.New()
	defer hub.Close()
	service.SetEventPublisher(hub)
	subscription := hub.Subscribe("account-login", events.Resource{Kind: "global"})
	defer subscription.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/deviceauth/usercode":
			_, _ = w.Write([]byte(`{"device_auth_id":"secret-device-identity","user_code":"CODE","interval":1}`))
		case "/api/accounts/deviceauth/token":
			_, _ = w.Write([]byte(`{"authorization_code":"secret-authorization-code","code_verifier":"secret-verifier"}`))
		case "/oauth/token":
			_ = json.NewEncoder(w).Encode(subscriptionTokens("user", "secret-new-refresh"))
		default:
			t.Errorf("unexpected OAuth route: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	service.oauth = &codexoauth.Client{HTTP: server.Client(), Origin: server.URL}
	login, err := service.StartLogin(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-subscription.Events:
			if strings.Contains(string(event.Payload), "secret-") {
				t.Fatal("event leaked server-only authentication data")
			}
			var payload struct {
				// Login 是安全授权事件里的可读进度。
				Login Login `json:"login"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if !payload.Login.Complete {
				continue
			}
			if event.Resource.ID != account.ID || event.Resource.ParentID != login.ID {
				t.Fatal("progress lost its account/job identity")
			}
			// 推送成功时凭据已提交，账号租约也已释放。
			release, err := service.Exclusive(account.ID)
			if err != nil {
				t.Fatalf("completed authorization retained lease: %v", err)
			}
			release()
			secret, err := service.accounts.Credential(context.Background(), account.ID)
			if err != nil || !strings.Contains(secret, "secret-new-refresh") {
				t.Fatalf("credentials not committed before push: %v", err)
			}
			return
		case <-deadline:
			t.Fatal("Core authorization required a Renderer poll to complete")
		}
	}
}

func TestLoginSnapshotsDoNotAdvanceOfficialPollingAndExpirationReleasesLease(t *testing.T) {
	service, account, previous := subscriptionFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/deviceauth/usercode":
			_, _ = w.Write([]byte(`{"device_auth_id":"device","user_code":"CODE","interval":1,"expires_in":1}`))
		case "/api/accounts/deviceauth/token":
			requests.Add(1)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected OAuth route: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	service.oauth = &codexoauth.Client{HTTP: server.Client(), Origin: server.URL}
	hub := events.New()
	defer hub.Close()
	service.SetEventPublisher(hub)
	subscription := hub.Subscribe("account-login", events.Resource{Kind: "global"})
	defer subscription.Close()
	login, err := service.StartLogin(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 20; index++ {
		snapshot, err := service.GetLogin(context.Background(), account.ID, login.ID)
		if err != nil || snapshot.ID != login.ID {
			t.Fatalf("snapshot=%v err=%v", snapshot, err)
		}
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-subscription.Events:
			var payload struct {
				// Login 是待验证的任务快照，不包含官方令牌。
				Login Login `json:"login"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Login.ErrorCode == "" {
				continue
			}
			if payload.Login.ErrorCode != status_error.OperationTimeout.Key() || requests.Load() != 1 {
				t.Fatalf("expired state=%+v official calls=%d", payload.Login, requests.Load())
			}
			release, err := service.Exclusive(account.ID)
			if err != nil {
				t.Fatal("expiration did not release account lease")
			}
			release()
			secret, err := service.accounts.Credential(context.Background(), account.ID)
			if err != nil || secret != mustEncodeCredential(t, previous) {
				t.Fatal("expiration changed existing credentials")
			}
			return
		case <-deadline:
			t.Fatal("expired login did not produce terminal state")
		}
	}
}
