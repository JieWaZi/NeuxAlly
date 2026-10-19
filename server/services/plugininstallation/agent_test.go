package plugininstallation

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func TestAllyCLIInstallsThroughExecutionGrantAndReplaysSameReceipt(t *testing.T) {
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(root, "plugins.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	store := controllers.NewPluginController(database.DB)
	wakers := controllers.NewWakerController(database.DB)
	for _, id := range []string{"current-partner", "another-partner"} {
		if err := wakers.Create(context.Background(), models.Waker{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range map[string]string{"neuxally-plugin/demo/.codex-plugin/plugin.json": `{"name":"demo","version":"2.0.0"}`, "neuxally-plugin/demo/skills/probe/SKILL.md": "---\nname: probe\ndescription: Test installation\n---\nProbe " + root} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive.Bytes())
	downloads := 0
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/catalog.json" {
			fmt.Fprintf(w, `{"schemaVersion":2,"entries":[{"key":"demo","sourceCommit":"1111111111111111111111111111111111111111","archiveURL":%q,"archiveSHA256":"%x","packagePath":"neuxally-plugin/demo","name":"demo","version":"2.0.0","description":"Demo","format":"codex"}]}`, source.URL+"/demo.zip", sum)
			return
		}
		downloads++
		_, _ = w.Write(archive.Bytes())
	}))
	defer source.Close()
	installer := New(Config{Store: store, Wakers: wakers, Transactions: controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB)), DataDirectory: root, CatalogSource: source.URL + "/catalog.json", Commands: controllers.NewAgentCommandController(database.DB)})
	registry := agentcapability.New(nil)
	defer registry.Close()
	grant, activate, revoke, err := registry.Prepare(agentcapability.GrantSpec{ExecutionID: "execution-one", Bindings: installer.AgentBindings(models.Chat{ID: "chat-one", WakerID: "current-partner"}, "execution-one"), Validate: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	activate()
	endpoint := httptest.NewServer(registry)
	defer endpoint.Close()
	connection, err := agentcapability.WriteConnection(root, "connection.json", agentcapability.Connection{ProtocolVersion: constants.AgentCLIProtocolVersion, Endpoint: endpoint.URL + constants.AgentCLIPath, CoreEpoch: grant.Epoch, GrantID: grant.ID, Token: grant.Token})
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "allycli")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/allycli")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %s %v", output, err)
	}
	globals := filepath.Join(root, "user-home", ".codex")
	if os.MkdirAll(globals, 0700) != nil {
		t.Fatal("create user home")
	}
	sentinel := filepath.Join(globals, "config.toml")
	if os.WriteFile(sentinel, []byte("user-config-unchanged"), 0600) != nil {
		t.Fatal("write sentinel")
	}
	run := func(input string) (map[string]any, error) {
		command := exec.Command(binary, "plugins", "installations", "install", "--input", "-", "--request-id", "stable-install")
		command.Env = []string{constants.AgentCLIContextEnvironment + "=" + connection, "PATH=" + root, "HOME=" + filepath.Dir(globals)}
		command.Stdin = strings.NewReader(input)
		output, err := command.CombinedOutput()
		var response map[string]any
		if json.Unmarshal(output, &response) != nil {
			t.Fatalf("invalid CLI output: %s", output)
		}
		return response, err
	}
	first, err := run(`{"key":"demo","trustConfirmed":true}`)
	if err != nil {
		t.Fatalf("first install: %+v %v", first, err)
	}
	replay, err := run(`{"key":"demo","trustConfirmed":true}`)
	if err != nil {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	encoded, _ := json.Marshal(replay)
	if !bytes.Contains(encoded, []byte(`"replayed":true`)) {
		t.Fatalf("missing replay: %s", encoded)
	}
	relations, err := store.Installations(context.Background(), "", "current-partner")
	if err != nil || len(relations) != 1 {
		t.Fatalf("missing current-partner install: %+v %v", relations, err)
	}
	version, err := store.Version(context.Background(), relations[0].VersionID)
	if err != nil || version.Provenance["archiveSHA256"] != fmt.Sprintf("%x", sum) || version.Provenance["sourceCommit"] != "1111111111111111111111111111111111111111" {
		t.Fatalf("lost verified distribution provenance: %+v %v", version.Provenance, err)
	}
	other, err := store.Installations(context.Background(), "", "another-partner")
	if err != nil || len(other) != 0 {
		t.Fatalf("installed for wrong partner: %+v %v", other, err)
	}
	if downloads != 1 {
		t.Fatalf("replay downloaded twice: %d", downloads)
	}
	if content, _ := os.ReadFile(sentinel); string(content) != "user-config-unchanged" {
		t.Fatal("modified user global native config")
	}
	if _, err := run(`{"key":"demo","trustConfirmed":false}`); err == nil {
		t.Fatal("accepted changed payload under same request id")
	}
	revoke()
	if _, err := run(`{"key":"demo","trustConfirmed":true}`); err == nil {
		t.Fatal("accepted revoked execution")
	}
}
