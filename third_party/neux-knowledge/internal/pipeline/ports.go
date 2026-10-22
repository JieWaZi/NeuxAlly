// Package pipeline 复用固定 WeKnora Lite 的批次、提取和归并逻辑。
package pipeline

import (
	"context"
	"encoding/json"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"github.com/JieWaZi/neux-knowledge/internal/modelcontext"
)

// ReadRequest 固定本轮允许读取的不可变原件，不授予任意文件系统访问。
type ReadRequest struct {
	SourceID        string `json:"sourceId"`        // 稳定来源身份。
	SourceVersionID string `json:"sourceVersionId"` // 不可变来源版本。
	SHA256          string `json:"sha256"`          // 原始字节摘要。
	MIME            string `json:"mime"`            // 实际媒体类型。
	Filename        string `json:"filename"`        // 原件文件名，仅供选择读取能力。
	Raw             []byte `json:"raw,omitempty"`   // 完整原始字节，不能按文件名伪造正文。
	Scope           string `json:"scope"`           // 明确要求的范围；首版为 all。
	MaxRunes        int    `json:"maxRunes"`        // 实际阅读预算，超出必须返回 partial。
}

// Chunk 保留 WeKnora 的稳定块顺序；每块绑定 Reader 实际位置。
type Chunk struct {
	EndAt      int                 `json:"endAt"`   // 本块对应原文的rune结束位置，不含结束点。
	ID         string              `json:"id"`      // 本次原件内稳定块身份。
	ChunkIndex int                 `json:"index"`   // 原文顺序。
	StartAt    int                 `json:"startAt"` // 原文 rune 偏移。
	Content    string              `json:"content"` // 实际读到的原文。
	ChunkType  string              `json:"type"`    // 当前仅 text。
	Locator    types.SourceLocator `json:"locator"` // 已核实的位置。
}

// ReadArtifact 保存 Reader 的实际覆盖，不把 partial 升格 complete。
type ReadArtifact struct {
	SourceVersionID string          `json:"sourceVersionId"` // 实际读取的不可变版本。
	SHA256          string          `json:"sha256"`          // 实际读取字节摘要。
	Status          string          `json:"status"`          // complete、partial、unsupported 或 failed。
	Scope           string          `json:"scope"`           // 实际执行的规定范围。
	Blocks          []*Chunk        `json:"blocks"`          // 原文证据块，不能含虚构页码。
	Coverage        json.RawMessage `json:"coverage"`        // 可复核覆盖与读取能力。
	Diagnostic      string          `json:"diagnostic"`      // 真实限制或错误。
}

// ReaderPort 由宿主提供真实 PDF/DOCX 等能力；MD 可使用 MarkdownReader。
type ReaderPort interface {
	Read(context.Context, ReadRequest) (ReadArtifact, error)
}

// ModelMessage 是一次独立模型请求中的明确角色与文本。
type ModelMessage struct {
	Role    string `json:"role"`    // system、user 或该次续写的 assistant。
	Content string `json:"content"` // 此次请求的完整文本。
}

// GenerateRequest 是知识核心到模型的唯一端口，不授予工具或文件权限。
type GenerateRequest struct {
	JobID           string         `json:"jobId"`                     // 所属知识任务。
	SourceVersionID string         `json:"sourceVersionId,omitempty"` // 单来源阶段的不可变版本。
	StageID         string         `json:"stageId"`                   // 纯知识调用诊断身份。
	Stage           string         `json:"stage"`                     // map、cite、dedup、summary、taxonomy、reduce 或 index。
	Messages        []ModelMessage `json:"messages"`                  // 本次独立上下文，禁止跨调用追加历史。
	MaxTokens       int            `json:"maxTokens"`                 // 上游完成预算。
	Temperature     float64        `json:"temperature"`               // 上游温度。
	Thinking        bool           `json:"thinking"`                  // 默认关闭思考。
}

// GenerateResult 保留模型原文、停止原因和可用的真实用量。
type GenerateResult struct {
	Diagnostics  *types.ModelDiagnostics `json:"diagnostics,omitempty"` // 本次CLI实际能力与配置诊断。
	Text         string                  `json:"text"`                  // 模型直接返回的全文，由核心解析。
	FinishReason string                  `json:"finishReason"`          // 原生模型停止原因。
	Usage        *types.ModelUsage       `json:"usage,omitempty"`       // 原生用量，未知为空。
}

// Generator 每次调用均独立，取消由传入 Context 传播。
type Generator interface {
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}

