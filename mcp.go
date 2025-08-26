package nacosmcp

import (
	"context"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
)

// MCPTool 表示一个 MCP 工具的元数据，符合 MCP 规范
type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"` // JSON Schema
}

// Server 是框架的主结构
type Server struct {
	NamingClient naming_client.INamingClient
	ServiceName  string
	GroupName    string
	Ip           string
	Port         uint64
	Metadata     map[string]string
	Tools        []MCPTool // 工具列表
	Ctx          context.Context
	Cancel       context.CancelFunc
}
