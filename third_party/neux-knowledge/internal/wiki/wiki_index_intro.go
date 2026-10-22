package wiki

import (
	"regexp"
	"strings"
)

var indexLeadingTitle = regexp.MustCompile(`^ {0,3}#([ \t]+|$)`)
var indexSetextTitle = regexp.MustCompile(`^ {0,3}=+[ \t]*$`)

// NormalizeIndexIntro 固定索引开头的一级标题，保留概述、目录和代码正文。
// 仅用于索引生成和只读投影，不修改普通 Wiki 标题或读取时的持久记录。
func NormalizeIndexIntro(content string) string {
	lines := strings.Split(strings.TrimRight(content, "\r\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return "# Wiki Index"
	}
	if indexLeadingTitle.MatchString(lines[0]) {
		lines[0] = "# Wiki Index"
		return strings.Join(lines, "\n")
	}
	if len(lines) > 1 && !strings.HasPrefix(lines[0], "    ") && !strings.HasPrefix(lines[0], "\t") && indexSetextTitle.MatchString(lines[1]) {
		return strings.Join(append([]string{"# Wiki Index"}, lines[2:]...), "\n")
	}
	return "# Wiki Index\n\n" + strings.Join(lines, "\n")
}
