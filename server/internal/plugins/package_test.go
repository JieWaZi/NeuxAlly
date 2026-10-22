package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPortablePluginKeepsCompleteSkillAndSelectsPortableManifest(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"office.tools","version":"1.0.0","author":{"name":"Example"},"extensions":{"com.example.other":{"enabled":true}}}`)
	fixtureFile(t, root, ".claude-plugin/plugin.json", `{"name":"ignored-claude"}`)
	fixtureFile(t, root, "skills/reports/SKILL.md", "---\nname: reports\ndescription: Create reports\n---\nRead references/rules.md\n")
	fixtureFile(t, root, "skills/reports/references/rules.md", "Complete instructions")
	fixtureFile(t, root, "mcp.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"reports":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js"],"cwd":"./"},"bad":{"type":"stdio","command":"../escape"}}}`)
	pkg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Format != "agent-plugins" || pkg.Manifest.Name != "office.tools" || len(pkg.Skills) != 1 || len(pkg.Servers) != 1 {
		t.Fatalf("unexpected package: %+v", pkg)
	}
	if pkg.Skills[0].Name != "reports" || pkg.Skills[0].Path != "skills/reports" || pkg.Servers[0].Key != "reports" || len(pkg.Diagnostics) == 0 {
		t.Fatalf("missing component facts: %+v", pkg)
	}
}

func TestLoadClaudePluginImportsDeclaredSkillsAndMCPWithoutHooks(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, ".claude-plugin/plugin.json", `{"name":"legacy-tools","version":"2.0","skills":["./custom"],"mcpServers":{"remote":{"type":"http","url":"https://example.com/mcp"}},"hooks":{"SessionStart":[]}}`)
	fixtureFile(t, root, "custom/SKILL.md", "---\nname: custom\ndescription: A custom skill\n---\nUse this\n")
	pkg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Format != "claude" || len(pkg.Skills) != 1 || len(pkg.Servers) != 1 || pkg.Servers[0].Type != "streamable-http" {
		t.Fatalf("unexpected Claude import: %+v", pkg)
	}
	if len(pkg.Diagnostics) == 0 {
		t.Fatal("unsupported hooks must remain visible")
	}
}

func TestLoadRejectsInvalidManifestButKeepsOtherComponentTypes(t *testing.T) {
	for _, manifest := range []string{`{"name":"missing-schema"}`, `{"$schema":"https://bad.example/schema","name":"tools"}`, `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"Bad Name"}`, `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"tools","version":3}`} {
		root := t.TempDir()
		fixtureFile(t, root, "plugin.json", manifest)
		if _, err := Load(root); err == nil {
			t.Fatalf("accepted invalid manifest: %s", manifest)
		}
	}
	root := t.TempDir()
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"tools","unknown":true,"extensions":3}`)
	fixtureFile(t, root, "skills", "not a directory")
	fixtureFile(t, root, "mcp.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"remote":{"type":"sse","url":"https://example.com/mcp"}}}`)
	pkg, err := Load(root)
	if err != nil || len(pkg.Servers) != 1 || len(pkg.Diagnostics) < 2 {
		t.Fatalf("component error spread: %+v %v", pkg, err)
	}
}

func TestLoadRejectsEscapingManifestAndSkipsEscapingSkill(t *testing.T) {
	outside := t.TempDir()
	fixtureFile(t, outside, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"outside"}`)
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "plugin.json"), filepath.Join(root, "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("accepted manifest outside package")
	}
	root = t.TempDir()
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"safe"}`)
	fixtureFile(t, root, "skills/local/SKILL.md", "---\nname: local\ndescription: safe\n---\n")
	fixtureFile(t, outside, "SKILL.md", "---\nname: outside\ndescription: unsafe\n---\n")
	if err := os.Symlink(outside, filepath.Join(root, "skills", "external")); err != nil {
		t.Fatal(err)
	}
	pkg, err := Load(root)
	if err != nil || len(pkg.Skills) != 1 || len(pkg.Diagnostics) == 0 {
		t.Fatalf("unsafe skill loaded: %+v %v", pkg, err)
	}
}

func TestPortableSkillsDiscoverOnlyImmediateSubdirectories(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"discovery"}`)
	fixtureFile(t, root, "skills/SKILL.md", "---\nname: wrong-root\ndescription: not a portable component\n---\n")
	fixtureFile(t, root, "skills/child/SKILL.md", "---\nname: child\ndescription: valid component\n---\n")
	pkg, err := Load(root)
	if err != nil || len(pkg.Skills) != 1 || pkg.Skills[0].Name != "child" {
		t.Fatalf("nonstandard discovery: %+v %v", pkg, err)
	}
}
func TestPortableRejectsNullMetadataAndMalformedServerFields(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"metadata","keywords":null}`)
	if _, err := Load(root); err == nil {
		t.Fatal("accepted null keyword metadata")
	}
	fixtureFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"metadata"}`)
	fixtureFile(t, root, "mcp.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"bad-args":{"type":"stdio","command":"node","args":null},"bad-env":{"type":"stdio","command":"node","env":{"TOKEN":null}},"safe":{"type":"stdio","command":"node","args":[]}}}`)
	pkg, err := Load(root)
	if err != nil || len(pkg.Servers) != 1 || pkg.Servers[0].Key != "safe" {
		t.Fatalf("malformed entries affected valid boundary: %+v %v", pkg, err)
	}
}

func TestClaudeNativeCommandAndWorkingDirectoryUseClaudeRootAlias(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, ".claude-plugin/plugin.json", `{"name":"native-command"}`)
	fixtureFile(t, root, "scripts/server", "native fixture")
	fixtureFile(t, root, ".mcp.json", `{"mcpServers":{"native":{"command":"${CLAUDE_PLUGIN_ROOT}/scripts/server","cwd":"${CLAUDE_PLUGIN_ROOT}"}}}`)
	pkg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Servers) != 1 {
		t.Fatalf("Claude root alias rejected: %+v", pkg.Diagnostics)
	}
}

func TestPackageSnapshotRetainsLegacyRootWithoutChangingNewPackages(t *testing.T) {
	for _, raw := range []string{`{"manifest":{"name":"office"},"sourcePath":"plugins/office"}`, `{"manifest":{"name":"office"}}`} {
		var pkg Package
		if err := json.Unmarshal([]byte(raw), &pkg); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(pkg)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip Package
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if roundTrip.legacySourcePath != pkg.legacySourcePath || roundTrip.Manifest.Name != "office" {
			t.Fatalf("snapshot changed: %s", encoded)
		}
	}
}

func TestLegacyPackageRuntimeRootRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"../outside", outside, "outside", "missing"} {
		pkg := Package{legacySourcePath: source}
		if _, err := pkg.RuntimeRoot(root); err == nil {
			t.Fatalf("unsafe or unavailable legacy root accepted: %q", source)
		}
	}
	pkg := Package{}
	if actual, err := pkg.RuntimeRoot(root); err != nil || actual != root {
		t.Fatalf("new package root changed: %q %v", actual, err)
	}
}
