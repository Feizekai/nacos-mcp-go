package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	nacosmcp "nacos-mcp-go"
)

// NacosMcpApiClient Nacos MCP API 客户端
type NacosMcpApiClient struct {
	ServerAddr  string
	Username    string
	Password    string
	NamespaceId string
	httpClient  *http.Client
	accessToken string
}

// McpServerBasicInfo MCP服务器基本信息
type McpServerBasicInfo struct {
	ID                 string               `json:"id,omitempty"`
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	Repository         string               `json:"repository,omitempty"`
	Protocol           string               `json:"protocol"`
	FrontProtocol      string               `json:"frontProtocol,omitempty"`
	Version            string               `json:"version"`
	VersionDetail      *ServerVersionDetail `json:"versionDetail"`
	Capabilities       []string             `json:"capabilities,omitempty"`
	RemoteServerConfig *RemoteServerConfig  `json:"remoteServerConfig,omitempty"`
}

// ServerVersionDetail 服务器版本详情
type ServerVersionDetail struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date,omitempty"`
	IsLatest    bool   `json:"is_latest,omitempty"`
}

// RemoteServerConfig 远程服务器配置
type RemoteServerConfig struct {
	ExportPath              string                 `json:"exportPath"`
	ServiceRef              *McpServiceRef         `json:"serviceRef,omitempty"`
	FrontEndpointConfigList []*FrontEndpointConfig `json:"frontEndpointConfigList,omitempty"`
}

// McpServiceRef MCP服务引用
type McpServiceRef struct {
	NamespaceId       string `json:"namespaceId"`
	GroupName         string `json:"groupName"`
	ServiceName       string `json:"serviceName"`
	TransportProtocol string `json:"transportProtocol"`
}

// FrontEndpointConfig 前端端点配置
type FrontEndpointConfig struct {
	EndpointType string          `json:"endpointType"`
	EndpointData interface{}     `json:"endpointData"`
	Path         string          `json:"path"`
	Protocol     string          `json:"protocol"`
	Headers      []KeyValueInput `json:"headers,omitempty"`
}

// KeyValueInput 键值对输入
type KeyValueInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// McpToolSpecification MCP工具规范
type McpToolSpecification struct {
	Tools           []McpTool        `json:"tools"`
	SecuritySchemes []SecurityScheme `json:"securitySchemes,omitempty"`
}

// McpTool MCP工具
type McpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// SecurityScheme 安全方案
type SecurityScheme struct {
	Type   string `json:"type"`
	Scheme string `json:"scheme,omitempty"`
}

// McpEndpointSpec MCP端点规范
type McpEndpointSpec struct {
	Data map[string]string `json:"data"`
}

// ClientOption 客户端配置选项
type ClientOption func(*NacosMcpApiClient)

// WithAuth 设置认证信息
func WithAuth(username, password string) ClientOption {
	return func(c *NacosMcpApiClient) {
		c.Username = username
		c.Password = password
	}
}

// WithNamespace 设置命名空间
func WithNamespace(namespaceId string) ClientOption {
	return func(c *NacosMcpApiClient) {
		c.NamespaceId = namespaceId
	}
}

// WithTimeout 设置HTTP超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *NacosMcpApiClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewNacosMcpApiClient 创建新的Nacos MCP API 客户端
func NewNacosMcpApiClient(serverAddr string, options ...ClientOption) *NacosMcpApiClient {
	// 确保ServerAddr包含协议前缀
	if !strings.HasPrefix(serverAddr, "http://") && !strings.HasPrefix(serverAddr, "https://") {
		serverAddr = "http://" + serverAddr
	}

	client := &NacosMcpApiClient{
		ServerAddr:  serverAddr,
		NamespaceId: "", // 默认为public命名空间
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 应用选项
	for _, option := range options {
		option(client)
	}

	return client
}

// login 登录获取访问令牌（仅在需要认证时使用）
func (c *NacosMcpApiClient) login() error {
	if c.Username == "" || c.Password == "" {
		return nil // 无需认证
	}

	loginURL := fmt.Sprintf("%s/nacos/v1/auth/login", c.ServerAddr)

	data := url.Values{}
	data.Set("username", c.Username)
	data.Set("password", c.Password)

	resp, err := c.httpClient.PostForm(loginURL, data)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read login response failed: %w", err)
	}

	var loginResp struct {
		AccessToken string `json:"accessToken"`
	}

	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("parse login response failed: %w", err)
	}

	c.accessToken = loginResp.AccessToken
	return nil
}

