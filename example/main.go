package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	nacosmcp "nacos-mcp-go"
	"nacos-mcp-go/registry"
)

// MyMCPService 使用函数字段定义MCP工具的服务
type MyMCPService struct {
	GetTime func() string              `mcp:"tool;name=get_current_time;description=获取服务器当前时间"`
	Search  func(string, int) []string `mcp:"tool;name=search_users;description=搜索用户;paramNames=keyword,limit"`
	Echo    func(string) string        `mcp:"tool;name=echo_message;description=回显消息;paramNames=message"`
}

// TimeService 传统方法定义的服务（向后兼容）
type TimeService struct{}

// GetCurrentTime 获取当前时间
func (ts *TimeService) GetCurrentTime(format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return time.Now().Format(format)
}

// GetTimestamp 获取时间戳
func (ts *TimeService) GetTimestamp() int64 {
	return time.Now().Unix()
}

func main() {
	// Nacos配置
	nacosServerAddr := "192.144.175.104:8848"
	nacosUsername := "nacos"
	nacosPassword := "37768848f!"

	// 创建MCP服务器，支持不同协议
	server := nacosmcp.NewServer("advanced-mcp-service",
		nacosmcp.WithNamespace(""),
		nacosmcp.WithGroup("DEFAULT_GROUP"),
		nacosmcp.WithAddress("127.0.0.1", 8082),
		nacosmcp.WithProtocol(nacosmcp.ProtocolSSE), // 可选: ProtocolStdio, ProtocolSSE, ProtocolStreamHTTP
		nacosmcp.WithMetadata(map[string]string{
			"version": "2.0.0",
			"author":  "nacos-mcp-go",
			"type":    "advanced",
		}),
	)

	// 创建函数字段服务实例并实现函数
	mcpService := &MyMCPService{
		GetTime: func() string {
			return time.Now().Format("2006-01-02 15:04:05")
		},
		Search: func(keyword string, limit int) []string {
			// 模拟搜索逻辑
			users := []string{"alice", "bob", "charlie", "david", "eve"}
			var result []string
			for _, user := range users {
				if keyword == "" || user == keyword {
					result = append(result, user)
				}
			}
			if limit > 0 && limit < len(result) {
				result = result[:limit]
			}
			return result
		},
		Echo: func(message string) string {
			return fmt.Sprintf("Echo: %s", message)
		},
	}

	// 注册函数字段服务
	if err := server.RegisterService(mcpService); err != nil {
		log.Fatalf("Failed to register MCP service: %v", err)
	}

	// 注册单个函数
	server.RegisterTool(func(count int) []int {
		result := make([]int, count)
		for i := 0; i < count; i++ {
			result[i] = i + 1
		}
		return result
	})

	fmt.Printf("🚀 MCP Server '%s' initialized\n", server.GetName())
	fmt.Printf("📋 Protocol: %s\n", server.GetProtocol())
	fmt.Printf("🔧 Registered %d tools\n", len(server.GetTools()))

	// 打印工具信息
	fmt.Println("\n📋 Registered Tools:")
	for i, tool := range server.GetTools() {
		fmt.Printf("  %d. %s - %s\n", i+1, tool.Name, tool.Description)
	}

	// 启动服务器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动MCP服务器
	fmt.Println("\n🚀 Starting MCP Server...")
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}

	// 注册到Nacos
	fmt.Println("\n🔄 Registering to Nacos MCP Registry...")
	serverId, err := registry.Register(ctx, server, nacosServerAddr,
		registry.WithAuth(nacosUsername, nacosPassword),
		registry.WithNamespace(""),
	)
	if err != nil {
		log.Printf("Failed to register to Nacos: %v", err)
	} else {
		fmt.Printf("✅ Successfully registered to Nacos, Server ID: %s\n", serverId)
	}

	// 查询MCP服务列表
	fmt.Println("\n🔍 Listing MCP servers...")
	if servers, err := registry.List(ctx, nacosServerAddr, "", 1, 10,
		registry.WithAuth(nacosUsername, nacosPassword),
		registry.WithNamespace(""),
	); err != nil {
		log.Printf("Failed to list MCP servers: %v", err)
	} else {
		fmt.Printf("📋 MCP Servers: %+v\n", servers)
	}

	// 等待中断信号
	fmt.Println("\n🚀 MCP Server is running...")
	fmt.Println("📡 Nacos Console: http://192.144.175.104:8848/nacos")
	fmt.Println("🤖 MCP Management: AI -> MCP Management")
	fmt.Printf("🔗 Protocol: %s\n", server.GetProtocol())
	if server.GetProtocol() != nacosmcp.ProtocolStdio {
		ip, port := server.GetAddress()
		fmt.Printf("🌐 Endpoint: %s:%d/mcp\n", ip, port)
	}
	fmt.Println("⏹️  Press Ctrl+C to stop...")

	// 优雅关闭
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\n🛑 Shutting down...")

	// 注销服务
	if serverId != "" {
		if err := registry.Deregister(ctx, serverId, nacosServerAddr,
			registry.WithAuth(nacosUsername, nacosPassword),
			registry.WithNamespace(""),
		); err != nil {
			log.Printf("Failed to deregister: %v", err)
		} else {
			fmt.Println("✅ Successfully deregistered from Nacos")
		}
	}

	// 停止MCP服务器
	if err := server.Stop(ctx); err != nil {
		log.Printf("Failed to stop MCP server: %v", err)
	} else {
		fmt.Println("✅ MCP Server stopped")
	}

	fmt.Println("✅ Server stopped gracefully")
}
