package types

// Protocol MCP协议类型
type Protocol string

const (
	ProtocolStdio      Protocol = "stdio"           // 标准输入输出
	ProtocolSSE        Protocol = "sse"             // Server-Sent Events
	ProtocolStreamHTTP Protocol = "streamable-http" // 流式HTTP
)

// Tool MCP工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     interface{}            `json:"-"`
}

// ServerInterface MCP服务器接口
type ServerInterface interface {
	GetName() string
	GetNamespace() string
	GetGroup() string
	GetAddress() (string, int)
	GetProtocol() Protocol
	GetTools() []Tool
	GetMetadata() map[string]string
	IsRunning() bool
}
