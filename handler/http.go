package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	nacosmcp "nacos-mcp-go"
)

// HTTPHandler 封装 MCP HTTP 接口
type HTTPHandler struct {
	server *nacosmcp.Server
	mu     sync.RWMutex
}

// NewHTTPHandler 创建新的处理器
func NewHTTPHandler(server *nacosmcp.Server) *HTTPHandler {
	return &HTTPHandler{
		server: server,
	}
}

// RegisterRoutes 注册 MCP 路由到 http.ServeMux
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/mcp/tools", h.listTools)
	mux.HandleFunc("/mcp/tools/", h.invokeTool)
}

// listTools 处理 /mcp/tools
func (h *HTTPHandler) listTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	tools := h.server.Tools
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		log.Printf("Error encoding tools list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// invokeTool 处理 /mcp/tools/{name}/invoke
func (h *HTTPHandler) invokeTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析工具名
	path := strings.TrimPrefix(r.URL.Path, "/mcp/tools/")
	parts := strings.Split(path, "/invoke")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	toolName := parts[0]

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 查找工具
	h.mu.RLock()
	var tool *nacosmcp.MCPTool
	for _, t := range h.server.Tools {
		if t.Name == toolName {
			tool = &t
			break
		}
	}
	h.mu.RUnlock()

	if tool == nil {
		http.Error(w, "Tool not found", http.StatusNotFound)
		return
	}

	// 返回结果 - 简化实现，实际应该执行工具函数
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	result := map[string]interface{}{
		"result":    "Tool invoked: " + toolName,
		"arguments": req.Arguments,
	}
	json.NewEncoder(w).Encode(result)
}
