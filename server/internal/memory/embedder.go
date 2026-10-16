package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
)

// Embedder 是记忆嵌入端口，对齐 记忆嵌入调用契约。
type Embedder interface {
	// Embed 返回单条文本的嵌入向量。
	Embed(ctx context.Context, text string, action constants.MemoryEmbeddingAction) ([]float32, error)
	// EmbedBatch 返回多条文本的嵌入向量，与输入一一对应。
	EmbedBatch(ctx context.Context, texts []string, action constants.MemoryEmbeddingAction) ([][]float32, error)
}

// EmbeddingConfig 是 OpenAI 兼容嵌入端点的配置。
type EmbeddingConfig struct {
	// BaseURL 是 OpenAI 兼容服务的基础地址，例如 https://api.openai.com/v1。
	BaseURL string
	// APIKey 是访问嵌入服务的凭据。
	APIKey string
	// Model 是嵌入模型名。
	Model string
}

// Complete 返回三项配置是否齐全；齐全才启用语义检索。
func (c EmbeddingConfig) Complete() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		c.APIKey != "" &&
		strings.TrimSpace(c.Model) != ""
}

// embeddingRequest 是 OpenAI 兼容 /embeddings 的请求体。
type embeddingRequest struct {
	// Model 是嵌入模型名。
	Model string `json:"model"`
	// Input 是待嵌入文本列表。
	Input []string `json:"input"`
	// EncodingFormat 要求以浮点数组返回向量。
	EncodingFormat string `json:"encoding_format"`
}

// embeddingResponse 是 OpenAI 兼容 /embeddings 的响应体。
type embeddingResponse struct {
	// Data 是按输入顺序返回的嵌入结果。
	Data []struct {
		// Embedding 是浮点向量。
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	// Error 是服务返回的错误对象。
	Error *struct {
		// Message 是服务错误描述。
		Message string `json:"message"`
	} `json:"error"`
}

// openAIEmbedder 是 OpenAI 兼容端点的 Embedder 实现。
type openAIEmbedder struct {
	// config 是当前嵌入端点配置。
	config EmbeddingConfig
	// client 是带超时的 HTTP 客户端。
	client *http.Client
}

// embeddingHTTPTimeout 是单次嵌入调用的最大耗时。
const embeddingHTTPTimeout = 30 * time.Second

// NewOpenAIEmbedder 创建 OpenAI 兼容端点的 Embedder。
func NewOpenAIEmbedder(config EmbeddingConfig) Embedder {
	return &openAIEmbedder{
		config: config,
		client: &http.Client{Timeout: embeddingHTTPTimeout},
	}
}

// Embed 嵌入单条文本。
func (e *openAIEmbedder) Embed(
	ctx context.Context,
	text string,
	action constants.MemoryEmbeddingAction,
) ([]float32, error) {
	vectors, err := e.EmbedBatch(ctx, []string{text}, action)
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, status_error.MemoryEmbeddingResponseInvalid
	}
	return vectors[0], nil
}

// EmbedBatch 一次请求嵌入多条文本，失败不做逐条降级重试，由上层决定整体失败语义。
func (e *openAIEmbedder) EmbedBatch(
	ctx context.Context,
	texts []string,
	action constants.MemoryEmbeddingAction,
) ([][]float32, error) {
	_ = action
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	body, err := json.Marshal(embeddingRequest{
		Model:          e.config.Model,
		Input:          texts,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, status_error.MemoryEmbeddingRequestFailed
	}
	endpoint := strings.TrimRight(strings.TrimSpace(e.config.BaseURL), "/") + "/embeddings"
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, status_error.MemoryEmbeddingRequestFailed
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	response, err := e.client.Do(request)
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			return nil, status_error.OperationCanceled
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return nil, status_error.OperationTimeout
		default:
			return nil, status_error.MemoryEmbeddingUnavailable
		}
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, status_error.MemoryEmbeddingResponseInvalid
	}
	var decoded embeddingResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, status_error.MemoryEmbeddingResponseInvalid
	}
	if response.StatusCode != http.StatusOK {
		return nil, status_error.MemoryEmbeddingUnavailable
	}
	if len(decoded.Data) != len(texts) {
		return nil, status_error.MemoryEmbeddingResponseInvalid
	}
	vectors := make([][]float32, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		if len(item.Embedding) == 0 {
			return nil, status_error.MemoryEmbeddingResponseInvalid
		}
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
