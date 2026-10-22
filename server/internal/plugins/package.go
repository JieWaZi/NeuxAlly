// Package plugins 解析本地插件包，保留原始内容并生成可交付组件与诊断。
package plugins

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/skills"
)

const maxMetadataBytes = 1024 * 1024

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,62}[a-z0-9])?$`)

// Author 保存包作者的公开署名资料。
type Author struct {
	// Name 是作者署名。
	Name string `json:"name,omitempty"`
	// Email 是公开联系地址。
	Email string `json:"email,omitempty"`
	// URL 是作者公开主页。
	URL string `json:"url,omitempty"`
}

// Manifest 保存标准元数据；客户端扩展不覆盖标准组件发现。
type Manifest struct {
	// Schema 是固定规范标识。
	Schema string `json:"$schema,omitempty"`
	// Name 是包声明名称。
	Name string `json:"name"`
	// Version 是上游版本字符串。
	Version string `json:"version,omitempty"`
	// Description 是能力说明。
	Description string `json:"description,omitempty"`
	// Author 是原作者资料。
	Author Author `json:"author"`
	// Homepage 是公开说明入口。
	Homepage string `json:"homepage,omitempty"`
	// Repository 是上游来源。
	Repository string `json:"repository,omitempty"`
	// License 是上游许可标识。
	License string `json:"license,omitempty"`
	// Keywords 是包声明标签。
	Keywords []string `json:"keywords"`
	// Extensions 是按命名空间保留的客户端扩展。
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Diagnostic 保存组件局部失败和未支持能力，不含凭据。
type Diagnostic struct {
	// Component 是包内组件或字段身份。
	Component string `json:"component"`
	// Code 是可本地化的稳定诊断标识。
	Code string `json:"code"`
}

// Skill 是插件版本派生的技能，不拥有独立安装事实。
type Skill struct {
	// Key 是版本内稳定组件键。
	Key string `json:"key"`
	// Name 是 SKILL.md 的真实声明名。
	Name string `json:"name"`
	// Description 是技能用途。
	Description string `json:"description"`
	// Path 是相对包根的交付目录。
	Path string `json:"path"`
	// Digest 是完整技能内容摘要。
	Digest string `json:"digest"`
}

// Server 保存插件内 MCP 连接定义，不包含本机共享凭据。
type Server struct {
	// Key 是清单内稳定服务键。
	Key string `json:"key"`
	// Type 是标准 MCP 传输。
	Type string `json:"type"`
	// Command 是程序令牌或包内相对程序。
	Command string `json:"command,omitempty"`
	// Args 是保持原序的程序参数。
	Args []string `json:"args"`
	// Env 是待展开的环境定义。
	Env map[string]string `json:"env"`
	// Cwd 是规范允许的工作目录。
	Cwd string `json:"cwd,omitempty"`
	// URL 是待展开的外部服务地址。
	URL string `json:"url,omitempty"`
	// Headers 是待展开的请求头定义。
	Headers map[string]string `json:"headers"`
}

// Package 是对原始包的一次确定性解析结果。
type Package struct {
	// legacySourcePath 保留旧版仓库快照中已固定的包子目录；新导入直接以包根存储。
	legacySourcePath string
	// Manifest 是已验证的包元数据。
	Manifest Manifest `json:"manifest"`
	// Format 是实际采用的包格式。
	Format string `json:"format"`
	// Skills 是可交付技能，按组件键排序。
	Skills []Skill `json:"skills"`
	// Servers 是可交付 MCP 定义，按组件键排序。
	Servers []Server `json:"servers"`
	// Diagnostics 是不阻断其他组件的逐项诊断。
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// UnmarshalJSON 保留历史不可变版本的包根，避免丢弃旧快照的路径语义。
func (pkg *Package) UnmarshalJSON(data []byte) error {
	type snapshot Package
	var stored struct {
		// snapshot 是当前格式的组件描述。
		*snapshot
		// SourcePath 是旧版快照中相对于内容根的包路径。
		SourcePath string `json:"sourcePath"`
	}
	stored.snapshot = (*snapshot)(pkg)
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	pkg.legacySourcePath = stored.SourcePath
	return nil
}

// MarshalJSON 在保存旧版快照时保留包根，不向新导入增加历史字段。
func (pkg Package) MarshalJSON() ([]byte, error) {
	type snapshot Package
	return json.Marshal(struct {
		// snapshot 是当前格式的组件描述。
		snapshot
		// SourcePath 仅为仍引用仓库快照的旧版本保留。
		SourcePath string `json:"sourcePath,omitempty"`
	}{snapshot: snapshot(pkg), SourcePath: pkg.legacySourcePath})
}

// RuntimeRoot 从已验证的内容根解析实际交付目录，不猜测目录或修改固定快照。
func (pkg Package) RuntimeRoot(contentRoot string) (string, error) {
	if pkg.legacySourcePath == "" {
		return contentRoot, nil
	}
	root, err := contained(contentRoot, pkg.legacySourcePath)
	if err != nil {
		return "", status_error.PluginContentInvalid
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", status_error.PluginContentInvalid
	}
	return root, nil
}

// Load 优先读取通用清单，缺失时按 Claude 导入规则发现受支持组件。
func Load(root string) (Package, error) {
	pkg := Package{Skills: []Skill{}, Servers: []Server{}, Diagnostics: []Diagnostic{}}
	portable := filepath.Join(root, "plugin.json")
	_, err := os.Lstat(portable)
	manifestPath := "plugin.json"
	if os.IsNotExist(err) {
		manifestPath = ".claude-plugin/plugin.json"
		pkg.Format = constants.PluginFormatClaude
	} else {
		pkg.Format = constants.PluginFormatPortable
	}
	path, err := contained(root, manifestPath)
	if err != nil {
		return pkg, status_error.PluginManifestInvalid
	}
	raw, err := readObject(path)
	if err != nil {
		return pkg, status_error.PluginManifestInvalid
	}
	if pkg.Format == constants.PluginFormatPortable {
		if !stringEquals(raw["$schema"], constants.PluginManifestSchema) {
			return pkg, status_error.PluginManifestInvalid
		}
		allowed := map[string]bool{"$schema": true, "name": true, "version": true, "description": true, "author": true, "homepage": true, "repository": true, "license": true, "keywords": true, "extensions": true}
		for key := range raw {
			if !allowed[key] {
				pkg.diagnose(key, "unknown_field")
				delete(raw, key)
			}
		}
		if value, ok := raw["extensions"]; ok {
			var ext map[string]json.RawMessage
			if json.Unmarshal(value, &ext) != nil || ext == nil {
				pkg.diagnose("extensions", "invalid_extension")
				delete(raw, "extensions")
			}
		}
		if value, ok := raw["author"]; ok {
			var author map[string]json.RawMessage
			if json.Unmarshal(value, &author) != nil || author == nil {
				return pkg, status_error.PluginManifestInvalid
			}
			for key, value := range author {
				var text string
				if (key != "name" && key != "email" && key != "url") || json.Unmarshal(value, &text) != nil || string(value) == "null" {
					return pkg, status_error.PluginManifestInvalid
				}
			}
		}
	}
	for _, key := range []string{"name", "version", "description", "homepage", "repository", "license"} {
		if value, ok := raw[key]; ok {
			var text string
			if json.Unmarshal(value, &text) != nil || string(value) == "null" {
				return pkg, status_error.PluginManifestInvalid
			}
		}
	}
	if value, ok := raw["keywords"]; ok && !validStringArray(value) {
		return pkg, status_error.PluginManifestInvalid
	}
	encoded, _ := json.Marshal(raw)
	if json.Unmarshal(encoded, &pkg.Manifest) != nil {
		return pkg, status_error.PluginManifestInvalid
	}
	if !pluginNamePattern.MatchString(pkg.Manifest.Name) || strings.Contains(pkg.Manifest.Name, "--") || strings.Contains(pkg.Manifest.Name, "..") {
		return pkg, status_error.PluginManifestInvalid
	}
	if pkg.Manifest.Keywords == nil {
		pkg.Manifest.Keywords = []string{}
	}
	skillPaths := []string{"skills"}
	mcpPaths := []string{"mcp.json"}
	if pkg.Format == constants.PluginFormatClaude {
		mcpPaths = []string{".mcp.json"}
		if value, ok := raw["skills"]; ok {
			if err := decodePaths(value, &skillPaths); err != nil {
				pkg.diagnose("skills", "invalid_component")
				skillPaths = nil
			}
		}
		if value, ok := raw["mcpServers"]; ok {
			var inline map[string]json.RawMessage
			if json.Unmarshal(value, &inline) == nil && inline != nil {
				pkg.loadServers(root, inline)
			} else if decodePaths(value, &mcpPaths) != nil {
				pkg.diagnose("mcpServers", "invalid_component")
				mcpPaths = nil
			}
		}
		for _, key := range []string{"hooks", "agents", "commands", "lspServers", "channels"} {
			if _, ok := raw[key]; ok {
				pkg.diagnose(key, "unsupported_component")
			}
		}
	}
	seen := map[string]bool{}
	for _, path := range skillPaths {
		pkg.loadSkills(root, path, seen)
	}
	for _, path := range mcpPaths {
		resolved, pathErr := contained(root, path)
		if pathErr != nil {
			if !os.IsNotExist(pathErr) {
				pkg.diagnose(path, "invalid_component")
			}
			continue
		}
		document, readErr := readObject(resolved)
		if readErr != nil {
			pkg.diagnose(path, "invalid_component")
			continue
		}
		if pkg.Format == constants.PluginFormatPortable && !stringEquals(document["$schema"], constants.PluginMCPSchema) {
			pkg.diagnose(path, "unsupported_schema")
			continue
		}
		if pkg.Format == constants.PluginFormatPortable {
			valid := true
			for key := range document {
				if key != "$schema" && key != "mcpServers" {
					valid = false
				}
			}
			if !valid {
				pkg.diagnose(path, "invalid_component")
				continue
			}
		}
		var servers map[string]json.RawMessage
		if json.Unmarshal(document["mcpServers"], &servers) != nil || servers == nil {
			pkg.diagnose(path, "invalid_component")
			continue
		}
		pkg.loadServers(root, servers)
	}
	sort.Slice(pkg.Skills, func(i, j int) bool { return pkg.Skills[i].Key < pkg.Skills[j].Key })
	sort.Slice(pkg.Servers, func(i, j int) bool { return pkg.Servers[i].Key < pkg.Servers[j].Key })
	sort.Slice(pkg.Diagnostics, func(i, j int) bool {
		return pkg.Diagnostics[i].Component+pkg.Diagnostics[i].Code < pkg.Diagnostics[j].Component+pkg.Diagnostics[j].Code
	})
	return pkg, nil
}

func (p *Package) diagnose(component, code string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Component: component, Code: code})
}
func readObject(path string) (map[string]json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxMetadataBytes {
		return nil, status_error.PluginContentInvalid
	}
	var value map[string]json.RawMessage
	decoder := json.NewDecoder(file)
	if decoder.Decode(&value) != nil || value == nil {
		return nil, status_error.PluginContentInvalid
	}
	// 第二个 JSON 值不能被当作未读取的尾部忽略。
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return nil, status_error.PluginContentInvalid
	}
	return value, nil
}
func stringEquals(raw json.RawMessage, want string) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && value == want
}
func decodePaths(raw json.RawMessage, result *[]string) error {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		*result = []string{one}
		return nil
	}
	var paths []string
	if json.Unmarshal(raw, &paths) != nil || paths == nil {
		return status_error.PluginContentInvalid
	}
	*result = paths
	return nil
}
func contained(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", status_error.PluginContentInvalid
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", status_error.PluginContentInvalid
	}
	return resolved, nil
}
func (p *Package) loadSkills(root, relative string, seen map[string]bool) {
	path, err := contained(root, relative)
	if err != nil {
		if !os.IsNotExist(err) {
			p.diagnose(relative, "invalid_component")
		}
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		p.diagnose(relative, "invalid_component")
		return
	}
	if _, err := os.Lstat(filepath.Join(path, "SKILL.md")); err == nil && p.Format == constants.PluginFormatClaude {
		p.loadSkill(root, relative, seen)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		p.diagnose(relative, "invalid_component")
		return
	}
	for _, entry := range entries {
		p.loadSkill(root, filepath.ToSlash(filepath.Join(relative, entry.Name())), seen)
	}
}
func (p *Package) loadSkill(root, relative string, seen map[string]bool) {
	if seen[relative] {
		return
	}
	seen[relative] = true
	path, err := contained(root, relative)
	if err != nil {
		p.diagnose(relative, "invalid_skill")
		return
	}
	if _, err := contained(root, filepath.Join(relative, "SKILL.md")); err != nil {
		if !os.IsNotExist(err) {
			p.diagnose(relative, "invalid_skill")
		}
		return
	}
	metadata, err := skills.ReadMetadata(path)
	if err != nil {
		p.diagnose(relative, "invalid_skill")
		return
	}
	digest, err := pluginArchive.Digest(path)
	if err != nil {
		p.diagnose(relative, "invalid_skill")
		return
	}
	p.Skills = append(p.Skills, Skill{Key: filepath.ToSlash(relative), Name: metadata.Name, Description: metadata.Description, Path: filepath.ToSlash(relative), Digest: digest})
}
func (p *Package) loadServers(root string, definitions map[string]json.RawMessage) {
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var server Server
		raw := definitions[key]
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &server) != nil || json.Unmarshal(raw, &fields) != nil || fields == nil || key == "" {
			p.diagnose(key, "invalid_mcp")
			continue
		}
		if p.Format == constants.PluginFormatClaude {
			if server.Type == "" && server.Command != "" {
				server.Type = "stdio"
			}
			if server.Type == "http" {
				server.Type = "streamable-http"
			}
		}
		valid := true
		for _, field := range []string{"type", "command", "cwd", "url"} {
			if raw, ok := fields[field]; ok && !validString(raw) {
				valid = false
			}
		}
		if raw, ok := fields["args"]; ok && !validStringArray(raw) {
			valid = false
		}
		for _, field := range []string{"env", "headers"} {
			if raw, ok := fields[field]; ok && !validStringMap(raw) {
				valid = false
			}
		}
		switch server.Type {
		case "stdio":
			if p.Format == constants.PluginFormatPortable && (strings.Contains(server.Command, "${") || strings.IndexFunc(server.Command, unicode.IsSpace) >= 0) {
				valid = false
			}
			if server.Command == "" {
				valid = false
			}
			commandForValidation, cwdForValidation := server.Command, server.Cwd
			if p.Format == constants.PluginFormatClaude {
				commandForValidation = strings.ReplaceAll(commandForValidation, "${CLAUDE_PLUGIN_ROOT}", "${PLUGIN_ROOT}")
				cwdForValidation = strings.ReplaceAll(strings.ReplaceAll(cwdForValidation, "${CLAUDE_PLUGIN_ROOT}", "${PLUGIN_ROOT}"), "${CLAUDE_PLUGIN_DATA}", "${PLUGIN_DATA}")
			}
			if server.Cwd != "" && !validWorkingDirectory(root, cwdForValidation) {
				valid = false
			}
			if strings.ContainsAny(commandForValidation, "/\\") && !strings.HasPrefix(commandForValidation, "${PLUGIN_ROOT}/") && !strings.HasPrefix(commandForValidation, "./") {
				valid = false
			}
			if strings.HasPrefix(commandForValidation, "./") || (p.Format == constants.PluginFormatClaude && strings.HasPrefix(commandForValidation, "${PLUGIN_ROOT}/")) {
				_, err := contained(root, strings.TrimPrefix(strings.TrimPrefix(commandForValidation, "${PLUGIN_ROOT}/"), "./"))
				if err != nil {
					valid = false
				}
			}
			if _, exists := server.Env["PLUGIN_ROOT"]; exists {
				valid = false
			}
			if _, exists := server.Env["PLUGIN_DATA"]; exists {
				valid = false
			}
		case "streamable-http", "sse":
			if p.Format == constants.PluginFormatPortable && !validRemote(server.URL, server.Headers) {
				valid = false
			}
			if server.URL == "" {
				valid = false
			}
		default:
			valid = false
		}
		if p.Format == constants.PluginFormatPortable {
			allowed := map[string]bool{"type": true, "url": true, "headers": true}
			if server.Type == "stdio" {
				allowed = map[string]bool{"type": true, "command": true, "args": true, "env": true, "cwd": true}
			}
			for field := range fields {
				if !allowed[field] {
					valid = false
				}
			}
		}
		for _, existing := range p.Servers {
			if existing.Key == key {
				valid = false
			}
		}
		if !valid {
			p.diagnose(key, "invalid_mcp")
			continue
		}
		server.Key = key
		if server.Args == nil {
			server.Args = []string{}
		}
		if server.Env == nil {
			server.Env = map[string]string{}
		}
		if server.Headers == nil {
			server.Headers = map[string]string{}
		}
		p.Servers = append(p.Servers, server)
	}
}
func validWorkingDirectory(root, value string) bool {
	if strings.HasPrefix(value, "${PLUGIN_DATA}") {
		suffix := strings.TrimPrefix(value, "${PLUGIN_DATA}")
		if suffix != "" && !strings.HasPrefix(suffix, "/") {
			return false
		}
		clean := filepath.Clean("." + suffix)
		return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
	}
	relative := strings.TrimPrefix(value, "${PLUGIN_ROOT}")
	if relative != value {
		relative = "." + relative
	} else if !strings.HasPrefix(value, "./") {
		return false
	}
	path, err := contained(root, relative)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func validString(raw json.RawMessage) bool {
	var value string
	return string(raw) != "null" && json.Unmarshal(raw, &value) == nil
}
func validStringArray(raw json.RawMessage) bool {
	var values []json.RawMessage
	if string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, value := range values {
		if !validString(value) {
			return false
		}
	}
	return true
}
func validStringMap(raw json.RawMessage) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return false
	}
	for _, value := range values {
		if !validString(value) {
			return false
		}
	}
	return true
}
func validRemote(address string, headers map[string]string) bool {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return false
	}
	if parsed.Scheme == "http" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return false
		}
	}
	names := map[string]bool{}
	for key, value := range headers {
		lower := strings.ToLower(key)
		if names[lower] || key == "" || strings.ContainsAny(key, "()<>@,;:\"/[]?={} \t\r\n") {
			return false
		}
		for _, r := range key {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("!#$%&' *+-.^_`|~", r) && r != ' ') {
				return false
			}
		}
		names[lower] = true
		for _, r := range value {
			if (r < 32 && r != '\t') || r == 127 {
				return false
			}
		}
		if http.CanonicalHeaderKey(key) == "" {
			return false
		}
	}
	return true
}
