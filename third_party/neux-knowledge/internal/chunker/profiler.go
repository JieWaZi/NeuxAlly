// 来源：Tencent/WeKnora，固定提交 8776a2d92d11ad3e76865d51cf3192e37e2a9976；原完整许可见 upstream/WeKnora.LICENSE；迁入记录见 upstream/manifest.json。
// Package chunker - profiler.go scans a document once to gather structure
// indicators that drive strategy selection (heading-aware vs. heuristic vs.
// recursive). Profiling is cheap (a few regex passes plus rune counting)
// and runs before any chunking decision is made.
package chunker

import (
	"math"
	"strings"
)

// DocProfile holds the document-level signals used to choose a chunking tier.
//
// The JSON shape (snake_case via struct tags) is part of the public preview
// endpoint API — keep field names stable. Internal callers should use the Go
// field names; only the preview handler relies on the wire format.
type DocProfile struct {
	// TotalChars 全文的 rune 字符数。
	TotalChars int `json:"total_chars"`
	// TotalLines 全文按换行划分的总行数。
	TotalLines int `json:"total_lines"`
	// AvgLineLen 文本行长度的平均值。
	AvgLineLen float64 `json:"avg_line_len"`
	// StdLineLen 文本行长度的标准差。
	StdLineLen float64 `json:"std_line_len"`

	// Markdown structure
	// MdHeadingCounts 各 Markdown 标题级别的出现次数。
	MdHeadingCounts map[int]int `json:"md_heading_counts"` // level (1..6) → count
	// MdHeadingTotal 全部 Markdown 标题的总数。
	MdHeadingTotal int `json:"md_heading_total"`

	// Heuristic indicators
	// NumberedSectionCount 编号章节标记的出现次数。
	NumberedSectionCount int `json:"numbered_section_count"`
	// AllCapsShortLineCount 全大写短行的数量。
	AllCapsShortLineCount int `json:"all_caps_short_line_count"`
	// BlankParagraphBreaks 空行形成的段落边界数量。
	BlankParagraphBreaks int `json:"blank_paragraph_breaks"`
	// FormFeedCount 换页符的出现次数。
	FormFeedCount int `json:"form_feed_count"`
	// VisualSepCount 视觉分隔行的数量。
	VisualSepCount int `json:"visual_sep_count"`
	// GermanChapterCount 德文章节标记的数量。
	GermanChapterCount int `json:"german_chapter_count"`
	// EnglishChapterCount 英文章节标记的数量。
	EnglishChapterCount int `json:"english_chapter_count"`
	// ChineseChapterCount 中文章节标记的数量。
	ChineseChapterCount int `json:"chinese_chapter_count"`
	// RepeatedFooterCount 重复页脚候选的数量。
	RepeatedFooterCount int `json:"repeated_footer_count"`

	// Content characteristics
	// HasTables 文档是否包含表格。
	HasTables bool `json:"has_tables"`
	// HasCode 文档是否包含围栏代码。
	HasCode bool `json:"has_code"`
	// CodeRatio 代码内容占全文字符的比例。
	CodeRatio float64 `json:"code_ratio"`

	// Detected language hints (best-effort)
	// DetectedLangs 按文本特征检测出的语言提示。
	DetectedLangs []string `json:"detected_langs"`
}

// HeadingDensity returns the share of lines that are Markdown headings.
func (p *DocProfile) HeadingDensity() float64 {
	if p.TotalLines == 0 {
		return 0
	}
	return float64(p.MdHeadingTotal) / float64(p.TotalLines)
}

// DominantHeadingLevel returns the heading level (1..6) that should drive
// section splitting. Preference order:
//  1. The lowest level (closest to root) that has at least 3 occurrences —
//     a "real" structural backbone of the document.
//  2. Otherwise the deepest level present at least once — gives finer-grained
//     boundaries for small documents that just have an H1 + a few H2s.
//
// Returns 0 when no Markdown headings exist.
func (p *DocProfile) DominantHeadingLevel() int {
	if p.MdHeadingTotal == 0 {
		return 0
	}
	for level := 1; level <= 6; level++ {
		if p.MdHeadingCounts[level] >= 3 {
			return level
		}
	}
	for level := 6; level >= 1; level-- {
		if p.MdHeadingCounts[level] > 0 {
			return level
		}
	}
	return 0
}

// HeuristicMarkerTotal sums the non-Markdown structural markers.
func (p *DocProfile) HeuristicMarkerTotal() int {
	return p.NumberedSectionCount +
		p.GermanChapterCount + p.EnglishChapterCount + p.ChineseChapterCount +
		p.AllCapsShortLineCount + p.VisualSepCount + p.FormFeedCount
}