// RegisterMcpServer 注册MCP服务器到Nacos
func (c *NacosMcpApiClient) RegisterMcpServer(s *nacosmcp.Server) (string, error) {
	// 如果配置了认证信息，则先登录
	if c.Username != "" && c.Password != "" && c.accessToken == "" {
		if err := c.login(); err != nil {
			return "", fmt.Errorf("login failed: %w", err)
		}
	}

	// 构建MCP服务器基本信息
	basicInfo := map[string]interface{}{
		"protocol":      "sse",
		"frontProtocol": "http",
		"name":          s.ServiceName,
		"id":            "",
		"description":   fmt.Sprintf("MCP Server registered via nacos-mcp-go framework. Tools: %d", len(s.Tools)),
		"versionDetail": map[string]interface{}{
			"version": "1.0.0",
		},
		"enabled": true,
		"remoteServerConfig": map[string]interface{}{
			"exportPath": "/mcp",
			"serviceRef": map[string]interface{}{
				"namespaceId":       c.NamespaceId,
				"groupName":         "DEFAULT_GROUP",
				"serviceName":       s.ServiceName,
				"transportProtocol": "http",
			},
		},
	}

	// 构建工具规范
	var mcpTools []map[string]interface{}
	toolsMeta := make(map[string]interface{})

	for _, tool := range s.Tools {
		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})

		// 为每个工具添加元数据
		toolsMeta[tool.Name] = map[string]interface{}{
			"invokeContext": map[string]interface{}{
				"path":   "/mcp",
				"method": "POST",
			},
			"enabled": true,
			"templates": map[string]interface{}{
				"json-template": map[string]interface{}{
					"templateType": "string",
					"requestTemplate": map[string]interface{}{
						"url":            "",
						"method":         "POST",
						"headers":        []interface{}{},
						"argsToJsonBody": true,
						"argsToUrlParam": false,
						"argsToFormBody": false,
						"body":           "string",
					},
					"responseTemplate": map[string]interface{}{
						"body": "string",
					},
				},
			},
		}
	}

	toolSpec := map[string]interface{}{
		"tools":     mcpTools,
		"toolsMeta": toolsMeta,
	}

	// 构建端点规范
	endpointSpec := map[string]interface{}{
		"type": "DIRECT",
		"data": map[string]interface{}{
			"address": s.Ip,
			"port":    fmt.Sprintf("%d", s.Port),
		},
	}

	// 将规范转换为JSON字符串
	serverSpecJSON, err := json.Marshal(basicInfo)
	if err != nil {
		return "", fmt.Errorf("marshal server specification failed: %w", err)
	}

	toolSpecJSON, err := json.Marshal(toolSpec)
	if err != nil {
		return "", fmt.Errorf("marshal tool specification failed: %w", err)
	}

	endpointSpecJSON, err := json.Marshal(endpointSpec)
	if err != nil {
		return "", fmt.Errorf("marshal endpoint specification failed: %w", err)
	}

	// 构建表单数据
	formData := url.Values{}
	formData.Set("namespaceId", c.NamespaceId)
	formData.Set("serverSpecification", string(serverSpecJSON))
	formData.Set("toolSpecification", string(toolSpecJSON))
	formData.Set("endpointSpecification", string(endpointSpecJSON))

	// 发送创建MCP服务器请求
	createURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp", c.ServerAddr)
	req, err := http.NewRequest("POST", createURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create mcp server request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read create response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create mcp server failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}

	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("parse create response failed: %w", err)
	}

	if createResp.Code != 200 {
		return "", fmt.Errorf("create mcp server failed: %s", createResp.Message)
	}

	fmt.Printf("✅ Successfully registered MCP Server '%s' to Nacos MCP Registry\n", s.ServiceName)
	fmt.Printf("📋 MCP Server ID: %s\n", createResp.Data)
	fmt.Printf("🔧 Tools Count: %d\n", len(s.Tools))
	fmt.Printf("🌐 Protocol: streamable-http\n")

	return createResp.Data, nil
}

// DeregisterMcpServer 从Nacos注销MCP服务器
func (c *NacosMcpApiClient) DeregisterMcpServer(serverId string) error {
	// 构建删除请求URL
	baseURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp", c.ServerAddr)
	params := url.Values{}
	params.Set("namespaceId", c.NamespaceId)
	params.Set("mcpId", serverId)

	deleteURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create delete request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete mcp server request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read delete response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete mcp server failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("✅ Successfully deregistered MCP Server '%s' from Nacos MCP Registry\n", serverId)
	return nil
}

// ListMcpServers 列出MCP服务器
func (c *NacosMcpApiClient) ListMcpServers(search string, pageNo, pageSize int) (interface{}, error) {
	// 构建查询参数
	params := url.Values{}
	params.Set("namespaceId", c.NamespaceId)
	if search != "" {
		params.Set("mcpName", search)
		params.Set("search", "blur")
	}
	params.Set("pageNo", strconv.Itoa(pageNo))
	params.Set("pageSize", strconv.Itoa(pageSize))

	listURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp/list?%s", c.ServerAddr, params.Encode())

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create list request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list mcp servers request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list mcp servers failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var listResp interface{}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("parse list response failed: %w", err)
	}

	return listResp, nil
}

// RegisterToNacosMcpRegistry 将 MCP Server 注册到 Nacos MCP 注册表
// 这个函数直接调用 Nacos 的 MCP HTTP API，不依赖 nacos-go-sdk
func RegisterToNacosMcpRegistry(s *nacosmcp.Server, serverAddr string, options ...ClientOption) (string, error) {
	client := NewNacosMcpApiClient(serverAddr, options...)
	return client.RegisterMcpServer(s)
}

// DeregisterFromNacosMcpRegistry 从 Nacos MCP 注册表注销服务
func DeregisterFromNacosMcpRegistry(serverId, serverAddr string, options ...ClientOption) error {
	client := NewNacosMcpApiClient(serverAddr, options...)
	return client.DeregisterMcpServer(serverId)
}

// ListNacosMcpServers 列出 Nacos MCP 注册表中的服务
func ListNacosMcpServers(serverAddr string, search string, pageNo, pageSize int, options ...ClientOption) (interface{}, error) {
	client := NewNacosMcpApiClient(serverAddr, options...)
	return client.ListMcpServers(search, pageNo, pageSize)
}
