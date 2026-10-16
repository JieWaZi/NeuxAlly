package constants

//go:generate toolx gen enum RuntimeConnectorTransport

// RuntimeConnectorTransport 表示 Runtime Adapter 可以翻译的 MCP 传输方式。
type RuntimeConnectorTransport string

const (
	// RuntimeConnectorTransportUnspecified 表示未指定 MCP 传输方式。
	RuntimeConnectorTransportUnspecified RuntimeConnectorTransport = ""
	// RuntimeConnectorTransportStdio 表示本地标准输入输出传输。
	RuntimeConnectorTransportStdio RuntimeConnectorTransport = "stdio"
	// RuntimeConnectorTransportStreamableHTTP 表示 Streamable HTTP 传输。
	RuntimeConnectorTransportStreamableHTTP RuntimeConnectorTransport = "streamable_http"
	// RuntimeConnectorTransportSSE 表示旧版 HTTP SSE 传输。
	RuntimeConnectorTransportSSE RuntimeConnectorTransport = "sse"
)