// ProfileDocument runs a single pass over text and returns its profile.
func ProfileDocument(text string) *DocProfile {
	p := &DocProfile{
		MdHeadingCounts: make(map[int]int),
	}
	if text == "" {
		return p
	}

	p.TotalChars = len([]rune(text))
	p.FormFeedCount = strings.Count(text, "\f")

	lines := strings.Split(text, "\n")
	p.TotalLines = len(lines)

	// First pass: per-line markers and length stats
	var lengths []float64
	var fence markdownFence
	codeChars := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if code, delimiter := fence.consume(line); code {
			p.HasCode = true
			if !delimiter {
				codeChars += len([]rune(line))
			}
			continue
		}

		runeLen := len([]rune(line))
		lengths = append(lengths, float64(runeLen))

		if matchHeading(line, &p.MdHeadingCounts) {
			p.MdHeadingTotal++
			continue
		}
		if NumberedSectionPattern.MatchString(line) {
			p.NumberedSectionCount++
		}
		if GermanChapterPattern.MatchString(line) {
			p.GermanChapterCount++
		}
		if EnglishChapterPattern.MatchString(line) {
			p.EnglishChapterCount++
		}
		if ChineseChapterPattern.MatchString(line) {
			p.ChineseChapterCount++
		}
		if AllCapsHeadingPattern.MatchString(line) {
			p.AllCapsShortLineCount++
		}
		if VisualSeparatorPattern.MatchString(line) {
			p.VisualSepCount++
		}
		if PageFooterPattern.MatchString(line) {
			p.RepeatedFooterCount++
		}
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			p.HasTables = true
		}
	}

	if len(lengths) > 0 {
		var sum float64
		for _, l := range lengths {
			sum += l
		}
		p.AvgLineLen = sum / float64(len(lengths))
		var variance float64
		for _, l := range lengths {
			d := l - p.AvgLineLen
			variance += d * d
		}
		variance /= float64(len(lengths))
		p.StdLineLen = math.Sqrt(variance)
	}

	if p.TotalChars > 0 {
		p.CodeRatio = float64(codeChars) / float64(p.TotalChars)
	}

	p.BlankParagraphBreaks = strings.Count(text, "\n\n\n")

	// Sample a slice of the document for language detection — avoids paying
	// O(N) scan cost on huge inputs while still giving a stable signal.
	sample := text
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	lang := DetectLanguage(sample)
	p.DetectedLangs = []string{lang}
	if lang == LangMixed {
		// Provide all three for downstream pattern selection.
		p.DetectedLangs = []string{LangEnglish, LangGerman, LangChinese}
	}

	return p
}

// matchHeading checks whether line is an ATX heading and increments the
// appropriate level counter when so. Returns true on match.
func matchHeading(line string, counts *map[int]int) bool {
	m := MarkdownHeadingPattern.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	level := len(m[1])
	if level < 1 || level > 6 {
		return false
	}
	(*counts)[level]++
	return true
}

// StrategyTier identifies which chunking implementation should run.
type StrategyTier string

const (
	TierHeading   StrategyTier = "heading"
	TierHeuristic StrategyTier = "heuristic"
	TierLegacy    StrategyTier = "legacy"
)

// SelectStrategy returns the ordered tier chain to attempt for this document.
// The first tier is the primary choice; subsequent tiers are fallbacks if
// validation rejects the previous output. The "legacy" tier is appended as
// a final safety net so callers always receive at least one chunk-set.
func SelectStrategy(p *DocProfile) []StrategyTier {
	if p == nil {
		return []StrategyTier{TierLegacy}
	}
	var chain []StrategyTier

	// Tier 1 candidate: Markdown heading-aware
	if p.MdHeadingTotal >= 3 && p.HeadingDensity() > 0.005 && p.DominantHeadingLevel() > 0 {
		chain = append(chain, TierHeading)
	}

	// Tier 2 candidate: heuristic boundary detection
	if p.HeuristicMarkerTotal() >= 5 || p.FormFeedCount > 0 ||
		p.GermanChapterCount+p.EnglishChapterCount+p.ChineseChapterCount > 0 {
		chain = append(chain, TierHeuristic)
	}

	// Legacy is the ultimate fallback: always returns chunks even when
	// validation fails, so callers never get an empty result.
	chain = append(chain, TierLegacy)
	return chain
}
