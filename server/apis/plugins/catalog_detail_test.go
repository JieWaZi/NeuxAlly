package plugins

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/catalog"
)

func TestCatalogDetailAndRawReadmeAreReadOnlyAndSourceBound(t *testing.T) {
	for _, size := range []int{300 * 1024, 1024*1024 + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			ctx := pluginAPIContext(t)
			readme := strings.Repeat("Readme正文\n", size/13+1)
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			for name, content := range map[string]string{"neuxally-plugin/demo/.codex-plugin/plugin.json": `{"name":"demo","version":"2"}`, "neuxally-plugin/demo/README.md": readme} {
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
			var source *httptest.Server
			source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/catalog.json" {
					fmt.Fprintf(w, `{"schemaVersion":2,"entries":[{"key":"demo","sourceCommit":"1111111111111111111111111111111111111111","archiveURL":%q,"archiveSHA256":"%x","packagePath":"neuxally-plugin/demo","name":"demo","version":"2","format":"codex"}]}`, source.URL+"/demo.zip", sum)
					return
				}
				_, _ = w.Write(archive.Bytes())
			}))
			defer source.Close()
			t.Setenv("ALLY_PLUGIN_CATALOG_URL", source.URL+"/catalog.json")
			output, err := (&GetCatalogPlugin{Key: "demo"}).Output(ctx)
			if err != nil {
				t.Fatal(err)
			}
			entry := output.(catalog.Entry)
			if size == 300*1024 && (entry.Package.ReadmeStatus != "available" || entry.Package.Readme != readme) {
				t.Fatal("300 KiB readme was truncated")
			}
			if size > 1024*1024 && entry.Package.ReadmeStatus != "too-large" {
				t.Fatalf("oversize not explicit: %s", entry.Package.ReadmeStatus)
			}
			raw, err := (&GetCatalogPluginReadme{Key: "demo", SourceCommit: entry.SourceCommit, ArchiveSHA256: entry.ArchiveSHA256}).Output(ctx)
			if err != nil || raw.(PluginReadmeResponse).Content != readme {
				t.Fatalf("raw content changed: %v", err)
			}
			_, err = (&GetCatalogPluginReadme{Key: "demo", SourceCommit: "2222222222222222222222222222222222222222"}).Output(ctx)
			if !errors.Is(err, status_error.PluginVersionConflict) {
				t.Fatalf("accepted mismatched displayed version: %v", err)
			}
			_, total, err := global.PluginControllerManager.List(ctx, "", "", "", 0, 100)
			if err != nil || total != 0 {
				t.Fatalf("read-only preview created system resources: %d %v", total, err)
			}
		})
	}
}
