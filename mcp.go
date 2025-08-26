package nacosmcp

import (
	"context"
	"fmt"
	"net/http"

	"nacos-mcp-go/handler"
	"nacos-mcp-go/httpclient"
	"nacos-mcp-go/scanner"
	"nacos-mcp-go/types"
)

type Protocol = types.Protocol
type Tool = types.Tool

const (
	ProtocolStdio      = types.ProtocolStdio
	ProtocolSSE        = types.ProtocolSSE
	ProtocolStreamHTTP = types.ProtocolStreamHTTP
)

// Server MCP服务器实例
type Server struct {
	name       string
	namespace  string
	group      string
	ip         string
	port       int
	protocol   Protocol
	tools      []Tool
	metadata   map[string]string
	httpServer *httpclient.Server
	running    bool
}

type Option func(*Server)

// WithNamespace 设置命名空间
func WithNamespace(namespace string) Option {
	return func(s *Server) {
		s.namespace = namespace
	}
}

// WithGroup 设置服务组
func WithGroup(group string) Option {
	return func(s *Server) {
		s.group = group
	}
}

// WithAddress 设置服务地址
func WithAddress(ip string, port int) Option {
	return func(s *Server) {
		s.ip = ip
		s.port = port
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]string) Option {
	return func(s *Server) {
		if s.metadata == nil {
			s.metadata = make(map[string]string)
		}
		for k, v := range metadata {
			s.metadata[k] = v
		}
	}
}

// WithProtocol 设置MCP协议类型
func WithProtocol(protocol Protocol) Option {
	return func(s *Server) {
		s.protocol = protocol
	}
}

// NewServer 创建MCP服务器
func NewServer(name string, opts ...Option) *Server {
	server := &Server{
		name:     name,
		group:    "DEFAULT_GROUP",
		ip:       "127.0.0.1",
		port:     8080,
		protocol: ProtocolSSE, // 默认使用SSE协议
		metadata: make(map[string]string),
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// RegisterTool 注册单个工具函数
func (s *Server) RegisterTool(handler interface{}) error {
	toolInfo, err := s.scanTool(handler)
	if err != nil {
		return fmt.Errorf("scan tool failed: %w", err)
	}

	tool := Tool{
		Name:        toolInfo.Name,
		Description: toolInfo.Description,
		InputSchema: toolInfo.InputSchema,
		Handler:     toolInfo.Handler,
	}

	s.tools = append(s.tools, tool)
	return nil
}

// RegisterService 注册服务对象的所有导出方法为工具
func (s *Server) RegisterService(service interface{}) error {
	toolInfos, err := s.scanStruct(service)
	if err != nil {
		return fmt.Errorf("scan service failed: %w", err)
	}

	for _, toolInfo := range toolInfos {
		tool := Tool{
			Name:        toolInfo.Name,
			Description: toolInfo.Description,
			InputSchema: toolInfo.InputSchema,
			Handler:     toolInfo.Handler,
		}
		s.tools = append(s.tools, tool)
	}

	return nil
}

// Start 启动服务器
func (s *Server) Start(ctx context.Context) error {
	if s.running {
		return fmt.Errorf("server is already running")
	}

	// 只有非stdio协议才需要启动HTTP服务器
	if s.protocol != ProtocolStdio {
		// 创建HTTP处理器
		httpHandler := handler.NewHTTPHandler(s)
		mux := http.NewServeMux()
		httpHandler.RegisterRoutes(mux)

		// 创建HTTP服务器
		addr := fmt.Sprintf("%s:%d", s.ip, s.port)
		s.httpServer = httpclient.NewServer(addr, mux)

		// 在goroutine中启动HTTP服务器
		go func() {
			if err := s.httpServer.Start(); err != nil {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	}

	s.running = true
	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if !s.running {
		return nil
	}

	// 停止HTTP服务器
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	s.running = false
	return nil
}

// GetName 获取服务名称
func (s *Server) GetName() string {
	return s.name
}

// GetNamespace 获取命名空间
func (s *Server) GetNamespace() string {
	return s.namespace
}

// GetGroup 获取服务组
func (s *Server) GetGroup() string {
	return s.group
}

// GetAddress 获取服务地址
func (s *Server) GetAddress() (string, int) {
	return s.ip, s.port
}

// GetTools 获取工具列表
func (s *Server) GetTools() []Tool {
	return s.tools
}

// GetMetadata 获取元数据
func (s *Server) GetMetadata() map[string]string {
	return s.metadata
}

// GetProtocol 获取协议类型
func (s *Server) GetProtocol() Protocol {
	return s.protocol
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	return s.running
}

// scanTool 扫描单个工具函数
func (s *Server) scanTool(handler interface{}) (*scanner.ToolInfo, error) {
	return scanner.ScanTool(handler)
}

// scanStruct 扫描结构体方法
func (s *Server) scanStruct(service interface{}) ([]*scanner.ToolInfo, error) {
	return scanner.ScanStruct(service)
}
