package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
)

func TestAuthorizationReusesPendingSessionAndVerifiesFinalIdentity(t *testing.T) {
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	completed := make(chan struct{})
	changed := make(chan authorization.Session, 8)
	var starts atomic.Int32
	var ready atomic.Bool
	auth := New(Config{ProfileDirectory: profile, Scopes: []string{"docx:document"}, Changed: func(s authorization.Session) { changed <- s }, Run: func(ctx context.Context, args []string, stdin string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--no-wait") {
			starts.Add(1)
			return []byte(`{"verification_url":"https://example.com/auth?code=a%2Bb","device_code":"private-device","expires_in":300}`), nil
		}
		if strings.Contains(joined, "--device-code") {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-completed:
				ready.Store(true)
				return []byte(`{"event":"authorization_complete","missing":[]}`), nil
			}
		}
		if strings.HasPrefix(joined, "auth status") {
			if ready.Load() {
				return []byte(`{"appId":"app","identities":{"user":{"available":true,"verified":true,"openId":"user","userName":"测试用户","scope":"offline_access docx:document"}}}`), nil
			}
			return []byte(`{"appId":"app","identities":{"user":{"available":false}}}`), nil
		}
		return nil, status_error.PluginAuthorizationFailed
	}})
	auth.Open(context.Background())
	defer auth.Close()
	session, err := auth.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	again, err := auth.Login(context.Background())
	if err != nil || again.ID != session.ID || starts.Load() != 1 {
		t.Fatalf("pending auth duplicated: %+v %+v %v", session, again, err)
	}
	encoded, err := json.Marshal(session)
	if err != nil || strings.Contains(string(encoded), "private-device") {
		t.Fatalf("private code leaked: %s %v", encoded, err)
	}
	if session.VerificationURL != "https://example.com/auth?code=a%2Bb" {
		t.Fatal("authorization URL changed")
	}
	close(completed)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-changed:
			if event.State == constants.PluginAuthorizationStateReady {
				if event.Account.Name != "测试用户" || event.VerificationURL != "" {
					t.Fatalf("unverified result: %+v", event)
				}
				return
			}
		case <-deadline:
			t.Fatal("authorization did not finish")
		}
	}
}

func TestAuthorizationAccountChangesAreExclusiveWithActiveWorkAndCancelStopsWait(t *testing.T) {
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	started := make(chan struct{})
	auth := New(Config{ProfileDirectory: profile, Run: func(ctx context.Context, args []string, stdin string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--no-wait") {
			return []byte(`{"verification_url":"https://example.com/auth","device_code":"private","expires_in":300}`), nil
		}
		if strings.Contains(joined, "--device-code") {
			close(started)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		}
		return []byte(`{"appId":"app","identities":{"user":{"available":false}}}`), nil
	}})
	auth.Open(context.Background())
	defer auth.Close()
	release, err := auth.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.Configure(context.Background(), "app", "secret"); !errors.Is(err, status_error.PluginBusy) {
		t.Fatalf("changed active identity: %v", err)
	}
	if _, err := auth.Login(context.Background()); !errors.Is(err, status_error.PluginBusy) {
		t.Fatalf("login changed active identity: %v", err)
	}
	release()
	session, err := auth.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := auth.Cancel(session.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop CLI wait")
	}
	if auth.Snapshot().State != constants.PluginAuthorizationStateCancelled {
		t.Fatalf("cancel state overwritten: %+v", auth.Snapshot())
	}
}

func TestActiveUsageIsVisibleAndReleasedWithExecution(t *testing.T) {
	auth := New(Config{})
	release, err := auth.Acquire(authorization.Usage{ChatID: "chat", ExecutionID: "execution", WakerID: "partner", Title: "整理文档"})
	if err != nil {
		t.Fatal(err)
	}
	uses := auth.Usages()
	if len(uses) != 1 || uses[0].ExecutionID != "execution" {
		t.Fatalf("active work missing: %+v", uses)
	}
	uses[0].Title = "changed"
	if auth.Usages()[0].Title != "整理文档" {
		t.Fatal("caller mutated authorization usage state")
	}
	release()
	release()
	if len(auth.Usages()) != 0 {
		t.Fatal("finished work still blocks account")
	}
}

