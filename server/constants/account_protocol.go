package constants

//go:generate toolx gen enum AccountProtocol

// AccountProtocol 表示账号服务原生支持的模型接口协议。
type AccountProtocol string

const (
	// AccountProtocolUnspecified 表示未选择协议。
	AccountProtocolUnspecified AccountProtocol = ""
	// AccountProtocolOpenAIResponses 对应 OpenAI Responses API。
	AccountProtocolOpenAIResponses AccountProtocol = "openai-responses"
	// AccountProtocolOpenAICompletions 对应 OpenAI Chat Completions API。
	AccountProtocolOpenAICompletions AccountProtocol = "openai-completions"
	// AccountProtocolAnthropicMessages 对应 Anthropic Messages API。
	AccountProtocolAnthropicMessages AccountProtocol = "anthropic-messages"
	// AccountProtocolGoogleGenerativeAI 对应 Google GenerateContent API。
	AccountProtocolGoogleGenerativeAI AccountProtocol = "google-generative-ai"
)
