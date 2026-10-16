package constants

//go:generate toolx gen enum MCPTransport

// MCPTransport 表示 MCP 连接器使用的传输方式。
type MCPTransport string

const (
	// MCPTransportUnspecified 表示数据库或框架产生的空传输方式。
	MCPTransportUnspecified MCPTransport = ""
	// MCPTransportStdio 表示由本地子进程标准输入输出承载 MCP。
	MCPTransportStdio MCPTransport = "stdio"
	// MCPTransportStreamableHTTP 表示使用 Streamable HTTP 连接远程 MCP。
	MCPTransportStreamableHTTP MCPTransport = "streamable_http"
	// MCPTransportSSE 表示使用旧版 HTTP SSE 连接远程 MCP。
	MCPTransportSSE MCPTransport = "sse"
)
