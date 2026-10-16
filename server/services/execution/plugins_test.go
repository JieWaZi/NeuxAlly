package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/feishu"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// installedPluginFixture 只替换资源读取端口，内容解析和执行解析走真实实现。
type installedPluginFixture struct {
	// PluginController 保留本测试未消费的控制器端口。
	controllers.PluginController
	// states 是已固定关系和配置的同一事务快照。
	states []models.PluginState
}

func (f installedPluginFixture) InstalledStates(context.Context, string) ([]models.PluginState, error) {
	return f.states, nil
}

func executionPlugin(t *testing.T, data string, id string, skillText string, server bool) models.PluginState {
	t.Helper()
	source := t.TempDir()
	manifest := map[string]any{"$schema": constants.PluginManifestSchema, "name": "office", "version": "1.0.0"}
	if server {
		manifest["extensions"] = map[string]any{constants.PluginExtensionNamespace: map[string]any{"bindings": []any{map[string]any{"server": "office", "target": "env", "name": "API_TOKEN", "value": "${TOKEN}"}}}}
	}
	body, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if skillText != "" {
		if err := os.MkdirAll(filepath.Join(source, "skills", "office"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "skills", "office", "SKILL.md"), []byte("---\nname: office\ndescription: Office documents\n---\n"+skillText), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if server {
		if err := os.WriteFile(filepath.Join(source, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"office":{"type":"stdio","command":"native-tool","args":["a b"],"env":{}}}}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stage, err := plugins.Stage(filepath.Join(data, "plugins", "staging"), source)
	if err != nil {
		t.Fatal(err)
	}
	content, err := plugins.CopyContent(filepath.Join(data, "plugins", "content"), stage.Root, stage.Digest)
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(data, content)
	if err != nil {
		t.Fatal(err)
	}
	version := models.PluginVersion{ID: id + "-v1", PluginID: id, Version: "1.0.0", ContentPath: relative, Digest: stage.Digest, Package: stage.Package, TrustedAt: 1}
	return models.PluginState{Plugin: models.Plugin{ID: id, Name: "办公"}, Versions: []models.PluginVersion{version}, Installations: []models.WakerPlugin{{WakerID: "w1", PluginID: id, VersionID: version.ID, Enabled: true}}, Configuration: models.PluginConfiguration{PluginID: id, Revision: 3, Values: map[string]string{}, Secrets: []models.MCPSecretDefinition{{Name: "TOKEN", Value: "private-token", Configured: true}}}}
}

func TestPluginAdmissionPinsResourcesAndDoesNotSelectEveryComponent(t *testing.T) {
	data := t.TempDir()
	state := executionPlugin(t, data, "p1", "use office", true)
	s := &service{pluginController: installedPluginFixture{states: []models.PluginState{state}}, skillDataDirectory: data, pluginLauncher: "/core"}
	detection := runtimes.DetectionResult{State: runtimes.DetectionStateReady, Capabilities: runtimes.CapabilitySet{runtimes.CapabilitySkillDirectory: true}}
	prepared, err := s.preparePlugins(context.Background(), "w1", pendingMessage{message: userMessage{PluginIDs: []string{"p1"}}}, detection)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.release()
	if len(prepared.skills) != 1 || prepared.skills[0].Selected || len(prepared.connectors) != 1 {
		t.Fatalf("components: %+v", prepared)
	}
	connector := prepared.connectors[0]
	if connector.Command != "/core" || connector.Arguments[0] != constants.PluginStdioMode || connector.RuntimeKey != "plugin_"+connector.ID {
		t.Fatalf("stdio launch: %+v", connector)
	}
	if len(prepared.snapshots) != 1 || prepared.snapshots[0].VersionID != "p1-v1" || prepared.snapshots[0].ConfigurationRevision != 3 || len(prepared.selected) != 1 {
		t.Fatalf("snapshot: %+v", prepared)
	}
	body, _ := json.Marshal(prepared.snapshots)
	if strings.Contains(string(body), "private-token") {
		t.Fatal("snapshot leaked credential")
	}
	state.Installations[0].Enabled = false
	if _, err = s.preparePlugins(context.Background(), "w1", pendingMessage{message: userMessage{PluginIDs: []string{"p1"}}}, detection); !errors.Is(err, status_error.PluginNotReady) {
		t.Fatalf("disabled explicit reference accepted: %v", err)
	}
}

func TestPluginSkillCollisionDeduplicatesOnlyEqualContent(t *testing.T) {
	data := t.TempDir()
	a := executionPlugin(t, data, "a", "first", false)
	a.Configuration.Secrets = nil
	b := executionPlugin(t, data, "b", "second", false)
	b.Configuration.Secrets = nil
	s := &service{pluginController: installedPluginFixture{states: []models.PluginState{a, b}}, skillDataDirectory: data}
	detection := runtimes.DetectionResult{State: runtimes.DetectionStateReady, Capabilities: runtimes.CapabilitySet{runtimes.CapabilitySkillDirectory: true}}
	result, err := s.preparePlugins(context.Background(), "w1", pendingMessage{}, detection)
	if err != nil {
		t.Fatal(err)
	}
	defer result.release()
	if _, err = mergePluginSkills(nil, result.skills); !errors.Is(err, status_error.PluginSkillConflict) {
		t.Fatalf("different same-name skills accepted: %v", err)
	}
	if merged, err := mergePluginSkills(nil, append(result.skills[:1], result.skills[0])); err != nil || len(merged) != 1 {
		t.Fatalf("identical skills not deduplicated: %v %v", merged, err)
	}
}

func TestPluginRetryRejectsChangedConfiguration(t *testing.T) {
	data := t.TempDir()
	state := executionPlugin(t, data, "p1", "use office", false)
	state.Configuration.Secrets = nil
	s := &service{pluginController: installedPluginFixture{states: []models.PluginState{state}}, skillDataDirectory: data}
	_, err := s.preparePlugins(context.Background(), "w1", pendingMessage{retry: true, pluginSnapshots: []plugins.Snapshot{{ID: "p1", VersionID: "p1-v1", ConfigurationRevision: 2}}}, runtimes.DetectionResult{})
	if !errors.Is(err, status_error.PluginConfigurationConflict) {
		t.Fatalf("retry silently changed config: %v", err)
	}
}

func TestPluginRestrictedEntryFiltersMCPAndRejectsProviderCLI(t *testing.T) {
	data := t.TempDir()
	state := executionPlugin(t, data, "p1", "use office", true)
	s := &service{pluginController: installedPluginFixture{states: []models.PluginState{state}}, skillDataDirectory: data, pluginLauncher: "/core"}
	detection := runtimes.DetectionResult{State: runtimes.DetectionStateReady, Capabilities: runtimes.CapabilitySet{runtimes.CapabilitySkillDirectory: true}}
	empty := []string{}
	result, err := s.preparePlugins(context.Background(), "w1", pendingMessage{allowedConnectorIDs: &empty}, detection)
	if err != nil {
		t.Fatal(err)
	}
	result.release()
	if len(result.connectors) != 0 || len(result.skills) != 1 {
		t.Fatalf("MCP whitelist bypass: %+v", result)
	}
	extension := json.RawMessage(`{"provider":"feishu"}`)
	state.Versions[0].Package.Manifest.Extensions = map[string]json.RawMessage{constants.PluginExtensionNamespace: extension}
	state.Configuration.Secrets = nil
	s.pluginController = installedPluginFixture{states: []models.PluginState{state}}
	result, err = s.preparePlugins(context.Background(), "w1", pendingMessage{allowedConnectorIDs: &empty}, detection)
	if err != nil {
		t.Fatal(err)
	}
	result.release()
	if len(result.skills) > 0 || len(result.environment) > 0 {
		t.Fatal("restricted entry received CLI skills or provider environment")
	}
	if _, err = s.preparePlugins(context.Background(), "w1", pendingMessage{allowedConnectorIDs: &empty, message: userMessage{PluginIDs: []string{"p1"}}}, detection); !errors.Is(err, status_error.PluginForbidden) {
		t.Fatalf("explicit provider bypass: %v", err)
	}
}

func TestPluginIntentAndVersionSurviveAdmissionAndHistory(t *testing.T) {
	data := t.TempDir()
	state := executionPlugin(t, data, "p1", "", true)
	state.Installations[0].WakerID = "waker-1"
	runtime := &fakeRuntime{}
	manager := newMessageTestService(t, t.TempDir(), runtime, messageTestOptions{plugins: installedPluginFixture{states: []models.PluginState{state}}, skillDataDirectory: data})
	manager.execution.(*service).pluginLauncher = "/core"
	intent := WorkIntent{ClientRequestID: "plugin-intent", Chat: ChatTarget{Create: &CreateChatSpec{WakerID: "waker-1", WorkingDirectory: t.TempDir()}}, Input: WorkInput{Actor: "user", Content: "整理文档", PluginIDs: []string{"p1"}}, Options: WorkOptions{RuntimeID: runtimes.RuntimeIDCodex, PermissionLevel: runtimes.PermissionLevelDefault}}
	accepted, err := manager.execution.Start(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	waitForChatTerminal(t, manager, accepted.Chat.ID)
	snapshot, err := manager.execution.(*service).journal.LoadExecutionContext(context.Background(), accepted.Chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PreviousMessages) < 1 || len(snapshot.PreviousMessages[0].Plugins) != 1 || snapshot.PreviousMessages[0].Plugins[0].VersionID != "p1-v1" {
		t.Fatalf("history lost plugin snapshot: %+v", snapshot.PreviousMessages)
	}
	records, err := manager.execution.(*service).journal.ReadRecords(accepted.Chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	recorded, _, err := executionFromRecords(records, accepted.Execution.ID)
	if err != nil || len(recorded.PluginResources) != 1 || recorded.PluginResources[0].ConfigurationRevision != 3 {
		t.Fatalf("execution lost immutable resources: %+v %v", recorded, err)
	}
	replay, err := manager.execution.Start(context.Background(), intent)
	if err != nil || replay.Chat.ID != accepted.Chat.ID {
		t.Fatalf("retry mismatch: %+v %v", replay, err)
	}
	intent.Input.PluginIDs = []string{"changed"}
	if _, err = manager.execution.Start(context.Background(), intent); !errors.Is(err, status_error.CreateRequestConflict) {
		t.Fatalf("plugin intent absent from idempotency hash: %v", err)
	}
	if len(runtime.requests) != 1 || !strings.Contains(runtime.requests[0].Message.Text, "p1") {
		t.Fatalf("explicit intent missing: %+v", runtime.requests)
	}
}

func TestSharedProviderIsVerifiedOnceAndIdentityParticipatesInExecutionEnvironment(t *testing.T) {
	data := t.TempDir()
	a := executionPlugin(t, data, "a", "use office", false)
	b := executionPlugin(t, data, "b", "use office", false)
	extension := json.RawMessage(`{"provider":"feishu","scopes":["scope-a"]}`)
	for _, state := range []*models.PluginState{&a, &b} {
		state.Configuration.Secrets = nil
		state.Versions[0].Package.Manifest.Extensions = map[string]json.RawMessage{constants.PluginExtensionNamespace: extension}
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "config.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	identity := "user-1"
	checks := 0
	authorizer := feishu.New(feishu.Config{Binary: filepath.Join(t.TempDir(), "lark-cli"), ProfileDirectory: profile, Run: func(_ context.Context, args []string, _ string) ([]byte, error) {
		if len(args) > 1 && args[0] == "auth" && args[1] == "status" {
			checks++
			return []byte(`{"appId":"app","identities":{"user":{"available":true,"verified":true,"openId":"` + identity + `","scope":"scope-a"}}}`), nil
		}
		return nil, nil
	}})
	service := &service{pluginController: installedPluginFixture{states: []models.PluginState{a, b}}, skillDataDirectory: data, pluginAuthorizer: authorizer}
	detection := runtimes.DetectionResult{State: runtimes.DetectionStateReady, Capabilities: runtimes.CapabilitySet{runtimes.CapabilitySkillDirectory: true}}
	first, err := service.preparePlugins(context.Background(), "w1", pendingMessage{}, detection)
	if err != nil {
		t.Fatal(err)
	}
	if checks != 1 || len(first.snapshots) != 2 || first.environment["NEUXALLY_FEISHU_IDENTITY"] != "app:user-1" {
		t.Fatalf("provider snapshot not shared: checks=%d %+v", checks, first)
	}
	if err := authorizer.Logout(context.Background()); !errors.Is(err, status_error.PluginBusy) {
		t.Fatalf("active identity changed: %v", err)
	}
	first.release()
	identity = "user-2"
	second, err := service.preparePlugins(context.Background(), "w1", pendingMessage{}, detection)
	if err != nil {
		t.Fatal(err)
	}
	defer second.release()
	if first.environment["NEUXALLY_FEISHU_IDENTITY"] == second.environment["NEUXALLY_FEISHU_IDENTITY"] || first.snapshots[0].ProviderAccountID != "user-1" {
		t.Fatal("new account reused old environment or mutated frozen snapshot")
	}
}

func TestDeduplicatedPluginSkillKeepsLeasedImmutableSource(t *testing.T) {
	data := t.TempDir()
	state := executionPlugin(t, data, "p1", "use office", false)
	version := state.Versions[0]
	root, err := plugins.ContentPath(data, version.ContentPath)
	if err != nil {
		t.Fatal(err)
	}
	derived := runtimes.Skill{Name: "office", Path: filepath.Join(root, "skills", "office")}
	content, err := os.ReadFile(filepath.Join(derived.Path, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	independent := t.TempDir()
	if err := os.WriteFile(filepath.Join(independent, "SKILL.md"), content, 0600); err != nil {
		t.Fatal(err)
	}
	merged, err := mergePluginSkills([]runtimes.Skill{{Name: "office", Path: independent, Selected: true}}, []runtimes.Skill{derived})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || !merged[0].Selected || merged[0].Path != derived.Path {
		t.Fatalf("dedup lost selection or immutable path: %+v", merged)
	}
	if err := os.RemoveAll(independent); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(merged[0].Path, "SKILL.md")); err != nil {
		t.Fatal("independent resource deletion broke plugin snapshot", err)
	}
}

func TestIndependentSkillDuplicatesUseTheSameContentRule(t *testing.T) {
	makeSkill := func(body string) runtimes.Skill {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return runtimes.Skill{Name: "same", Path: root}
	}
	first, second := makeSkill("same body"), makeSkill("same body")
	second.Selected = true
	merged, err := mergePluginSkills([]runtimes.Skill{first, second}, nil)
	if err != nil || len(merged) != 1 || !merged[0].Selected {
		t.Errorf("same independent content did not merge: %+v %v", merged, err)
	}
	if _, err := mergePluginSkills([]runtimes.Skill{first, makeSkill("different body")}, nil); !errors.Is(err, status_error.PluginSkillConflict) {
		t.Errorf("conflicting independent content admitted: %v", err)
	}
}
