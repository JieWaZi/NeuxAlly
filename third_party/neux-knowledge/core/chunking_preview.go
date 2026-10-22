// 来源：Tencent/WeKnora 8776a2d92d11ad3e76865d51cf3192e37e2a9976 internal/handler/chunker_debug.go。
// 仅将 HTTP 外壳适配为独立核心函数；算法、诊断、统计、父子关系与边界保持原规则。
package core

import (
	"context"
	"github.com/JieWaZi/neux-knowledge/internal/chunker"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const previewMaxChunks = 500

// PreviewChunkingRequest is the body shape accepted by /chunker/preview.
// Text is checked manually below so we can return a friendlier error than
// gin's default "Field validation for 'Text' failed on the 'required' tag".
type PreviewChunkingRequest struct {
	// 样例文本
	Text string `json:"text"`
	// 分块配置
	ChunkingConfig PreviewChunkingPayload `json:"chunking_config"`
}

// PreviewChunkingPayload mirrors the snake_case JSON the rest of the API
// uses for ChunkingConfig fields. We don't reuse types.ChunkingConfig
// directly because it carries unrelated parser rules and table-processing
// metadata that the preview path does not use.
type PreviewChunkingPayload struct {
	// 分块大小
	ChunkSize int `json:"chunk_size"`
	// 分块重叠
	ChunkOverlap int `json:"chunk_overlap"`
	// 分隔符
	Separators []string `json:"separators"`
	// 启用父子分块
	EnableParentChild bool `json:"enable_parent_child"`
	// 父块大小
	ParentChunkSize int `json:"parent_chunk_size"`
	// 子块大小
	ChildChunkSize int `json:"child_chunk_size"`
	// 分块策略
	Strategy string `json:"strategy"`
	// 令牌上限
	TokenLimit int `json:"token_limit"`
	// 语言提示
	Languages []string `json:"languages"`
}

// PreviewChunkResult describes one chunk emitted during preview.
type PreviewChunkResult struct {
	// 分块序号
	Seq int `json:"seq"`
	// 起始字符位置
	Start int `json:"start"`
	// 结束字符位置
	End int `json:"end"`
	// 字符数量
	SizeChars int `json:"size_chars"`
	// 估算令牌数量
	SizeTokensApprox int `json:"size_tokens_approx"`
	// 上下文标题
	ContextHeader string `json:"context_header,omitempty"`
	// 分块正文
	Content string `json:"content"`
}

// PreviewChunkingStats summarizes chunk-size distribution. Computed over
// the FULL chunk set, even when the response truncates to previewMaxChunks
// items, so avg/min/max/stddev reflect the real distribution, not just
// the first N.
type PreviewChunkingStats struct {
	// 全部分块数量
	Count int `json:"count"`
	// 平均字符数量
	AvgChars int `json:"avg_chars"`
	// 最小字符数量
	MinChars int `json:"min_chars"`
	// 最大字符数量
	MaxChars int `json:"max_chars"`
	// 字符数量标准差
	StddevChars int `json:"stddev_chars"`
	// TruncatedTo, when set, is the original chunk count before the
	// response was truncated to previewMaxChunks for transport.
	// 截断前分块总数
	TruncatedTo int `json:"truncated_to,omitempty"`
}

// PreviewChunkingResponse is the body returned by /chunker/preview.
type PreviewChunkingResponse struct {
	// 实际选择策略
	SelectedTier chunker.StrategyTier `json:"selected_tier"`
	// 尝试策略链
	TierChain []chunker.StrategyTier `json:"tier_chain"`
	// 拒绝策略及原因
	Rejected []chunker.TierRejection `json:"rejected"`
	// 文档特征
	Profile *chunker.DocProfile `json:"profile"`
	// 预览分块
	Chunks []PreviewChunkResult `json:"chunks"`
	// 全部分块统计
	Stats PreviewChunkingStats `json:"stats"`
}

// PreviewDocumentChunking 迁入固定上游只读预览；不写数据库、不嵌入、不记录样例。
// 当前输入是与本地 Reader 相同的纯文本，不接入上游 OCR 的 HTML 表格转换。
func PreviewDocumentChunking(ctx context.Context, req PreviewChunkingRequest) (*PreviewChunkingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, &Error{Code: ErrInvalid, Operation: "chunking.preview", Detail: "text is empty"}
	}
	if utf8.RuneCountInString(req.Text) > 64*1024 {
		return nil, &Error{Code: ErrInvalid, Operation: "chunking.preview", Detail: "text exceeds preview limit"}
	}
	// 偏移必须对应当前 Reader 提供的原始文本，与实际入库使用相同输入。
	text := req.Text

	cfg := chunker.NormalizeSplitterConfig(chunker.SplitterConfig{
		ChunkSize:    req.ChunkingConfig.ChunkSize,
		ChunkOverlap: req.ChunkingConfig.ChunkOverlap,
		Separators:   req.ChunkingConfig.Separators,
		Strategy:     req.ChunkingConfig.Strategy,
		TokenLimit:   req.ChunkingConfig.TokenLimit,
		Languages:    req.ChunkingConfig.Languages,
	})

	// Run the splitter on a goroutine so we can honor the request timeout.
	// The splitter is CPU-bound and doesn't accept a context — wrapping
	// here is the cheapest cancellation we can offer.
	type splitResult struct {
		// chunks 保存实际分块。
		chunks []chunker.Chunk
		// diag 保存策略选择诊断。
		diag *chunker.Diagnostics
	}
	resCh := make(chan splitResult, 1)
	go func() {
		var chunks []chunker.Chunk
		var diag *chunker.Diagnostics
		if req.ChunkingConfig.EnableParentChild {
			parentCfg, childCfg := chunker.DeriveParentChildConfigs(
				cfg,
				req.ChunkingConfig.ParentChunkSize,
				req.ChunkingConfig.ChildChunkSize,
			)
			result, parentDiag := chunker.SplitParentChildWithDiagnostics(text, parentCfg, childCfg)
			chunks = make([]chunker.Chunk, len(result.Children))
			for i, child := range result.Children {
				chunks[i] = child.Chunk
			}
			diag = parentDiag
		} else {
			chunks, diag = chunker.SplitWithDiagnostics(text, cfg)
		}
		resCh <- splitResult{chunks: chunks, diag: diag}
	}()

	var sr splitResult
	select {
	case sr = <-resCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	chunks, diag := sr.chunks, sr.diag

	// Diagnostics carries the profile when auto-strategy ran; for explicit
	// strategies the profile is nil and we materialize it here so the UI
	// can still show document stats. Avoids the previous double-pass.
	profile := diag.Profile
	if profile == nil {
		profile = chunker.ProfileDocument(text)
	}

	lang := chunker.LangMixed
	if len(profile.DetectedLangs) > 0 {
		lang = profile.DetectedLangs[0]
	}

	// Compute rune lengths once per chunk; reused for stats and result
	// payload below. Avoids the previous triple-pass over each chunk's
	// content (stats + result + ApproxTokenCount each rune-counted).
	runeLens := make([]int, len(chunks))
	for i, ch := range chunks {
		runeLens[i] = utf8.RuneCountInString(ch.Content)
	}

	// Compute stats over the FULL chunk set first so the metrics stay
	// representative even when we trim the response to previewMaxChunks.
	totalCount := len(chunks)
	stats := computeChunkSizeStats(runeLens)
	if totalCount > previewMaxChunks {
		stats.TruncatedTo = totalCount
		chunks = chunks[:previewMaxChunks]
		runeLens = runeLens[:previewMaxChunks]
	}

	results := make([]PreviewChunkResult, 0, len(chunks))
	for i, ch := range chunks {
		results = append(results, PreviewChunkResult{
			Seq:              ch.Seq,
			Start:            ch.Start,
			End:              ch.End,
			SizeChars:        runeLens[i],
			SizeTokensApprox: chunker.ApproxTokenCountFromRuneLen(runeLens[i], lang),
			ContextHeader:    ch.ContextHeader,
			Content:          ch.Content,
		})
	}

	resp := PreviewChunkingResponse{
		SelectedTier: diag.SelectedTier,
		TierChain:    diag.TierChain,
		Rejected:     diag.Rejected,
		Profile:      profile,
		Chunks:       results,
		Stats:        stats,
	}
	return &resp, nil
}

// computeChunkSizeStats summarizes count / avg / min / max / stddev from
// a pre-computed rune-length slice. Decoupling from chunker.Chunk lets
// the caller compute rune lengths once and reuse them for the response
// payload (avoids a second []rune allocation per chunk).
func computeChunkSizeStats(runeLens []int) PreviewChunkingStats {
	stats := PreviewChunkingStats{Count: len(runeLens)}
	if len(runeLens) == 0 {
		return stats
	}
	var sum, sumSq float64
	minLen, maxLen := math.MaxInt32, 0
	for _, l := range runeLens {
		sum += float64(l)
		sumSq += float64(l) * float64(l)
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}
	avg := sum / float64(len(runeLens))
	variance := sumSq/float64(len(runeLens)) - avg*avg
	if variance < 0 {
		// Float precision can push the variance slightly below zero on
		// near-uniform inputs; clamp so sqrt doesn't return NaN.
		variance = 0
	}
	stats.AvgChars = int(avg + 0.5)
	stats.MinChars = minLen
	stats.MaxChars = maxLen
	stats.StddevChars = int(math.Sqrt(variance) + 0.5)
	return stats
}
