package plugins

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"image"
	_ "image/gif" // 注册可验证的插件图标格式。
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/packagefiles"
)

const maxPresentationBytes = 256 * 1024

// 插件展示资料
type Presentation struct {
	// 展示名称
	DisplayName string `json:"displayName,omitempty"`
	// 简短介绍
	ShortDescription string `json:"shortDescription,omitempty"`
	// 完整介绍
	LongDescription string `json:"longDescription,omitempty"`
	// 开发者
	DeveloperName string `json:"developerName,omitempty"`
	// 分类
	Category string `json:"category,omitempty"`
	// 官方网站
	WebsiteURL string `json:"websiteURL,omitempty"`
	// 隐私政策
	PrivacyPolicyURL string `json:"privacyPolicyURL,omitempty"`
	// 服务条款
	TermsOfServiceURL string `json:"termsOfServiceURL,omitempty"`
}

func (p *Package) loadPresentation(root string, raw map[string]json.RawMessage) {
	p.Presentation = Presentation{DisplayName: p.Manifest.Name, ShortDescription: p.Manifest.Description, DeveloperName: p.Manifest.Author.Name, WebsiteURL: publicURL(p.Manifest.Homepage)}
	presentation := map[string]json.RawMessage{}
	icon := ""
	if p.Format == constants.PluginFormatPortable {
		var settings map[string]json.RawMessage
		if value, exists := p.Manifest.Extensions["com.openai"]; exists {
			if json.Unmarshal(value, &settings) != nil || settings == nil {
				p.diagnose("extensions.com.openai", "invalid_extension")
			}
		} else if overlay, err := contained(root, ".codex-plugin/plugin.json"); err == nil {
			settings, _ = readObject(overlay)
		}
		if settings != nil {
			if value, exists := settings["interface"]; exists && (json.Unmarshal(value, &presentation) != nil || presentation == nil) {
				p.diagnose("interface", "invalid_component")
				presentation = map[string]json.RawMessage{}
			}
			for _, key := range []string{"apps", "hooks", "mcpServerIds", "onboardingSkill"} {
				if _, exists := settings[key]; exists {
					p.diagnose(key, "unsupported_component")
				}
			}
		}
	} else if p.Format == constants.PluginFormatCodex {
		if value, exists := raw["interface"]; exists && (json.Unmarshal(value, &presentation) != nil || presentation == nil) {
			p.diagnose("interface", "invalid_component")
			presentation = map[string]json.RawMessage{}
		}
	} else {
		presentation = raw
		icon = metadataString(raw, "icon")
		p.Presentation.PrivacyPolicyURL = publicURL(metadataString(raw, "privacyPolicyUrl"))
		p.Presentation.TermsOfServiceURL = publicURL(metadataString(raw, "termsOfServiceUrl"))
	}
	for key, target := range map[string]*string{
		"displayName": &p.Presentation.DisplayName, "shortDescription": &p.Presentation.ShortDescription,
		"longDescription": &p.Presentation.LongDescription, "developerName": &p.Presentation.DeveloperName, "category": &p.Presentation.Category,
	} {
		if text := metadataString(presentation, key); text != "" {
			*target = text
		}
	}
	for key, target := range map[string]*string{"websiteURL": &p.Presentation.WebsiteURL, "privacyPolicyURL": &p.Presentation.PrivacyPolicyURL, "termsOfServiceURL": &p.Presentation.TermsOfServiceURL} {
		if text := publicURL(metadataString(presentation, key)); text != "" {
			*target = text
		}
	}
	for _, key := range []string{"logo", "composerIcon"} {
		if value := metadataString(presentation, key); value != "" {
			icon = value
			break
		}
	}
	// 本机扩展的既有展示资料继续可读，不从外部资料派生执行设置。
	var ally map[string]json.RawMessage
	if json.Unmarshal(p.Manifest.Extensions[constants.PluginExtensionNamespace], &ally) == nil {
		if value := metadataString(ally, "displayName"); value != "" {
			p.Presentation.DisplayName = value
		}
		if icon == "" {
			icon = metadataString(ally, "icon")
		}
		if value := publicURL(metadataString(ally, "privacyPolicy")); value != "" && p.Presentation.PrivacyPolicyURL == "" {
			p.Presentation.PrivacyPolicyURL = value
		}
		if value := publicURL(metadataString(ally, "termsOfService")); value != "" && p.Presentation.TermsOfServiceURL == "" {
			p.Presentation.TermsOfServiceURL = value
		}
	}
	if icon != "" {
		if value, err := readIcon(root, icon); err == nil {
			p.Icon = value
		} else {
			p.diagnose("icon", "invalid_asset")
		}
	}
	for _, filename := range []string{"README.md", "readme.md"} {
		if _, err := os.Lstat(filepath.Join(root, filename)); os.IsNotExist(err) {
			continue
		}
		if content, err := readBounded(root, filename, maxPresentationBytes); err == nil && utf8.Valid(content) && !bytes.ContainsRune(content, 0) {
			p.Readme = string(content)
		} else {
			p.diagnose(filename, "invalid_documentation")
		}
		break
	}
}

