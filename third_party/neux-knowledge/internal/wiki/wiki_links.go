// 源自 Tencent WeKnora wiki_page.go 的 parseOutLinks；保留去重、别名显示与 slug 规范化算法。
package wiki

import (
	"path"
	"strings"
)

// ParseOutLinks 从真实 Markdown 正文投影外链，不接受调用方伪造关系字段。
func ParseOutLinks(content string) []string {
	// 复用上游链接扫描器的代码区段识别，示例代码中的链接不成为关系。
	_, used := computeForbiddenSpans(content)
	safe := map[string]bool{}
	for slug := range used {
		safe[normalizeSlug(slug)] = true
	}
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var links []string

	for _, match := range matches {
		if len(match) > 1 {
			slug := strings.TrimSpace(match[1])
			// Handle [[slug|display name]] format — slug is the first part
			if parts := strings.SplitN(slug, "|", 2); len(parts) == 2 {
				slug = strings.TrimSpace(parts[0])
			}
			slug = normalizeSlug(slug)
			if slug != "" && safe[slug] && !seen[slug] {
				seen[slug] = true
				links = append(links, slug)
			}
		}
	}
	return links
}

// RewritePageLinks 沿同一Markdown扫描边界修订真实链接，保留代码和其他链接中的示例。
// newSlug为空时移除链接语法，保留别名或页面可读名称。
func RewritePageLinks(content, oldSlug, newSlug string) string {
	spans, _ := computeForbiddenSpans(content)
	var out strings.Builder
	last, blockedEnd := 0, 0
	for _, sp := range spans {
		raw := content[sp.start:sp.end]
		match := wikiLinkRegex.FindStringIndex(raw)
		if match == nil || match[0] != 0 || match[1] != len(raw) {
			if sp.end > blockedEnd {
				blockedEnd = sp.end
			}
			continue
		}
		if sp.start < blockedEnd || sp.start < last {
			continue
		}
		inner := raw[2 : len(raw)-2]
		if normalizeSlug(extractWikiSlug(inner)) != normalizeSlug(oldSlug) {
			continue
		}
		display := ""
		if pipe := strings.IndexByte(inner, '|'); pipe >= 0 {
			display = inner[pipe:]
		}
		replacement := "[[" + newSlug + display + "]]"
		if newSlug == "" {
			replacement = strings.ReplaceAll(path.Base(oldSlug), "-", " ")
			if display != "" {
				replacement = display[1:]
			}
		}
		out.WriteString(content[last:sp.start])
		out.WriteString(replacement)
		last = sp.end
	}
	if last == 0 {
		return content
	}
	out.WriteString(content[last:])
	return out.String()
}
