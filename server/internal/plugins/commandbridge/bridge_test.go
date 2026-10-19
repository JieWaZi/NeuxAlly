package commandbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"os"
	"strings"
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
			params := map[string]any{"sessionId": session, "challengeId": "challenge-one", "actions": []any{map[string]any{"type": "copy-code", "value": "ONE"}}, "deadlineAt": time.Now().Add(time.Minute).UnixMilli()}
			if mode == "notice" {
				encoded, _ := json.Marshal(params)
				// 直接写原始 JSON，确保非法 UTF-8 不会被测试编码器替换。
				fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","method":"auth/challenge","params":%s,"notice":%s}}`+"\n", encoded[:len(encoded)-1], os.Getenv("NEUXALLY_NOTICE_JSON"))
			} else {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "method": "auth/challenge", "params": params})
			}
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

func TestBridgeChallengeNoticeIsPluginDeclared(t *testing.T) {
	t.Setenv("NEUXALLY_NOTICE_JSON", `{"messages":{"en":"Scan again to authorize permissions","zh-CN":"首次扫码完成后，还需再次扫码授权权限"}}`)
	var messages map[string]string
	err := testDriver(t, "notice").LoginSession(context.Background(), "session-one", nil, func(device authorization.Device) error {
		raw, _ := json.Marshal(device)
		var projected struct {
			// Notice 是通用授权提示协议。
			Notice *struct {
				// Messages 是插件声明的本地化文案。
				Messages map[string]string `json:"messages"`
			} `json:"notice"`
		}
		_ = json.Unmarshal(raw, &projected)
		if projected.Notice != nil {
			messages = projected.Notice.Messages
		}
		return nil
	})
	if err != nil || messages["zh-CN"] != "首次扫码完成后，还需再次扫码授权权限" || messages["en"] != "Scan again to authorize permissions" {
		t.Fatalf("plugin challenge notice missing: %+v %v", messages, err)
	}
}

func TestBridgeRejectsInvalidChallengeNotices(t *testing.T) {
	for name, raw := range map[string]string{
		"empty": `{}`, "null": `null`, "no-messages": `{"messages":{}}`,
		"unknown-locale":   `{"messages":{"fr":"Bonjour"}}`,
		"too-many-locales": `{"messages":{"en":"one","zh-CN":"二","fr":"trois"}}`,
		"blank":            `{"messages":{"en":" \n\t"}}`,
		"NUL":              `{"messages":{"en":"bad\u0000text"}}`,
		"too-long":         `{"messages":{"en":"` + strings.Repeat("x", 2049) + `"}}`,
		"invalid-UTF8":     "{\"messages\":{\"en\":\"" + string([]byte{255}) + "\"}}",
		"unknown-field":    `{"messages":{"en":"hello"},"tone":"warning"}`,
		"wrong-type":       `{"messages":{"en":2}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("NEUXALLY_NOTICE_JSON", raw)
			reported := false
			err := testDriver(t, "notice").LoginSession(context.Background(), "session-one", nil, func(authorization.Device) error { reported = true; return nil })
			if err == nil || reported {
				t.Fatalf("invalid notice reached a challenge: reported=%v error=%v", reported, err)
			}
		})
	}
}
