package types

// LintIssue 是只读核查发现；不携带隐式修复权限。
type LintIssue struct {
	Type        string `json:"type"`               // orphan_page、broken_link、empty_content 或 stale_ref。
	Severity    string `json:"severity"`           // warning 或 error，仅用于人工核查。
	PageID      string `json:"pageId"`             // 被核查页面的稳定身份。
	PageSlug    string `json:"pageSlug"`           // 被核查页面的可读地址。
	PageVersion int    `json:"pageVersion"`        // 本次快照内的页面版本。
	TargetID    string `json:"targetId,omitempty"` // 问题目标的页面地址或来源版本身份。
	Description string `json:"description"`        // 基于实际快照的中文问题说明。
}

// LintReport 固定一次一致读取的库版本与全部发现，不持久化第二份问题事实。
type LintReport struct {
	LibraryID        string      `json:"libraryId"`        // 核查的知识库。
	LibraryVersion   int64       `json:"libraryVersion"`   // 库配置版本。
	StructureVersion int64       `json:"structureVersion"` // 导航结构版本。
	PolicyVersion    int64       `json:"policyVersion"`    // 本次读取的权限策略版本。
	PageCount        int         `json:"pageCount"`        // 实际核查的非归档页面数量。
	Issues           []LintIssue `json:"issues"`           // 按页面及规则稳定排序的问题。
}

// ExportInput 选择同一库的已发布页面；空集合表示全部已发布页面。
type ExportInput struct {
	LibraryID string   `json:"libraryId"`         // 导出知识库。
	PageIDs   []string `json:"pageIds,omitempty"` // 显式页面身份，最多一千项且不得重复。
}

// ExportSourceRef 只导出来源身份与定位，不包含原件或原文摘录。
type ExportSourceRef struct {
	SourceID        string        `json:"sourceId"`           // 页面原证据绑定的稳定来源身份。
	SourceVersionID string        `json:"sourceVersionId"`    // 页面原证据绑定的不可变版本。
	Version         int64         `json:"version,omitempty"`  // 快照中仍存在的来源代次。
	SHA256          string        `json:"sha256"`             // 页面原证据保存的来源字节摘要。
	Filename        string        `json:"filename,omitempty"` // 快照中的来源文件显示名称，不能作为输出路径。
	Locator         SourceLocator `json:"locator"`            // 页面已保存的真实位置或 unknown。
	ClaimKind       string        `json:"claimKind"`          // 页面保存的论断类型。
}

// ExportPageMetadata 由同一事务内的页面、库和主题派生，供 frontmatter 和清单共用。
type ExportPageMetadata struct {
	LibraryID         string             `json:"libraryId"`                   // 所属库身份。
	LibraryVersion    int64              `json:"libraryVersion"`              // 库配置版本。
	StructureVersion  int64              `json:"structureVersion"`            // 导航结构版本。
	PolicyVersion     int64              `json:"policyVersion"`               // 权限策略版本。
	PageID            string             `json:"pageId"`                      // 页面稳定身份。
	PageVersion       int                `json:"pageVersion"`                 // 已发布正文版本。
	Slug              string             `json:"slug"`                        // 页面地址，仅用于显示和关系映射。
	Title             string             `json:"title"`                       // 已发布标题。
	PageType          string             `json:"pageType"`                    // 页面知识类型。
	WikiLanguage      string             `json:"wikiLanguage"`                // 已发布正文语言。
	ThemeID           string             `json:"themeId"`                     // 当前主主题身份。
	ThemePath         []string           `json:"themePath"`                   // 从真实主题父链派生的名称链。
	SortOrder         int                `json:"sortOrder"`                   // 页面在当前主题中的排序。
	Sources           []ExportSourceRef  `json:"sources"`                     // 来源身份和定位，无原文。
	Code              []CodeRef          `json:"code"`                        // 已保存的软件实现证据。
	SourcePurgeReview *SourcePurgeReview `json:"sourcePurgeReview,omitempty"` // 仅包含待核查事实与不可用身份的清理标记。
}

// MarkdownFile 是可移交的导出副本；修改它不会回写 SQLite。
type MarkdownFile struct {
	Path     string             `json:"path"`               // 基于页面身份摘要的安全文件名。
	SHA256   string             `json:"sha256"`             // 完整 Markdown 文件字节摘要。
	Metadata ExportPageMetadata `json:"metadata"`           // 生成该文件时的快照身份。
	Markdown string             `json:"markdown,omitempty"` // JSON 形式的 YAML frontmatter 加原始知识正文。
}

// MarkdownExport 是一个 SQLite 一致快照的导出清单与 Markdown 文件集合。
type MarkdownExport struct {
	FormatVersion    int            `json:"formatVersion"`    // 当前导出格式版本，固定为 1。
	LibraryID        string         `json:"libraryId"`        // 本次唯一来源知识库。
	LibraryVersion   int64          `json:"libraryVersion"`   // 一致快照中的库配置版本。
	StructureVersion int64          `json:"structureVersion"` // 一致快照中的导航版本。
	PolicyVersion    int64          `json:"policyVersion"`    // 一致快照中的权限策略版本。
	Files            []MarkdownFile `json:"files"`            // 按稳定页面身份排序的已发布文件。
}
