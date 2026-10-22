package global

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/JieWaZi/neux-knowledge/httpapi"
)

func TestKnowledgeServerConfigRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote.json")
	for _, raw := range []string{`{}`, `{"security":{"Tokens":{"secret":{"Subject":"alice"}}},"unknown":1}`, `{"security":{"Tokens":{"secret":{"Subject":"alice"}}}} {}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readKnowledgeServerConfig(path); err == nil {
			t.Fatalf("accepted invalid config")
		}
	}
	cfg := knowledgeServerConfiguration{Listen: "127.0.0.1:0", Security: httpapi.SecurityConfig{Tokens: map[string]httpapi.Principal{"secret": {Subject: "alice"}}}}
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(path, b, 0600)
	if _, err := readKnowledgeServerConfig(path); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeServerSharesEngineButNeverExposesDesktopRoutes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	engine, err := core.Open(ctx, core.Options{DBPath: filepath.Join(dir, "wiki.db"), RawDir: filepath.Join(dir, "raw")})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	receipt, err := engine.CreateLibrary(ctx, core.CreateLibraryInput{RequestID: "create", Name: "Remote", Kind: "general", WikiLanguage: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := knowledgeServerConfiguration{Security: httpapi.SecurityConfig{Tokens: map[string]httpapi.Principal{"remote-secret": {Subject: "alice", Libraries: map[string]httpapi.LibraryGrant{receipt.LibraryID: {Operations: []string{"read"}, PolicyVersion: 1}}}}}}
	host, err := newKnowledgeRemoteHost(engine, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(host.server.Handler)
	defer server.Close()
	for _, tc := range []struct {
		path, token string
		want        int
	}{{httpapi.BasePath + "/libraries", "remote-secret", 200}, {httpapi.BasePath + "/libraries", "local-secret", 401}, {"/api/v1/wakers", "remote-secret", 404}, {httpapi.BasePath + "/libraries/unknown", "remote-secret", 403}} {
		req, _ := http.NewRequest("GET", server.URL+tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		response, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != tc.want {
			t.Fatalf("%s got %d want %d", tc.path, response.StatusCode, tc.want)
		}
	}
}