func metadataString(fields map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(fields[key], &value)
	return strings.TrimSpace(value)
}

func publicURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil {
		return ""
	}
	return value
}

func readBounded(root, relative string, limit int64) ([]byte, error) {
	path, err := contained(root, relative)
	if err != nil {
		return nil, status_error.PluginContentInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, storageFailure(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, status_error.PluginContentInvalid
	}
	if info.Size() > limit {
		return nil, status_error.PluginContentLimitExceeded
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, storageFailure(err)
	}
	if int64(len(content)) > limit {
		return nil, status_error.PluginContentLimitExceeded
	}
	return content, nil
}

func readIcon(root, relative string) (string, error) {
	content, err := readBounded(root, relative, maxPresentationBytes)
	if err != nil {
		return "", err
	}
	mime := ""
	if strings.EqualFold(filepath.Ext(relative), ".svg") {
		if !staticSVG(content) {
			return "", status_error.PluginContentInvalid
		}
		mime = "image/svg+xml"
	} else {
		config, format, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 {
			return "", status_error.PluginContentInvalid
		}
		switch format {
		case "png", "jpeg", "gif":
			mime = "image/" + format
		default:
			return "", status_error.PluginContentInvalid
		}
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

// staticSVG 仅接受静态矢量图形，拒绝脚本、事件、样式与所有外部资源引用。
func staticSVG(content []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	depth, elements := 0, 0
	allowed := map[string]bool{"svg": true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true, "defs": true, "linearGradient": true, "radialGradient": true, "stop": true, "clipPath": true, "mask": true, "title": true, "desc": true}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return elements > 0 && depth == 0
		}
		if err != nil {
			return false
		}
		switch value := token.(type) {
		case xml.StartElement:
			if !allowed[value.Name.Local] || (value.Name.Space != "" && value.Name.Space != "http://www.w3.org/2000/svg") || (depth == 0 && (value.Name.Local != "svg" || elements > 0)) {
				return false
			}
			elements++
			depth++
			if depth > 32 || elements > 4096 {
				return false
			}
			for _, attribute := range value.Attr {
				name, text := strings.ToLower(attribute.Name.Local), strings.ToLower(attribute.Value)
				if strings.HasPrefix(name, "on") || name == "style" || name == "href" || name == "src" || strings.Contains(text, "\\") || (attribute.Name.Space != "" && attribute.Name.Space != "xmlns") {
					return false
				}
				if strings.Contains(text, "url(") && !(strings.HasPrefix(text, "url(#") && strings.HasSuffix(text, ")") && strings.Count(text, "url(") == 1 && !strings.ContainsAny(text, "\\\"'\n\r")) {
					return false
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return false
		case xml.ProcInst:
			// XML 编码声明不加载资源；其他处理指令（包括样式表）不能进入图标。
			if value.Target != "xml" || depth != 0 || elements != 0 {
				return false
			}
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(value)) != 0 {
				return false
			}
		}
	}
}

// ReadSkill 返回版本已发现技能的正文，不接受任意文件路径。
func ReadSkill(root string, pkg Package, key string) (Skill, string, error) {
	for _, skill := range pkg.Skills {
		if skill.Key != key {
			continue
		}
		content, err := readBounded(root, filepath.Join(skill.Path, "SKILL.md"), maxMetadataBytes)
		if err != nil {
			return Skill{}, "", err
		}
		if !utf8.Valid(content) || bytes.ContainsRune(content, 0) {
			return Skill{}, "", status_error.PluginContentInvalid
		}
		return skill, string(content), nil
	}
	return Skill{}, "", status_error.PluginContentInvalid
}

// ReadSkillResources 只暴露已发现技能目录的文件，根技能过滤插件清单与其他组件。
func ReadSkillResources(root string, pkg Package, key, requested string) ([]packagefiles.ResourceEntry, *packagefiles.ResourceFile, error) {
	for _, skill := range pkg.Skills {
		if skill.Key != key {
			continue
		}
		directory, err := contained(root, skill.Path)
		if err != nil {
			return nil, nil, status_error.PluginContentInvalid
		}
		excluded := []string{".git"}
		if filepath.Clean(skill.Path) == "." {
			excluded = append(excluded, ".codex-plugin", ".claude-plugin", "plugin.json", "mcp.json", ".mcp.json", ".app.json", "provenance.json", "README.md", "readme.md", "skills", "hooks", "agents", "commands")
			for _, other := range pkg.Skills {
				if other.Key != skill.Key && filepath.Clean(other.Path) != "." {
					excluded = append(excluded, filepath.Clean(other.Path))
				}
			}
		}
		resources, err := pluginArchive.Resources(directory, excluded)
		if err != nil {
			return nil, nil, err
		}
		if requested == "" {
			return resources, nil, nil
		}
		file, err := pluginArchive.ReadResource(directory, requested, resources)
		if err != nil {
			return nil, nil, err
		}
		return resources, &file, nil
	}
	return nil, nil, status_error.PluginContentInvalid
}
