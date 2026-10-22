// 来源：Tencent/WeKnora 8776a2d，完整迁入 internal/types/knowledgebase.go 的分块与解析规则字段。
package types

// ParserEngineRule maps a set of file types to a specific parser engine.
type ParserEngineRule struct {
	// FileTypes 匹配的文件扩展名集合。
	FileTypes []string `yaml:"file_types" json:"file_types" extensions:"x-agent"`
	// Engine 所选解析引擎标识。
	Engine string `yaml:"engine"     json:"engine" extensions:"x-agent"`
	// XLSXFirstRowAsHeader restores row-1 column context for flat XLSX tables.
	// nil preserves the parser default; an explicit false disables the mode.
	// XLSXFirstRowAsHeader 是否把表格首行用作列名，省略保留解析器默认。
	XLSXFirstRowAsHeader *bool `yaml:"xlsx_first_row_as_header,omitempty" json:"xlsx_first_row_as_header,omitempty" extensions:"x-agent"`
}

// ChunkingConfig represents the document splitting configuration
type ChunkingConfig struct {
	// Chunk size in characters. Empty on create uses the server default.
	// ChunkSize 普通块的目标字符数，零值采用上游默认。
	ChunkSize int `yaml:"chunk_size"    json:"chunk_size" minimum:"100" maximum:"10000" extensions:"x-agent"`
	// Chunk overlap in characters.
	// ChunkOverlap 相邻块的重叠字符数，上游归一化将零值视为默认。
	ChunkOverlap int `yaml:"chunk_overlap" json:"chunk_overlap" minimum:"0" maximum:"5000" extensions:"x-agent"`
	// Separators, in the order they are tried.
	// Separators 按优先顺序尝试的文本分隔符。
	Separators []string `yaml:"separators"    json:"separators" extensions:"x-agent,x-agent-maximum=20"`
	// ParserEngineRules configures which parser engine to use for each file type.
	// When empty, DefaultParserEngine is used (builtin/simple routing, except
	// types that only a specific engine can parse: ppt/pptx fall back to
	// markitdown). A linked anydoc binding is preferred for every type it
	// converts.
	// ParserEngineRules 按文件类型选择解析器的完整规则；当前核心尚不支持非空规则。
	ParserEngineRules []ParserEngineRule `yaml:"parser_engine_rules,omitempty" json:"parser_engine_rules,omitempty" extensions:"x-agent,x-agent-response"`
	// EnableParentChild splits into large parent chunks and small child chunks.
	// Search matches a child and returns its parent.
	// EnableParentChild 是否拆为提供上下文的父块和用于检索的子块。
	EnableParentChild bool `yaml:"enable_parent_child,omitempty" json:"enable_parent_child,omitempty" extensions:"x-agent"`
	// ParentChunkSize is the size of parent chunks (default: 4096).
	// Only used when EnableParentChild is true.
	// ParentChunkSize 父块的目标字符数，默认四千零九十六。
	ParentChunkSize int `yaml:"parent_chunk_size,omitempty" json:"parent_chunk_size,omitempty" minimum:"1" maximum:"20000" extensions:"x-agent"`
	// ChildChunkSize is the size of child chunks used for embedding (default: 384).
	// Only used when EnableParentChild is true.
	// ChildChunkSize 子块的目标字符数，默认三百八十四。
	ChildChunkSize int `yaml:"child_chunk_size,omitempty" json:"child_chunk_size,omitempty" minimum:"1" maximum:"10000" extensions:"x-agent"`
	// Strategy selects how documents are split: auto, heading, heuristic,
	// recursive or legacy. Empty keeps the server default.
	// Strategy 自适应、标题、启发式或递归分块策略。
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty" enums:"auto,heading,heuristic,recursive,legacy" extensions:"x-agent"`
	// TokenLimit caps chunk size in approximate tokens. 0 uses ChunkSize as a character count.
	// TokenLimit 近似 token 上限，零值使用字符数预算。
	TokenLimit int `yaml:"token_limit,omitempty" json:"token_limit,omitempty" minimum:"0" maximum:"100000" extensions:"x-agent"`
	// Languages hints the splitter, for example de or zh. Empty detects the language from the document.
	// Languages 用于多语言边界识别的语言提示。
	Languages []string `yaml:"languages,omitempty" json:"languages,omitempty" extensions:"x-agent,x-agent-maximum=20"`
	// TableMetadataInstructions is extra guidance for CSV and Excel summaries. The output format stays fixed.
	// TableMetadataInstructions 表格摘要的额外指导；当前核心尚未接入非空指导。
	TableMetadataInstructions string `yaml:"table_metadata_instructions,omitempty" json:"table_metadata_instructions,omitempty" extensions:"x-agent,x-agent-detail,x-agent-max-bytes=4000"`
}

// DefaultChunkingConfig 返回固定上游 8776 的新知识库创建表单默认值。
// 算法自身对显式零值配置的归一化仍由迁入的 chunker 原样执行。
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		ChunkSize:         512,
		ChunkOverlap:      80,
		Separators:        []string{"\n\n", "\n", "。", "！", "？", ";", "；"},
		EnableParentChild: true,
		ParentChunkSize:   4096,
		ChildChunkSize:    384,
		Strategy:          "auto",
		Languages:         []string{},
	}
}

// ValidateChunkingConfig 明确拒绝当前纯文本核心尚未接入的解析与表格摘要能力。
// 分块大小、重叠与策略的默认和保护规则由上游算法负责，不在此重复改写。
func ValidateChunkingConfig(cfg *ChunkingConfig) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.ParserEngineRules) > 0 {
		return &Error{Code: ErrInvalid, Operation: "chunking.config", Detail: "parser_engine_rules require an unavailable parser engine"}
	}
	if cfg.TableMetadataInstructions != "" {
		return &Error{Code: ErrInvalid, Operation: "chunking.config", Detail: "table_metadata_instructions require an unavailable table metadata pipeline"}
	}
	return nil
}
