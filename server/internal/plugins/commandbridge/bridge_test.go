package commandbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"os"
	"testing"
	"time"
)

func testDriver(t *testing.T, mode string) *Driver {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{Command: binary, Args: []string{"-test.run=^TestBridgeProcess$"}, Directory: t.TempDir(), DataDirectory: t.TempDir(), PluginID: "plugin-one", Environment: append(os.Environ(), "NEUXALLY_BRIDGE_TEST="+mode)})
}

func TestBridgeProcess(t *testing.T) {
	mode := os.Getenv("NEUXALLY_BRIDGE_TEST")
	if mode == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			// ID 是输入请求序号。
			ID int `json:"id"`
			// Method 是桥接方法。
			Method string `json:"method"`
			// Params 保留宿主范围。
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			os.Exit(2)
		}
		reply := func(value any) {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": value})
		}
		switch request.Method {
		case "auth/status":
			reply(map[string]any{"state": "authorized", "account": map[string]any{"configured": true, "available": true, "id": "public-user", "scopes": []string{"documents:read"}}})
		case "auth/start":
			reply(map[string]any{"state": "pending"})
			session := request.Params["sessionId"]
			if mode == "forged" {
				session = "another-session"
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "method": "auth/challenge", "params": map[string]any{"sessionId": session, "challengeId": "challenge-one", "actions": []any{map[string]any{"type": "copy-code", "value": "ONE"}}, "deadlineAt": time.Now().Add(time.Minute).UnixMilli()}})
			if mode != "cancel" {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "method": "auth/state", "params": map[string]any{"sessionId": session, "state": "authorized"}})
			}
		case "auth/resolve":
			reply(map[string]any{"bindings": []any{map[string]any{"componentId": "not-granted", "environment": map[string]string{"TOKEN": "secret"}}}})
		case "auth/cancel":
			reply(map[string]string{"state": "cancelled"})
			os.Exit(0)
		default:
			fmt.Fprintln(os.Stderr, "unexpected")
			os.Exit(2)
		}
	}
	os.Exit(0)
}

func TestBridgeChallengesAreBoundToSessionAndStatusVerified(t *testing.T) {
	driver := testDriver(t, "ready")
	account, err := driver.Status(context.Background(), true)
	if err != nil || !account.Available || account.ID != "public-user" {
		t.Fatalf("status: %+v %v", account, err)
	}
	challenges := 0
	if err := driver.LoginSession(context.Background(), "session-one", nil, func(device authorization.Device) error {
		challenges++
		if device.ChallengeID != "challenge-one" || device.Actions[0].Value != "ONE" {
			t.Fatal("invalid challenge")
		}
		return nil
	}); err != nil || challenges != 1 {
		t.Fatalf("login: %d %v", challenges, err)
	}
	forged := testDriver(t, "forged")
	if err := forged.LoginSession(context.Background(), "session-one", nil, func(authorization.Device) error { t.Fatal("foreign challenge exposed"); return nil }); err == nil {
		t.Fatal("foreign session accepted")
	}
}

func TestBridgeCancellationStopsProcessAndRejectsSecretScopeEscape(t *testing.T) {
	driver := testDriver(t, "cancel")
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	err := driver.LoginSession(ctx, "session-one", nil, func(authorization.Device) error { cancel(); return nil })
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel did not stop bridge: %v", err)
	}
	if _, err := driver.Resolve(context.Background(), []Component{{ComponentID: "allowed", Key: "skills/a", Target: "skill"}}); err == nil {
		t.Fatal("ungranted secret binding accepted")
	}
}
