// 源自 Tencent WeKnora wiki_page.go 的 parseOutLinks；保留去重、别名显示与 slug 规范化算法。
package wiki

import "strings"

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
