// 来源：Tencent/WeKnora，固定提交 8776a2d92d11ad3e76865d51cf3192e37e2a9976；原完整许可见 upstream/WeKnora.LICENSE；迁入记录见 upstream/manifest.json。
package chunker

import "strings"

// markdownFence tracks top-level backtick and tilde fences during line scans.
// It deliberately does not parse Markdown container blocks (lists/quotes).
type markdownFence struct {
	// marker 代码围栏采用的反引号或波浪号字符。
	marker byte
	// length 开启围栏的连续标记长度。
	length int
}

// consume reports whether line belongs to fenced code, including delimiters.
// The second result identifies opening/closing delimiters, so the profiler can
// count code content without counting fence lines. An unclosed fence remains
// active through the end of the scan.
func (f *markdownFence) consume(line string) (code, delimiter bool) {
	inside := f.length > 0
	// Scanners split on LF; remove the line ending's CR for CRLF documents.
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) {
		return inside, false
	}
	line = line[indent:]
	marker := line[0]
	if marker != '`' && marker != '~' {
		return inside, false
	}
	length := 0
	for length < len(line) && line[length] == marker {
		length++
	}
	if length < 3 {
		return inside, false
	}
	rest := line[length:]
	if inside {
		if marker == f.marker && length >= f.length && strings.Trim(rest, " \t") == "" {
			*f = markdownFence{}
			return true, true
		}
		return true, false
	}
	// Backtick info strings cannot themselves contain backticks.
	if marker == '`' && strings.Contains(rest, "`") {
		return false, false
	}
	f.marker, f.length = marker, length
	return true, true
}
