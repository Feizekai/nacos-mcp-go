package registry

import (
	"context"
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

// Client Nacos MCP注册客户端
type Client struct {
	serverAddr  string
	username    string
	password    string
	namespaceId string
	timeout     time.Duration
	httpClient  *http.Client
	accessToken string
}

// Option 客户端配置选项
type Option func(*Client)

// WithAuth 设置认证信息
func WithAuth(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithNamespace 设置命名空间
func WithNamespace(namespaceId string) Option {
	return func(c *Client) {
		c.namespaceId = namespaceId
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
		c.httpClient.Timeout = timeout
	}
}

// NewClient 创建注册客户端
func NewClient(serverAddr string, opts ...Option) *Client {
	if !strings.HasPrefix(serverAddr, "http://") && !strings.HasPrefix(serverAddr, "https://") {
		serverAddr = "http://" + serverAddr
	}

	client := &Client{
		serverAddr:  serverAddr,
		namespaceId: "",
		timeout:     30 * time.Second,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Register 注册MCP服务器到Nacos
func (c *Client) Register(ctx context.Context, server *nacosmcp.Server) (string, error) {
	if err := c.ensureAuth(); err != nil {
		return "", fmt.Errorf("authentication failed: %w", err)
	}

	// 构建注册请求
	req, err := c.buildRegisterRequest(server)
	if err != nil {
		return "", fmt.Errorf("build register request failed: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	return c.parseRegisterResponse(resp)
}

// Deregister 注销MCP服务器
func (c *Client) Deregister(ctx context.Context, serverId string) error {
	if err := c.ensureAuth(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// 构建注销请求
	req, err := c.buildDeregisterRequest(serverId)
	if err != nil {
		return fmt.Errorf("build deregister request failed: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deregister request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应
	return c.checkResponse(resp)
}

// List 列出MCP服务器
func (c *Client) List(ctx context.Context, search string, pageNo, pageSize int) (interface{}, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 构建列表请求
	req, err := c.buildListRequest(search, pageNo, pageSize)
	if err != nil {
		return nil, fmt.Errorf("build list request failed: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	return c.parseListResponse(resp)
}

// ensureAuth 确保认证
func (c *Client) ensureAuth() error {
	if c.username == "" || c.password == "" {
		return nil // 无需认证
	}

	if c.accessToken != "" {
		return nil // 已认证
	}

	return c.login()
}

// login 登录获取访问令牌
func (c *Client) login() error {
	loginURL := fmt.Sprintf("%s/nacos/v1/auth/login", c.serverAddr)

	data := url.Values{}
	data.Set("username", c.username)
	data.Set("password", c.password)

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

// buildRegisterRequest 构建注册请求
func (c *Client) buildRegisterRequest(server *nacosmcp.Server) (*http.Request, error) {
	// 构建服务器规范
	ip, port := server.GetAddress()
	protocol := string(server.GetProtocol())

	serverSpec := map[string]interface{}{
		"protocol":      protocol,
		"frontProtocol": c.getFrontProtocol(server.GetProtocol()),
		"name":          server.GetName(),
		"id":            "",
		"description":   fmt.Sprintf("MCP Server: %s", server.GetName()),
		"versionDetail": map[string]interface{}{
			"version": "1.0.0",
		},
		"enabled": true,
	}

	// 根据协议类型添加不同的配置
	if server.GetProtocol() == nacosmcp.ProtocolStdio {
		// stdio协议使用本地配置
		serverSpec["localServerConfig"] = map[string]interface{}{}
	} else {
		// sse和streamable-http协议使用远程配置
		serverSpec["remoteServerConfig"] = map[string]interface{}{
			"exportPath": "/mcp",
			"serviceRef": map[string]interface{}{
				"namespaceId":       c.namespaceId,
				"groupName":         server.GetGroup(),
				"serviceName":       server.GetName(),
				"transportProtocol": c.getTransportProtocol(server.GetProtocol()),
			},
		}
	}

	// 构建工具规范
	tools := server.GetTools()
	mcpTools := make([]map[string]interface{}, len(tools))
	toolsMeta := make(map[string]interface{})

	for i, tool := range tools {
		mcpTools[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}

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

	// 构建端点规范（仅对非stdio协议）
	var endpointSpec map[string]interface{}
	if server.GetProtocol() != nacosmcp.ProtocolStdio {
		endpointSpec = map[string]interface{}{
			"type": "DIRECT",
			"data": map[string]interface{}{
				"address": ip,
				"port":    fmt.Sprintf("%d", port),
			},
		}
	}

	// 序列化为JSON字符串
	serverSpecJSON, _ := json.Marshal(serverSpec)
	toolSpecJSON, _ := json.Marshal(toolSpec)

	// 构建表单数据
	formData := url.Values{}
	formData.Set("namespaceId", c.namespaceId)
	formData.Set("serverSpecification", string(serverSpecJSON))
	formData.Set("toolSpecification", string(toolSpecJSON))

	// 只有非stdio协议才需要端点规范
	if endpointSpec != nil {
		endpointSpecJSON, _ := json.Marshal(endpointSpec)
		formData.Set("endpointSpecification", string(endpointSpecJSON))
	}

	// 创建请求
	createURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp", c.serverAddr)
	req, err := http.NewRequest("POST", createURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return req, nil
}

// buildDeregisterRequest 构建注销请求
func (c *Client) buildDeregisterRequest(serverId string) (*http.Request, error) {
	baseURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp", c.serverAddr)
	params := url.Values{}
	params.Set("namespaceId", c.namespaceId)
	params.Set("mcpId", serverId)

	deleteURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return nil, err
	}

	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return req, nil
}

// buildListRequest 构建列表请求
func (c *Client) buildListRequest(search string, pageNo, pageSize int) (*http.Request, error) {
	params := url.Values{}
	params.Set("namespaceId", c.namespaceId)
	if search != "" {
		params.Set("mcpName", search)
		params.Set("search", "blur")
	}
	params.Set("pageNo", strconv.Itoa(pageNo))
	params.Set("pageSize", strconv.Itoa(pageSize))

	listURL := fmt.Sprintf("%s/nacos/v3/admin/ai/mcp/list?%s", c.serverAddr, params.Encode())

	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, err
	}

	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return req, nil
}

// parseRegisterResponse 解析注册响应
func (c *Client) parseRegisterResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("register failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response failed: %w", err)
	}

	if result.Code != 200 {
		return "", fmt.Errorf("register failed: %s", result.Message)
	}

	return result.Data, nil
}

// parseListResponse 解析列表响应
func (c *Client) parseListResponse(resp *http.Response) (interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return result, nil
}

// checkResponse 检查响应
func (c *Client) checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Register 注册MCP服务器到Nacos
func Register(ctx context.Context, server *nacosmcp.Server, serverAddr string, opts ...Option) (string, error) {
	client := NewClient(serverAddr, opts...)
	return client.Register(ctx, server)
}

// Deregister 从Nacos注销MCP服务器
func Deregister(ctx context.Context, serverId, serverAddr string, opts ...Option) error {
	client := NewClient(serverAddr, opts...)
	return client.Deregister(ctx, serverId)
}

// List 列出Nacos中的MCP服务器
func List(ctx context.Context, serverAddr string, search string, pageNo, pageSize int, opts ...Option) (interface{}, error) {
	client := NewClient(serverAddr, opts...)
	return client.List(ctx, search, pageNo, pageSize)
}

// getFrontProtocol 获取前端协议
func (c *Client) getFrontProtocol(protocol nacosmcp.Protocol) string {
	switch protocol {
	case nacosmcp.ProtocolStdio:
		return "stdio"
	case nacosmcp.ProtocolSSE:
		return "http"
	case nacosmcp.ProtocolStreamHTTP:
		return "http"
	default:
		return "http"
	}
}

// getTransportProtocol 获取传输协议
func (c *Client) getTransportProtocol(protocol nacosmcp.Protocol) string {
	switch protocol {
	case nacosmcp.ProtocolStdio:
		return "stdio"
	case nacosmcp.ProtocolSSE:
		return "http"
	case nacosmcp.ProtocolStreamHTTP:
		return "http"
	default:
		return "http"
	}
}
