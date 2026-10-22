package types

// SourceLocator 只表达 Reader 实际确认的位置；unknown 不推造页码。
type SourceLocator struct {
	Kind      string `json:"kind"`                // page、line、section、range 或 unknown。
	Page      int    `json:"page,omitempty"`      // 实际页码，未知为 0。
	StartLine int    `json:"startLine,omitempty"` // 原文起始行，未知为 0。
	EndLine   int    `json:"endLine,omitempty"`   // 原文结束行。
	Section   string `json:"section,omitempty"`   // 实际章节定位。
}

// SourceRef 固定论断所依据的不可变来源归因及可选摘录。
type SourceRef struct {
	SourceID        string        `json:"sourceId"`        // 稳定来源身份。
	SourceVersionID string        `json:"sourceVersionId"` // 不可变来源版本。
	SHA256          string        `json:"sha256"`          // 整份原件字节摘要。
	Locator         SourceLocator `json:"locator"`         // 已确认位置或 unknown；文档级归因为零值。
	Excerpt         string        `json:"excerpt"`         // 实际原文摘录；文档级归因不保存模型生成的摘录。
	ExcerptHash     string        `json:"excerptHash"`     // 摘录 UTF-8 字节的 SHA-256；文档级归因为空。
	ClaimKind       string        `json:"claimKind"`       // source_statement、inference、disagreement 或仅标识来源身份的 document_context。
}

// CodeRef 固定软件实现证据的仓库与提交。
type CodeRef struct {
	Repository string `json:"repository"`          // 已授权代码仓库身份。
	Commit     string `json:"commit"`              // 实际读取的提交。
	Path       string `json:"path"`                // 仓库内相对路径。
	Symbol     string `json:"symbol,omitempty"`    // 已确认符号。
	StartLine  int    `json:"startLine,omitempty"` // 起始行。
	EndLine    int    `json:"endLine,omitempty"`   // 结束行。
	VerifiedAt int64  `json:"verifiedAt"`          // 实际核验时间，Unix 毫秒。
}

// PageEvidence 是唯一权威证据；旧 SourceRefs 只从这里派生。
type PageEvidence struct {
	Sources []SourceRef `json:"sources"` // 原件证据。
	Code    []CodeRef   `json:"code"`    // 实现证据。
}
