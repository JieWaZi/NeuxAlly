package plugins

import (
	"context"
	"errors"
	"fmt"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/feishu"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/github"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func pluginAPIContext(t *testing.T) *gin.Context {
	t.Helper()
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "plugins.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	previousPlugins, previousWakers, previousTx, previousDir := global.PluginControllerManager, global.WakerControllerManager, global.TransactionControllerManager, global.Config.DataDirectory
	global.PluginControllerManager = controllers.NewPluginController(database.DB)
	global.WakerControllerManager = controllers.NewWakerController(database.DB)
	global.TransactionControllerManager = controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB))
	global.Config.DataDirectory = t.TempDir()
	t.Cleanup(func() {
		global.PluginControllerManager = previousPlugins
		global.WakerControllerManager = previousWakers
		global.TransactionControllerManager = previousTx
		global.Config.DataDirectory = previousDir
		_ = sqlDB.Close()
	})
	ctx, _ := gin.CreateTestContext(nil)
	for _, id := range []string{"w1", "w2"} {
		if err := global.WakerControllerManager.Create(ctx, models.Waker{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	return ctx
}

func TestFirstInstallationRequiresConfigurationAndReportsPartnerFailures(t *testing.T) {
	ctx := pluginAPIContext(t)
	source := t.TempDir()
	content := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-tools","version":"1.0.0","extensions":{"com.ally":{"config":[{"key":"TOKEN","label":"访问密钥","secret":true,"required":true}]}}}`
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	scanned, err := (&ScanPlugin{Body: ScanPluginRequest{Location: source}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stage := scanned.(packageplugin.StagedPackage)
	prepared, err := (&PreparePlugin{Body: PreparePluginRequest{Token: stage.Token}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detail := prepared.(PluginDetailResponse)
	id := detail.Plugin.ID
	version := detail.Versions[0].ID
	request := InstallPluginRequest{VersionID: version, WakerIDs: []string{"w1", "missing"}, ExpectedConfigurationRevision: 0}
	if _, err := (&InstallPlugin{PluginID: id, Body: request}).Output(ctx); !errors.Is(err, status_error.PluginNotReady) {
		t.Fatalf("installed before configuration: %v", err)
	}
	relations, err := global.PluginControllerManager.Installations(ctx, id, "")
	if err != nil || len(relations) != 0 {
		t.Fatalf("failed auth created placeholder installation: %+v %v", relations, err)
	}
	if _, err := (&UpdatePluginConfiguration{PluginID: id, Body: UpdateConfigurationRequest{ExpectedRevision: 0, Secrets: []SecretValue{{Key: "TOKEN", Value: "private-token"}}}}).Output(ctx); err != nil {
		t.Fatal(err)
	}
	request.ExpectedConfigurationRevision = 1
	value, err := (&InstallPlugin{PluginID: id, Body: request}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result := value.(InstallPluginResponse)
	if len(result.Results) != 2 || !result.Results[0].Installed || result.Results[1].Installed || result.Results[1].ErrorCode == "" {
		t.Fatalf("partial results incorrect: %+v", result)
	}
	got, err := global.PluginControllerManager.Configuration(ctx, id)
	if err != nil || got.Secrets[0].Value != "private-token" {
		t.Fatalf("shared config unavailable: %+v %v", got, err)
	}
}

func TestConfigurationUpgradeIncludesDisabledUnselectedVersions(t *testing.T) {
	ctx := pluginAPIContext(t)
	store := global.PluginControllerManager
	if err := store.Create(ctx, models.Plugin{ID: "p", Name: "Plugin", SourceKey: "source"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"old", "new"} {
		root := t.TempDir()
		manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"tools","version":"` + name + `","extensions":{"com.ally":{"config":[{"key":"` + name + `","required":true}]}}}`
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		staged, err := packageplugin.Stage(t.TempDir(), root)
		if err != nil {
			t.Fatal(err)
		}
		content, err := packageplugin.CopyContent(filepath.Join(global.Config.DataDirectory, "plugins", "content"), staged.Root, staged.Digest)
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(global.Config.DataDirectory, content)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PutVersion(ctx, models.PluginVersion{ID: name, PluginID: "p", Version: name, Digest: staged.Digest, ContentPath: filepath.ToSlash(relative), Package: staged.Package}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpdateConfiguration(ctx, models.PluginConfiguration{PluginID: "p", Values: map[string]string{"old": "value"}}, 0); err != nil {
		t.Fatal(err)
	}
	for _, waker := range []string{"w1", "w2"} {
		if err := store.Install(ctx, models.WakerPlugin{ID: waker, PluginID: "p", VersionID: "old", WakerID: waker, Enabled: waker == "w1"}); err != nil {
			t.Fatal(err)
		}
	}
	body := UpdateConfigurationRequest{ExpectedRevision: 1, Values: map[string]string{"new": "value"}, Switches: []VersionSwitch{{WakerID: "w1", ExpectedVersionID: "old", VersionID: "new"}}}
	if _, err := (&UpdatePluginConfiguration{PluginID: "p", Body: body}).Output(ctx); !errors.Is(err, status_error.PluginConfigurationConflict) {
		t.Fatalf("incompatible unselected version ignored: %v", err)
	}
	got, err := store.Configuration(ctx, "p")
	if err != nil || got.Revision != 1 || got.Values["old"] != "value" {
		t.Fatalf("failed upgrade changed config: %+v %v", got, err)
	}
	body.Switches = append(body.Switches, VersionSwitch{WakerID: "w2", ExpectedVersionID: "old", VersionID: "new"})
	if _, err := (&UpdatePluginConfiguration{PluginID: "p", Body: body}).Output(ctx); err != nil {
		t.Fatal(err)
	}
	relations, err := store.Installations(context.Background(), "p", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range relations {
		if r.VersionID != "new" {
			t.Fatalf("partial upgrade: %+v", relations)
		}
	}
}

func TestPrepareMCPPluginRequiresExplicitContentTrust(t *testing.T) {
	ctx := pluginAPIContext(t)
	root := t.TempDir()
	for name, content := range map[string]string{"plugin.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"mcp-tools"}`, "mcp.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"tool":{"type":"stdio","command":"node"}}}`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	value, err := (&ScanPlugin{Body: ScanPluginRequest{Location: root}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	token := value.(packageplugin.StagedPackage).Token
	if _, err := (&PreparePlugin{Body: PreparePluginRequest{Token: token}}).Output(ctx); !errors.Is(err, status_error.MCPConnectorTrustRequired) {
		t.Fatalf("untrusted MCP content accepted: %v", err)
	}
	if _, err := (&PreparePlugin{Body: PreparePluginRequest{Token: token, TrustConfirmed: true}}).Output(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestInstallRejectsMutatedManagedVersion(t *testing.T) {
	ctx := pluginAPIContext(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"integrity"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := (&ScanPlugin{Body: ScanPluginRequest{Location: root}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	value, err = (&PreparePlugin{Body: PreparePluginRequest{Token: value.(packageplugin.StagedPackage).Token}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detail := value.(PluginDetailResponse)
	version := detail.Versions[0]
	if err := os.WriteFile(filepath.Join(global.Config.DataDirectory, version.ContentPath, "injected.sh"), []byte("arbitrary mutation"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = (&InstallPlugin{PluginID: detail.Plugin.ID, Body: InstallPluginRequest{VersionID: version.ID, WakerIDs: []string{"w1"}}}).Output(ctx)
	if !errors.Is(err, status_error.PluginVersionConflict) {
		t.Fatalf("mutated managed content accepted: %v", err)
	}
}

func TestLocalManifestRepositoryCannotMergeIntoAnotherResource(t *testing.T) {
	ctx := pluginAPIContext(t)
	ids := []string{}
	for _, version := range []string{"1.0.0", "2.0.0"} {
		source := t.TempDir()
		manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"office","repository":"https://example.invalid/official","version":"` + version + `"}`
		if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		stage, err := packageplugin.Stage(stagedRoot(), source)
		if err != nil {
			t.Fatal(err)
		}
		result, err := (&PreparePlugin{Body: PreparePluginRequest{Token: stage.Token}}).Output(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, result.(PluginDetailResponse).Plugin.ID)
	}
	if ids[0] == ids[1] {
		t.Fatal("self-declared repository merged unverified local content into an existing resource")
	}
}

func TestSupplementalAuthorizationScopesMustBeExplicitIdentifiers(t *testing.T) {
	for _, scope := range []string{"", "scope with spaces", "scope\n--domain all", "permission;command"} {
		operator := StartPluginAuthorization{Body: AuthorizationScopeRequest{AdditionalScopes: []string{scope}}}
		if err := operator.Validate(nil); err == nil {
			t.Fatalf("invalid supplemental scope accepted: %q", scope)
		}
	}
	if err := (&StartPluginAuthorization{Body: AuthorizationScopeRequest{AdditionalScopes: []string{"calendar:calendar.event:create", "task:task:write"}}}).Validate(nil); err != nil {
		t.Fatal(err)
	}
}

func TestVersionSwitchRejectsUnreadyTargetsWithoutChangingInstallation(t *testing.T) {
	for _, kind := range []string{"missing-component", "changed-content", "unavailable-account"} {
		t.Run(kind, func(t *testing.T) {
			ctx := pluginAPIContext(t)
			prepare := func(id, version, extension string) PluginDetailResponse {
				root := t.TempDir()
				body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"upgrade","version":"` + version + `","extensions":{"com.ally":` + extension + `}}`
				if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				scan, err := (&ScanPlugin{Body: ScanPluginRequest{Location: root}}).Output(ctx)
				if err != nil {
					t.Fatal(err)
				}
				value, err := (&PreparePlugin{Body: PreparePluginRequest{Token: scan.(packageplugin.StagedPackage).Token, PluginID: id}}).Output(ctx)
				if err != nil {
					t.Fatal(err)
				}
				return value.(PluginDetailResponse)
			}
			old := prepare("", "1.0.0", `{}`)
			oldID := old.Versions[0].ID
			if _, err := (&InstallPlugin{PluginID: old.Plugin.ID, Body: InstallPluginRequest{VersionID: oldID, WakerIDs: []string{"w1"}}}).Output(ctx); err != nil {
				t.Fatal(err)
			}
			extension := `{}`
			if kind == "missing-component" {
				extension = `{"requiredComponents":["missing"]}`
			} else if kind == "unavailable-account" {
				extension = `{"provider":"feishu"}`
			}
			next := prepare(old.Plugin.ID, "2.0.0", extension)
			var target models.PluginVersion
			for _, v := range next.Versions {
				if v.Version == "2.0.0" {
					target = v
				}
			}
			if target.ID == "" {
				t.Fatal("target missing")
			}
			if kind == "changed-content" {
				root, err := packageplugin.ContentPath(global.Config.DataDirectory, target.ContentPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := (&UpdatePluginConfiguration{PluginID: old.Plugin.ID, Body: UpdateConfigurationRequest{ExpectedRevision: 0, Switches: []VersionSwitch{{WakerID: "w1", ExpectedVersionID: oldID, VersionID: target.ID}}}}).Output(ctx)
			if err == nil {
				t.Error("unready version replaced working installation")
			}
			installs, err := global.PluginControllerManager.Installations(ctx, old.Plugin.ID, "")
			if err != nil || len(installs) != 1 || installs[0].VersionID != oldID {
				t.Errorf("failed switch changed version: %+v %v", installs, err)
			}
			config, err := global.PluginControllerManager.Configuration(ctx, old.Plugin.ID)
			if err != nil || config.Revision != 0 {
				t.Errorf("failed switch changed config: %+v %v", config, err)
			}
		})
	}
}

// 防止授权范围被飞书固定值污染，并验证 GitHub 登录前后使用同一个安装准入。
func TestGitHubAuthorizationAndInstallationReadiness(t *testing.T) {
	ctx := pluginAPIContext(t)
	previous := global.PluginAuthorizations
	t.Cleanup(func() { global.PluginAuthorizations = previous })
	profile := t.TempDir()
	binary := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(binary, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	ready := false
	githubChecks := 0
	auth := github.New(github.Config{Binary: binary, ProfileDirectory: profile, Run: func(_ context.Context, args []string, _ string, out, _ io.Writer) error {
		githubChecks++
		_, _ = fmt.Fprintf(out, `{"hosts":{"github.com":[{"state":%q,"active":true,"host":"github.com","login":"octocat","tokenSource":%q,"scopes":"repo, read:org, gist","gitProtocol":"https"}]}}`, map[bool]string{true: "success", false: "error"}[ready], filepath.Join(profile, "hosts.yml"))
		return nil
	}})
	global.PluginAuthorizations = map[string]*authorization.Authorizer{"github": auth}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "github"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "github", "SKILL.md"), []byte("---\nname: github\ndescription: GitHub operations\n---\nUse official GitHub tools."), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"github","version":"1.0.0","extensions":{"com.ally":{"provider":"github","scopes":["repo"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	stage, err := packageplugin.Stage(t.TempDir(), root)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := prepareResource(ctx, stage, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	value, err := (&GetPluginAuthorization{PluginID: detail.Plugin.ID}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state := value.(PluginAuthorizationResponse)
	if state.Provider != "github" || len(state.RequiredScopes) != 3 || slices.Contains(state.RequiredScopes, "offline_access") || !state.Account.Configured || state.Account.Available {
		t.Fatalf("wrong github auth: %+v", state)
	}
	request := &InstallPlugin{PluginID: detail.Plugin.ID, Body: InstallPluginRequest{VersionID: detail.ResolvedVersionID, WakerIDs: []string{"w1", "w2"}}}
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginNotReady) {
		t.Fatalf("installed before auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profile, "hosts.yml"), []byte("github.com: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ready = true
	installed, err := request.Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	results := installed.(InstallPluginResponse).Results
	if len(results) != 2 || !results[0].Installed || !results[1].Installed {
		t.Fatalf("shared install: %+v", results)
	}

	feishuProfile := t.TempDir()
	if err := os.WriteFile(filepath.Join(feishuProfile, "config.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	feishuChecks := 0
	global.PluginAuthorizations["feishu"] = feishu.New(feishu.Config{Binary: binary, ProfileDirectory: feishuProfile, Run: func(context.Context, []string, string) ([]byte, error) {
		feishuChecks++
		return []byte(`{"appId":"app","identities":{"user":{"available":true,"openId":"feishu-user","scope":"offline_access"}}}`), nil
	}})
	manifest, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(strings.ReplaceAll(string(manifest), `"github"`, `"feishu"`)), 0600); err != nil {
		t.Fatal(err)
	}
	stage, err = packageplugin.Stage(t.TempDir(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareResource(ctx, stage, "", "", true); err != nil {
		t.Fatal(err)
	}
	githubChecks = 0
	feishuChecks = 0
	listed, err := listPluginResources(ctx, "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Resources) != 2 || githubChecks != 1 || feishuChecks != 1 {
		t.Fatalf("provider query cache mixed identities: github=%d feishu=%d list=%+v", githubChecks, feishuChecks, listed)
	}
	identities := map[string]string{}
	for _, resource := range listed.Resources {
		identities[resource.Account.ID] = resource.Plugin.ID
	}
	if identities["octocat"] == "" || identities["feishu-user"] == "" {
		t.Fatalf("provider identity leaked between resources: %+v", identities)
	}
}
