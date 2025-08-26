package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nacosmcp "nacos-mcp-go"
	"nacos-mcp-go/handler"
	"nacos-mcp-go/httpclient"
	"nacos-mcp-go/registry"
)

func main() {
	// Nacos MCP API 配置
	nacosServerAddr := "http://192.144.175.104:8848"
	nacosUsername := "nacos"
	nacosPassword := "37768848f!"
	namespaceId := "" // 空字符串表示public命名空间

	// 创建 MCP 服务器
	server := &nacosmcp.Server{
		ServiceName: "mcp-time-server",
		GroupName:   "DEFAULT_GROUP",
		Ip:          "127.0.0.1",
		Port:        8082,
		Metadata:    make(map[string]string),
	}

	// 添加工具 - 先定义工具，然后赋值
	formatProp := map[string]interface{}{
		"type":        "string",
		"description": "时间格式，如 '2006-01-02 15:04:05'",
		"default":     "2006-01-02 15:04:05",
	}

	timeToolProps := map[string]interface{}{
		"format": formatProp,
	}

	timeToolSchema := map[string]interface{}{
		"type":       "object",
		"properties": timeToolProps,
	}

	keywordProp := map[string]interface{}{
		"type":        "string",
		"description": "搜索关键词",
	}

	limitProp := map[string]interface{}{
		"type":        "integer",
		"description": "返回结果数量限制",
		"default":     10,
	}

	searchToolProps := map[string]interface{}{
		"keyword": keywordProp,
		"limit":   limitProp,
	}

	searchToolSchema := map[string]interface{}{
		"type":       "object",
		"properties": searchToolProps,
		"required":   []string{"keyword"},
	}

	server.Tools = []nacosmcp.MCPTool{
		{
			Name:        "get_current_time",
			Description: "获取当前时间",
			InputSchema: timeToolSchema,
		},
		{
			Name:        "search_users",
			Description: "搜索用户信息",
			InputSchema: searchToolSchema,
		},
	}

	// 创建 HTTP 处理器
	httpHandler := handler.NewHTTPHandler(server)

	// 创建并配置 HTTP 服务器
	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	httpServer := httpclient.NewServer(":8082", mux)

	// 启动 HTTP 服务器
	go func() {
		log.Println("🚀 Starting MCP Server on :8082")
		if err := httpServer.Start(); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(2 * time.Second)

	// 注册到 Nacos MCP 注册表（专门的MCP服务管理）
	fmt.Println("🔄 Registering to Nacos MCP Registry...")
	var mcpServerId string
	if serverId, err := registry.RegisterToNacosMcpRegistry(
		server,
		nacosServerAddr,
		registry.WithAuth(nacosUsername, nacosPassword),
		registry.WithNamespace(namespaceId),
	); err != nil {
		log.Printf("Failed to register to Nacos MCP Registry: %v", err)
	} else {
		mcpServerId = serverId
		fmt.Println("🎉 Successfully registered to Nacos MCP Registry!")
		fmt.Println("📋 You can now see this MCP server in Nacos Console -> AI -> MCP Management")
	}

	// 演示查询 MCP 服务列表
	fmt.Println("\n🔍 Listing MCP servers from Nacos MCP Registry...")
	if servers, err := registry.ListNacosMcpServers(
		nacosServerAddr,
		"", // 搜索关键词，空表示查询所有
		1,  // 页码
		10, // 页大小
		registry.WithAuth(nacosUsername, nacosPassword),
		registry.WithNamespace(namespaceId),
	); err != nil {
		log.Printf("Failed to list MCP servers: %v", err)
	} else {
		fmt.Printf("📋 MCP Servers: %+v\n", servers)
	}

	// 等待中断信号
	fmt.Println("\n🚀 MCP Server is running...")
	fmt.Println("📡 Service Discovery: http://127.0.0.1:8848/nacos (check Service Management)")
	fmt.Println("🤖 MCP Registry: http://127.0.0.1:8848/nacos (check AI -> MCP Management)")
	fmt.Println("🔗 MCP Endpoint: http://127.0.0.1:8082/mcp")
	fmt.Println("⏹️  Press Ctrl+C to stop...")

	// 优雅关闭
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\n🛑 Shutting down...")

	// 从 Nacos MCP 注册表注销
	if mcpServerId != "" {
		if err := registry.DeregisterFromNacosMcpRegistry(
			mcpServerId,
			nacosServerAddr,
			registry.WithAuth(nacosUsername, nacosPassword),
			registry.WithNamespace(namespaceId),
		); err != nil {
			log.Printf("Failed to deregister from Nacos MCP Registry: %v", err)
		}
	}

	// 停止 HTTP 服务器
	if err := httpServer.Shutdown(nil); err != nil {
		log.Printf("Failed to shutdown HTTP server: %v", err)
	}

	fmt.Println("✅ Server stopped gracefully")
}
