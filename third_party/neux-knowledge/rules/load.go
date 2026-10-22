// Package rules 嵌入并加载按版本发布的知识 Wiki 规则。
package rules

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Version 是规则资源包的稳定版本标识。
const Version = "2026-10-21.lite-direct"

// Resource 是规则包中可单独寻址、校验和读取的逻辑资源。
type Resource struct {
	Path    string // Path 是稳定的包内逻辑路径。
	Version string // Version 是该资源所属规则包的版本。
	SHA256  string // SHA256 是资源原始字节的小写十六进制摘要。
	Content string // Content 是可供宿主直接投影读取的完整资源正文。
}

// Bundle 是为单个知识库选择的完整规则指令包。
type Bundle struct {
	Version      string     // Version 是规则资源包的版本。
	Kind         string     // Kind 是当前支持的通用知识库类型 general。
	WikiLanguage string     // WikiLanguage 是知识库默认编写语言。
	Resources    []Resource // Resources 是按稳定逻辑路径排序的完整资源清单。
	Instructions string     // Instructions 是公共规则和对应类型规则的完整指令。
	SHA256       string     // SHA256 是版本、类型、语言绑定及所有规则文件的稳定摘要。
}

//go:embed weknora
var resources embed.FS

// Load 返回通用知识库的 WeKnora 规则，摘要绑定资源内容、类型和语言。
func Load(kind string, wikiLanguage string) (Bundle, error) {
	return LoadOrganize(kind, wikiLanguage, false)
}

// LoadOrganize 的索引模式不改变内容规则，阶段提示词统一采用 WeKnora 合同。
func LoadOrganize(kind string, wikiLanguage string, _ bool) (Bundle, error) {
	if kind != "general" {
		return Bundle{}, fmt.Errorf("unsupported knowledge kind %q", kind)
	}
	if wikiLanguage != "zh-CN" && wikiLanguage != "en-US" {
		return Bundle{}, fmt.Errorf("unsupported wiki language %q", wikiLanguage)
	}
	roots := []string{"weknora"}
	files := make([]string, 0, 16)
	for _, root := range roots {
		if err := fs.WalkDir(resources, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(name, ".md") {
				files = append(files, name)
			}
			return nil
		}); err != nil {
			return Bundle{}, fmt.Errorf("walk rules %s: %w", root, err)
		}
	}
	sort.Strings(files)

	manifest := make([]Resource, 0, len(files))
	var instructions strings.Builder
	hash := sha256.New()
	writeHashPart(hash, Version)
	writeHashPart(hash, kind)
	writeHashPart(hash, wikiLanguage)
	for _, name := range files {
		content, err := resources.ReadFile(name)
		if err != nil {
			return Bundle{}, fmt.Errorf("read rules %s: %w", name, err)
		}
		resourceHash := sha256.Sum256(content)
		item := Resource{
			Path: name, Version: Version,
			SHA256: hex.EncodeToString(resourceHash[:]), Content: string(content),
		}
		manifest = append(manifest, item)
		writeHashPart(hash, item.Path)
		writeHashPart(hash, item.Version)
		writeHashPart(hash, item.SHA256)
		fmt.Fprintf(&instructions, "\n\n<!-- resource: %s -->\n", name)
		instructions.Write(content)
	}
	languageBinding := fmt.Sprintf("\n\n## 本次任务的语言绑定\nwikiLanguage=%s\n%s", wikiLanguage, languageInstruction(wikiLanguage))
	instructions.WriteString(languageBinding)
	writeHashPart(hash, languageBinding)

	return Bundle{
		Version: Version, Kind: kind, WikiLanguage: wikiLanguage, Resources: manifest,
		Instructions: strings.TrimSpace(instructions.String()),
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

type hashWriter interface{ Write([]byte) (int, error) }

func writeHashPart(w hashWriter, value string) {
	fmt.Fprintf(w, "%d:", len(value))
	w.Write([]byte(value))
}

func languageInstruction(language string) string {
	if language == "en-US" {
		return "新建页面和新主题的标题、摘要、正文使用 en-US；更新既有页面沿用其页面语言。界面语言与原件语言不改变共享知识语言。来源摘录保留原文，专有名词、标准缩写和代码符号保留原样。"
	}
	return "新建页面和新主题的标题、摘要、正文使用 zh-CN；更新既有页面沿用其页面语言。界面语言与原件语言不改变共享知识语言。来源摘录保留原文，专有名词、标准缩写和代码符号保留原样。"
}
