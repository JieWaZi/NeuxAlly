// 来源：Tencent/WeKnora 8776a2d，提取 internal/types/source_locator.go 纯定位类型；不迁入数据库方法。
package types

// Source locator types. A locator points from a chunk back into the original
// uploaded file so a citation can open the file at the cited place. Every
// ordinal (page, slide, block, section, row) is 1-based.
const (
	// SourceLocatorPDF is a PDF page, optionally narrowed to a region.
	SourceLocatorPDF = "pdf"
	// SourceLocatorDocx is the Nth body-level paragraph or table of a Word file.
	SourceLocatorDocx = "docx"
	// SourceLocatorSlide is a presentation slide.
	SourceLocatorSlide = "slide"
	// SourceLocatorSheet is a row range of a spreadsheet sheet.
	SourceLocatorSheet = "sheet"
	// SourceLocatorText is a rune range of a plain-text original (md, txt, json).
	SourceLocatorText = "text"
	// SourceLocatorTime is a time range of an audio original.
	SourceLocatorTime = "time"
	// SourceLocatorSection is an EPUB spine item.
	SourceLocatorSection = "section"
)

// SourceLocatorQuoteMax caps display snippets, not stored source evidence.
const SourceLocatorQuoteMax = 300

// SourceLocator is one position in an original file. Only the fields of its
// Type are set; zero values are omitted from JSON and read back as zero.
type SourceLocator struct {
	// Type 原始文件中的位置类型。
	Type string `json:"type"`
	// Mapping records verified text-to-source alignment. Empty means legacy.
	// Mapping 文本到来源的已验证映射精度。
	Mapping string `json:"mapping,omitempty"`
	// Partial means some of the chunk has no source mapping.
	// Partial 当前范围是否存在没有来源映射的部分。
	Partial bool `json:"partial,omitempty"`
	// SourceID identifies a block within the parser result, not a rendered item.
	// SourceID 解析结果中的来源结构块标识。
	SourceID string `json:"source_id,omitempty"`
	// SourceHash binds the locator to the knowledge record’s source revision hash.
	// SourceHash 绑定本次来源版本的摘要。
	SourceHash string `json:"source_hash,omitempty"`
	// Page is the 1-based PDF page.
	// Page 从一开始的 PDF 页码。
	Page int `json:"page,omitempty"`
	// BBox is [x0, y0, x1, y1] as fractions of the page width and height,
	// origin at the top-left corner of the rendered page.
	// BBox PDF 页面上的归一化边界框。
	BBox []float64 `json:"bbox,omitempty"`
	// Block is the 1-based index among a Word file's body paragraphs and tables.
	// Block 从一开始的 Word 段落或表格位置。
	Block int `json:"block,omitempty"`
	// Slide is the 1-based slide number in presentation order.
	// Slide 从一开始的幻灯片页码。
	Slide int `json:"slide,omitempty"`
	// Sheet is the spreadsheet sheet name; RowStart/RowEnd are its 1-based
	// row numbers as shown by spreadsheet applications.
	// Sheet 原表格工作表名称。
	Sheet string `json:"sheet,omitempty"`
	// RowStart 工作表内从一开始的起始行。
	RowStart int `json:"row_start,omitempty"`
	// RowEnd 工作表内从一开始的结束行。
	RowEnd int `json:"row_end,omitempty"`
	// Start/End are rune offsets into the original text file.
	// Start 原文中的 rune 起点。
	Start int `json:"start,omitempty"`
	// End 原文中的 rune 终点。
	End int `json:"end,omitempty"`
	// StartMs/EndMs bound an audio segment.
	// StartMs 音视频片段的起始毫秒。
	StartMs int64 `json:"start_ms,omitempty"`
	// EndMs 音视频片段的结束毫秒。
	EndMs int64 `json:"end_ms,omitempty"`
	// Section is the 1-based EPUB spine index.
	// Section 从一开始的 EPUB 章节位置。
	Section int `json:"section,omitempty"`
	// Title names the section or slide when the source has one.
	// Title 来源章节或幻灯片的标题。
	Title string `json:"title,omitempty"`
	// Quote is the complete cited overlap as parsed. Viewers match it
	// inside the rendered original and against the
	// answer sentence that cites the chunk.
	// Quote 该来源交叠范围的完整引用文本。
	Quote string `json:"quote,omitempty"`
}

// SourceBlock ties a rune range of a parser's Markdown output to the place in
// the original file it came from.
type SourceBlock struct {
	// Start 原文中的 rune 起点。
	Start int
	// End 原文中的 rune 终点。
	End int
	// Locator 该文本对应的真实来源位置。
	Locator SourceLocator
}

// SourceLocators is the per-chunk locator list, stored as a JSON column.
type SourceLocators []SourceLocator
