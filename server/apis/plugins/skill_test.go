package plugins

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
)

func TestPluginSkillReadsOnlyRequestedImmutableVersionAndVisibleOwner(t *testing.T) {
	ctx := pluginAPIContext(t)
	source := t.TempDir()
	for filename, content := range map[string]string{
		".codex-plugin/plugin.json":          `{"name":"reader","version":"1.0.0","skills":"./skills/"}`,
		"skills/reports/SKILL.md":            "---\nname: reports\ndescription: Write reports\n---\n# Reports\nRead references/rules.md\n",
		"skills/reports/references/rules.md": "A real resource",
		"README.md":                          "Unrelated plugin overview",
	} {
		path := filepath.Join(source, filename)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stage, err := packageplugin.Stage(stagedRoot(), source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := (&PreparePlugin{Body: PreparePluginRequest{Token: stage.Token, OwnerWakerID: "w1"}}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detail := value.(PluginDetailResponse)
	version := detail.Versions[0]
	request := GetPluginSkill{PluginID: detail.Plugin.ID, VersionID: version.ID, WakerID: "w1", Key: "skills/reports"}
	value, err = request.Output(ctx)
	if err != nil || !strings.Contains(value.(PluginSkillResponse).Content, "Read references/rules.md") {
		t.Fatalf("skill content not readable: %+v %v", value, err)
	}
	response := value.(PluginSkillResponse)
	if len(response.Resources) != 3 || response.File != nil || response.Version.ID != version.ID || len(response.Versions) != 1 || response.Versions[0].Skill.Name != "reports" {
		t.Fatalf("skill detail facts incomplete: %+v", response)
	}
	request.FilePath = "references/rules.md"
	value, err = request.Output(ctx)
	if err != nil || value.(PluginSkillResponse).File == nil || value.(PluginSkillResponse).File.Content != "A real resource" {
		t.Fatalf("skill resource not readable: %+v %v", value, err)
	}
	request.FilePath = "../../README.md"
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginContentInvalid) {
		t.Fatalf("plugin file escaped skill boundary: %v", err)
	}
	request.FilePath = ""
	request.WakerID = "w2"
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginForbidden) {
		t.Fatalf("private skill leaked to another owner: %v", err)
	}
	request.WakerID, request.Key = "w1", "../plugin.json"
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginContentInvalid) {
		t.Fatalf("arbitrary file requested as a skill: %v", err)
	}
	request.Key = "skills/reports"
	if err := os.WriteFile(filepath.Join(global.Config.DataDirectory, version.ContentPath, "skills/reports/SKILL.md"), []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginVersionConflict) {
		t.Fatalf("mutable source displayed as the installed version: %v", err)
	}
}

func TestPluginSkillHistoryOnlyIncludesSameComponentKey(t *testing.T) {
	ctx := pluginAPIContext(t)
	prepare := func(target, version, key string) PluginDetailResponse {
		source := t.TempDir()
		for name, content := range map[string]string{
			".codex-plugin/plugin.json":   `{"name":"history-reader","version":"` + version + `"}`,
			"skills/" + key + "/SKILL.md": "---\nname: " + key + "\ndescription: Version " + version + "\n---\n# " + version + "\n",
		} {
			path := filepath.Join(source, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		stage, err := packageplugin.Stage(stagedRoot(), source)
		if err != nil {
			t.Fatal(err)
		}
		value, err := (&PreparePlugin{Body: PreparePluginRequest{Token: stage.Token, PluginID: target}}).Output(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return value.(PluginDetailResponse)
	}
	first := prepare("", "1.0.0", "reports")
	prepare(first.Plugin.ID, "2.0.0", "unrelated")
	last := prepare(first.Plugin.ID, "3.0.0", "reports")
	value, err := (&GetPluginSkill{PluginID: first.Plugin.ID, VersionID: last.ResolvedVersionID, Key: "skills/reports"}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	response := value.(PluginSkillResponse)
	if len(response.Versions) != 2 || response.Version.Version != "3.0.0" {
		t.Fatalf("wrong skill history: %+v", response)
	}
	for _, version := range response.Versions {
		if version.Skill.Key != "skills/reports" || version.Version == "2.0.0" || version.ID == "" || version.Skill.Description != "Version "+version.Version {
			t.Fatalf("unrelated history entry: %+v", version)
		}
	}
	value, err = (&GetPluginSkill{PluginID: first.Plugin.ID, VersionID: first.ResolvedVersionID, Key: "skills/reports"}).Output(ctx)
	if err != nil || value.(PluginSkillResponse).Version.Version != "1.0.0" || !strings.Contains(value.(PluginSkillResponse).Content, "# 1.0.0") {
		t.Fatalf("historical skill not readable: %+v %v", value, err)
	}
}

func TestCatalogSkillVerifiesArchiveAndReleasesTemporarySnapshot(t *testing.T) {
	ctx := pluginAPIContext(t)
	previous := global.Config.PluginResourcesDirectory
	global.Config.PluginResourcesDirectory = t.TempDir()
	t.Cleanup(func() { global.Config.PluginResourcesDirectory = previous })
	root := global.Config.PluginResourcesDirectory
	if err := os.MkdirAll(filepath.Join(root, "catalog"), 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "catalog", "reader.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range map[string]string{".claude-plugin/plugin.json": `{"name":"reader","version":"1.2.3"}`, "skills/reports/SKILL.md": "---\nname: reports\ndescription: Reports\n---\n# Catalog skill\n", "skills/reports/references/rules.md": "Catalog resource"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(bytes)
	index, _ := json.Marshal(map[string]any{"schemaVersion": 1, "entries": []map[string]any{{"key": "reader", "directory": "catalog/reader.zip", "sourceCommit": strings.Repeat("a", 40), "archiveSHA256": hex.EncodeToString(digest[:]), "package": map[string]any{"manifest": map[string]any{"name": "reader"}}}}})
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	request := GetCatalogPluginSkill{Key: "reader", SkillKey: "skills/reports"}
	value, err := request.Output(ctx)
	if err != nil || !strings.Contains(value.(PluginSkillResponse).Content, "# Catalog skill") {
		t.Fatalf("catalog skill not readable: %+v %v", value, err)
	}
	response := value.(PluginSkillResponse)
	if len(response.Resources) != 3 || response.File != nil || len(response.Versions) != 1 || response.Version.Version != "1.2.3" || response.Version.ID != "" {
		t.Fatalf("catalog skill details incomplete: %+v", response)
	}
	request.FilePath = "references/rules.md"
	value, err = request.Output(ctx)
	if err != nil || value.(PluginSkillResponse).File == nil || value.(PluginSkillResponse).File.Content != "Catalog resource" {
		t.Fatalf("catalog resource unavailable: %+v %v", value, err)
	}
	request.FilePath = "../plugin.json"
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginContentInvalid) {
		t.Fatalf("catalog resource traversed skill root: %v", err)
	}
	request.FilePath = ""
	entries, err := os.ReadDir(stagedRoot())
	if err != nil || len(entries) != 0 {
		t.Fatalf("skill preview left a persistent snapshot: %+v %v", entries, err)
	}
	if err := os.WriteFile(archive, []byte("mutated archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := request.Output(ctx); !errors.Is(err, status_error.PluginContentInvalid) {
		t.Fatalf("catalog preview accepted an unverified archive: %v", err)
	}
}