// Input 指定已经持久接纳的任务与外部能力。
type Input struct {
	OnProgress      func(context.Context)                                             // 来源开始或重排事实已提交后的既有资源变更通知。
	Instruction     string                                                            // 用户本次明确补充要求，仅进入当前任务的业务指令作用域。
	Embed           func(context.Context, []string) ([][]float32, error)              // 冻结嵌入能力，仅供上游目录相似排序。
	JobID           string                                                            // 已接纳的知识任务。
	Reader          ReaderPort                                                        // 实际原件阅读能力。
	Generator       Generator                                                         // 直接模型文本端口。
	PrepareDocument func(context.Context, string, ReadArtifact) (ReadArtifact, error) // 真实分块和向量索引。
	PublishGuard    func(context.Context) (func(), error)                             // 宿主授权发布闸门。
	OnPublished     func(context.Context, *types.Receipt)                             // 实际提交后的通知。
	MaxReadRunes    int                                                               // 原件阅读上限，零为不限制。
}

// Result 返回持久任务与真实回执。
type Result struct {
	Observer bool           `json:"observer,omitempty"` // 已有库内整理持有运行权。
	JobID    string         `json:"jobId"`              // 知识任务标识。
	State    string         `json:"state"`              // 持久任务状态。
	Receipt  *types.Receipt `json:"receipt,omitempty"`  // 最后提交回执。
}

// extractedItem 沿用上游候选语义，类型在 updates 中固定。
type extractedItem struct {
	Name         string   `json:"name"`                    // 知识名称。
	Slug         string   `json:"slug"`                    // 稳定语义地址。
	Aliases      []string `json:"aliases,omitempty"`       // 已核对别名。
	Description  string   `json:"description"`             // 页面定位说明。
	Details      string   `json:"details"`                 // 来源支持的详情；无块级引用时沿上游回退作为合成输入。
	SourceChunks []string `json:"source_chunks,omitempty"` // 实际证据块。
}

// newSlugFromCitation 沿用上游 cite 新发现知识合同。
type newSlugFromCitation struct {
	Type         string   `json:"type,omitempty"`          // 页面类型。
	Name         string   `json:"name"`                    // 知识名称。
	Slug         string   `json:"slug"`                    // 稳定语义地址。
	Aliases      []string `json:"aliases,omitempty"`       // 别名。
	Description  string   `json:"description"`             // 定位说明。
	Details      string   `json:"details"`                 // 候选描述。
	SourceChunks []string `json:"source_chunks,omitempty"` // 实际证据块句柄。
}

// SlugUpdate 是来自一个实际来源的知识增量。
type SlugUpdate struct {
	Slug              string `json:"slug"`              // 目标页面地址。
	DocTitle          string `json:"docTitle"`          // 文档标题。
	KnowledgeID       string `json:"knowledgeId"`       // 稳定文档身份，对应本地SourceID。
	SourceRef         string `json:"sourceRef"`         // 原上游文档级引用。
	Language          string `json:"language"`          // 本阶段业务语言。
	SummaryBody       string `json:"summaryBody"`       // 文档摘要正文。
	SummaryLine       string `json:"summaryLine"`       // 文档一句摘要。
	RetractDocContent string `json:"retractDocContent"` // 需要撤回的旧版本贡献。
	DocSummary        string `json:"docSummary"`        // 对编辑器提供的文档背景。

	Type            string        `json:"type"`            // 具体页面类型。
	Item            extractedItem `json:"item"`            // 候选知识。
	SourceVersionID string        `json:"sourceVersionId"` // 不可变来源身份。
	SourceChunks    []string      `json:"sourceChunks"`    // 核心已解析的证据块。
}
type chunkBatch struct {
	chunks       []*Chunk                  // 本批原文。
	handles      *modelcontext.HandleTable // 生成所见短句柄与原文块映射。
	totalRuneLen int                       // 实际字符预算。
}
type wikiTaxonomyItem struct {
	slug     string // 语义地址。
	title    string // 展示名称。
	pageType string // 页面类型。
	about    string // 定位说明。
}
type wikiTaxonomyNode struct {
	children map[string]*wikiTaxonomyNode // 下级目录。
}

const (
	maxRunesPerCitationBatch    = 12000
	maxCitationBatchConcurrency = 4
	dedupCandidateTopK          = 5
	dedupCandidateScoreFloor    = 0.08
	dedupSmallCorpusBypass      = 25
)
