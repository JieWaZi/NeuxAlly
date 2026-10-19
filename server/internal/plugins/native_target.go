package plugins

import (
	"encoding/json"
	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"os"
	"path/filepath"
	"strings"
)

// PrepareNativeTarget 为有相同原生格式的目标保留原包，否则只转换有明确映射的标准主体。
func PrepareNativeTarget(parent, id string, pkg Package, root, cli string) (NativeTarget, error) {
	digest, err := pluginArchive.Digest(root)
	if err != nil {
		return NativeTarget{}, err
	}
	target := NativeTarget{ID: id, Name: pkg.Manifest.Name, Version: pkg.Manifest.Version, Root: root, Digest: digest}
	if target.Version == "" {
		target.Version = digest[:12]
	}
	expected := constants.PluginFormatCodex
	if cli == "claude" {
		expected = constants.PluginFormatClaude
	}
	if pkg.Format == expected {
		return target, nil
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return NativeTarget{}, status_error.PluginStorageError
	}
	stage, err := os.MkdirTemp(parent, "native-")
	if err != nil {
		return NativeTarget{}, status_error.PluginStorageError
	}
	defer os.RemoveAll(stage)
	if err := pluginArchive.CopyDirectory(root, stage); err != nil {
		return NativeTarget{}, err
	}
	manifest := map[string]any{"name": pkg.Manifest.Name}
	if pkg.Manifest.Version != "" {
		manifest["version"] = pkg.Manifest.Version
	}
	if pkg.Manifest.Description != "" {
		manifest["description"] = pkg.Manifest.Description
	}
	if pkg.Manifest.Author.Name != "" {
		manifest["author"] = pkg.Manifest.Author
	}
	if pkg.Manifest.License != "" {
		manifest["license"] = pkg.Manifest.License
	}
	if pkg.Manifest.Homepage != "" {
		manifest["homepage"] = pkg.Manifest.Homepage
	}
	if pkg.Manifest.Repository != "" {
		manifest["repository"] = pkg.Manifest.Repository
	}
	if len(pkg.Manifest.Keywords) > 0 {
		manifest["keywords"] = pkg.Manifest.Keywords
	}
	original := "plugin.json"
	if pkg.Format == constants.PluginFormatCodex {
		original = ".codex-plugin/plugin.json"
	}
	if pkg.Format == constants.PluginFormatClaude {
		original = ".claude-plugin/plugin.json"
	}
	raw, readErr := readObject(filepath.Join(root, original))
	if readErr != nil {
		return NativeTarget{}, status_error.PluginManifestInvalid
	}
	// 目标原生机制解释事件和其他声明；宿主保留内容，不伪造跨端语义等价。
	for _, key := range []string{"hooks", "agents", "commands", "lspServers", "apps", "outputStyles", "settings", "userConfig", "dependencies", "experimental", "channels", "workflows"} {
		if value, ok := raw[key]; ok {
			manifest[key] = value
		}
	}

	if len(pkg.Skills) > 0 {
		paths := []string{}
		for _, skill := range pkg.Skills {
			paths = append(paths, "./"+skill.Path)
		}
		manifest["skills"] = paths
	}
	if len(pkg.Servers) > 0 {
		servers := map[string]any{}
		for _, server := range pkg.Servers {
			data, _ := json.Marshal(server)
			if cli == "claude" {
				data = []byte(strings.NewReplacer("${PLUGIN_ROOT}", "${CLAUDE_PLUGIN_ROOT}", "${PLUGIN_DATA}", "${CLAUDE_PLUGIN_DATA}").Replace(string(data)))
			} else {
				data = []byte(strings.NewReplacer("${CLAUDE_PLUGIN_ROOT}", "${PLUGIN_ROOT}", "${CLAUDE_PLUGIN_DATA}", "${PLUGIN_DATA}").Replace(string(data)))
			}
			var fields map[string]any
			_ = json.Unmarshal(data, &fields)
			delete(fields, "key")
			if fields["type"] == "streamable-http" {
				fields["type"] = "http"
			}
			servers[server.Key] = fields
		}
		manifest["mcpServers"] = servers
	}
	directory := ".codex-plugin"
	if cli == "claude" {
		directory = ".claude-plugin"
	}
	if os.MkdirAll(filepath.Join(stage, directory), 0700) != nil {
		return NativeTarget{}, status_error.PluginStorageError
	}
	data, _ := json.Marshal(manifest)
	if os.WriteFile(filepath.Join(stage, directory, "plugin.json"), data, 0600) != nil {
		return NativeTarget{}, status_error.PluginStorageError
	}
	derivedDigest, err := pluginArchive.Digest(stage)
	if err != nil {
		return NativeTarget{}, err
	}
	content, err := CopyContent(parent, stage, derivedDigest)
	if err != nil {
		return NativeTarget{}, err
	}
	target.Root = content
	target.Digest = derivedDigest
	return target, nil
}