func TestCancelledDeviceProcessKeepsIdentityExclusiveUntilExit(t *testing.T) {
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	started, cancelled, finish := make(chan struct{}), make(chan struct{}), make(chan struct{})
	auth := New(Config{ProfileDirectory: profile, Run: func(ctx context.Context, args []string, _ string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--no-wait") {
			select {
			case <-started:
				return nil, status_error.PluginAuthorizationFailed
			default:
			}
			return []byte(`{"verification_url":"https://example.com/auth","device_code":"private","expires_in":300}`), nil
		}
		if strings.Contains(joined, "--device-code") {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-finish
			return nil, ctx.Err()
		}
		return []byte(`{"appId":"app","identities":{"user":{"available":false}}}`), nil
	}})
	auth.Open(context.Background())
	defer auth.Close()
	finished := false
	defer func() {
		if !finished {
			close(finish)
		}
	}()
	session, err := auth.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := auth.Cancel(session.ID); err != nil {
		t.Fatal(err)
	}
	<-cancelled
	if release, err := auth.Acquire(); !errors.Is(err, status_error.PluginBusy) {
		if release != nil {
			release()
		}
		t.Errorf("accepted work while old login is alive: %v", err)
	}
	if err := auth.Configure(context.Background(), "new-app", "new-secret"); !errors.Is(err, status_error.PluginBusy) {
		t.Errorf("changed app while old login is alive: %v", err)
	}
	if err := auth.Logout(context.Background()); !errors.Is(err, status_error.PluginBusy) {
		t.Errorf("logged out while old login is alive: %v", err)
	}
	if _, err := auth.Login(context.Background()); !errors.Is(err, status_error.PluginBusy) {
		t.Errorf("started another login while old login is alive: %v", err)
	}
	close(finish)
	finished = true
	auth.Close()
	release, err := auth.Acquire()
	if err != nil {
		t.Fatal("identity remained busy after old process exited", err)
	}
	release()

}

func TestCommandRunnerUsesOnlyManagedProviderEnvironment(t *testing.T) {
	t.Setenv("NEUXALLY_TEST_PROVIDER_ENV", "1")
	t.Setenv("LARKSUITE_CLI_PROFILE", "external")
	t.Setenv("LARKSUITE_CLI_APP_ID", "external-app")
	t.Setenv("LARKSUITE_CLI_USER_ACCESS_TOKEN", "external-token")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output, err := commandRunner(binary, t.TempDir())(context.Background(), []string{"-test.run=^TestProviderEnvironmentChild$"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if err := json.Unmarshal(output, &env); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"LARKSUITE_CLI_PROFILE", "LARKSUITE_CLI_APP_ID", "LARKSUITE_CLI_USER_ACCESS_TOKEN"} {
		if env[key] != "" {
			t.Errorf("inherited external identity %s", key)
		}
	}
	if env["LARKSUITE_CLI_CONFIG_DIR"] == "" {
		t.Fatal("managed profile absent")
	}
}
func TestProviderEnvironmentChild(t *testing.T) {
	if os.Getenv("NEUXALLY_TEST_PROVIDER_ENV") != "1" {
		return
	}
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "LARKSUITE_CLI_") {
			values[key] = value
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(values)
	os.Exit(0)
}

func TestExecutionEnvironmentClearsHostAndRuntimeIdentityOverrides(t *testing.T) {
	t.Setenv("LARKSUITE_CLI_PROFILE", "host-profile")
	got := ManagedEnvironment("/managed/profile", map[string]string{"LARKSUITE_CLI_APP_ID": "runtime-app", "LARKSUITE_CLI_CONFIG_DIR": "/external", "PATH": "/runtime/bin"})
	if got["LARKSUITE_CLI_PROFILE"] != "" || got["LARKSUITE_CLI_APP_ID"] != "" || got["LARKSUITE_CLI_CONFIG_DIR"] != "/managed/profile" || got["PATH"] != "/runtime/bin" {
		t.Fatalf("managed environment did not retain profile authority: %+v", got)
	}
}
