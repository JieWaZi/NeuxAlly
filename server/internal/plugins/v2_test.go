package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2SourceSelectsPinnedNativeAndRejectsMissingTarget(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, "neuxally-plugin/github/.codex-plugin/plugin.json", `{"name":"github","version":"2"}`)
	fixtureFile(t, root, "neuxally-plugin/github/.neuxally/integration.json", `{"nativePlugins":{"codex":{"path":"codex-plugins/github"}}}`)
	fixtureFile(t, root, "codex-plugins/github/.codex-plugin/plugin.json", `{"name":"github-native","version":"1","hooks":{"SessionStart":[]}}`)
	pkg, err := LoadSource(root, "neuxally-plugin/github")
	if err != nil {
		t.Fatal(err)
	}
	selected, target, err := SelectTarget(root, pkg, "codex")
	if err != nil || selected.Manifest.Version != "1" || !strings.HasSuffix(target, "codex-plugins/github") {
		t.Fatalf("wrong target: %+v %s %v", selected, target, err)
	}
	if _, _, err := SelectTarget(root, pkg, "claude"); err == nil {
		t.Fatal("reference-only package fabricated a fallback")
	}
	fixtureFile(t, root, "neuxally-plugin/github/.neuxally/integration.json", `{"nativePlugins":{"codex":{"path":"../outside"}}}`)
	if _, err := LoadSource(root, "neuxally-plugin/github"); err == nil {
		t.Fatal("accepted escaping reference")
	}
}

func TestReadme300KiBAndExplicitOversizeState(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, ".codex-plugin/plugin.json", `{"name":"readme"}`)
	fixtureFile(t, root, "README.md", strings.Repeat("x", 300*1024))
	pkg, err := Load(root)
	if err != nil || len(pkg.Readme) != 300*1024 || pkg.ReadmeStatus != "available" {
		t.Fatalf("readme truncated: %d %s %v", len(pkg.Readme), pkg.ReadmeStatus, err)
	}
	fixtureFile(t, root, "README.md", strings.Repeat("x", 1024*1024+1))
	pkg, err = Load(root)
	if err != nil || pkg.ReadmeStatus != "too-large" || pkg.Readme != "" {
		t.Fatalf("oversize inaccurate: %+v %v", pkg, err)
	}
}

func TestBridgeNodeEntryIsContained(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, ".codex-plugin/plugin.json", `{"name":"bridge"}`)
	fixtureFile(t, root, ".neuxally/auth/authorize.mjs", "// entry")
	fixtureFile(t, root, ".neuxally/integration.json", `{"authentication":{"strategy":"plugin-command","protocol":"neuxally-auth/1","command":"node","args":["./.neuxally/auth/authorize.mjs"]}}`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, root, ".neuxally/integration.json", `{"authentication":{"strategy":"plugin-command","protocol":"neuxally-auth/1","command":"node","args":["../outside.mjs"]}}`)
	if _, err := Load(root); err == nil {
		t.Fatal("accepted outside bridge script")
	}
}

func TestCodexWorkspaceRefusesSymlinkContainer(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	fixtureFile(t, outside, "plugins/keep.txt", "outside must remain")
	if err := os.Symlink(outside, filepath.Join(workspace, ".agents")); err != nil {
		t.Skip(err)
	}
	if err := PrepareCodexWorkspace(workspace, nil); err == nil {
		t.Fatal("followed workspace .agents symlink")
	}
	if content, err := os.ReadFile(filepath.Join(outside, "plugins/keep.txt")); err != nil || string(content) != "outside must remain" {
		t.Fatal("modified external plugin directory")
	}
}

func TestCrossNativeDerivationPreservesInlineAndDefaultComponents(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, root, ".codex-plugin/plugin.json", `{"name":"native-mixed","version":"1","commands":"./custom-commands","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo preserved"}]}]},"agents":"./agents"}`)
	fixtureFile(t, root, "custom-commands/probe.md", "Preserved command")
	fixtureFile(t, root, "agents/reviewer.md", "Preserved agent")
	fixtureFile(t, root, ".lsp.json", `{"probe":{"command":"probe-lsp"}}`)
	pkg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	target, err := PrepareNativeTarget(t.TempDir(), "plugin", pkg, root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := readObject(filepath.Join(target.Root, ".claude-plugin/plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := declaration["author"]; exists {
		t.Fatal("invented invalid empty author for target CLI")
	}
	for _, key := range []string{"commands", "hooks", "agents"} {
		if _, ok := declaration[key]; !ok {
			t.Fatalf("lost inline %s", key)
		}
	}
	for _, path := range []string{".codex-plugin/plugin.json", "custom-commands/probe.md", "agents/reviewer.md", ".lsp.json"} {
		if _, err := os.Stat(filepath.Join(target.Root, path)); err != nil {
			t.Fatalf("lost native file %s", path)
		}
	}
}
