# Nacos MCP Go

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)

Nacos MCP Go 是一个用于将 Go 应用程序注册为 MCP (Model Context Protocol) 服务到 Nacos 的sdk。它支持自动扫描 Go 结构体方法并基于 `mcp` tag 生成 MCP 工具规范。

[Switch to English Version](README.md)

## 概述

[Nacos](https://nacos.io) 是一个易于使用的平台，专为动态服务发现、配置和服务管理而设计。它帮助您轻松构建云原生应用和微服务架构。

本 MCP(Model Context Protocol) 服务器提供搜索、安装、代理其他 MCP 服务器的工具。

Nacos-MCP-Go 有两种工作模式：

路由模式：默认模式，通过 MCP 服务器推荐、分发、安装和代理其他 MCP 服务器的功能，帮助用户更方便地使用 MCP 服务器服务。

代理模式：通过环境变量 MODE=proxy 指定，可以通过简单配置将 SSE 和 stdio 协议的 MCP 服务器转换为可流式传输的 HTTP 协议 MCP 服务器。

## 特性

- 🚀 **简单易用**: 类似传统微服务注册的 API 设计
- 🏷️ **Tag 驱动**: 基于 `mcp` tag 自动解析工具描述和参数
- 🔧 **灵活注册**: 支持注册单个函数或整个服务结构体
- 🌐 **Nacos 集成**: 完整支持 Nacos MCP 注册表
- ⚙️ **Option 模式**: 优雅的配置选项设计
- 🛡️ **类型安全**: 完整的 Go 类型到 JSON Schema 转换

## 快速开始

### 1. 定义服务

```go
package main

import (
    "time"
    nacosmcp "nacos-mcp-go"
    "nacos-mcp-go/registry"
)

// TimeService 时间服务
type TimeService struct{}

// GetCurrentTime 获取当前时间
func (ts *TimeService) GetCurrentTime(format string) string {
    if format == "" {
        format = "2006-01-02 15:04:05"
    }
    return time.Now().Format(format)
}

// UserService 用户服务
type UserService struct{}

// SearchUser 搜索用户
type SearchUserRequest struct {
    Keyword string `json:"keyword" mcp:"desc=搜索关键词,required"`
    Limit   int    `json:"limit" mcp:"desc=返回结果数量限制"`
}

func (us *UserService) SearchUser(req SearchUserRequest) []string {
    // 业务逻辑
    return []string{"user1", "user2", "user3"}
}
```

### 2. 创建和注册 MCP 服务器

```go
func main() {
    // 创建 MCP 服务器
    server := nacosmcp.NewServer("my-mcp-service",
        nacosmcp.WithNamespace(""),
        nacosmcp.WithGroup("DEFAULT_GROUP"),
        nacosmcp.WithAddress("127.0.0.1", 8080),
        nacosmcp.WithMetadata(map[string]string{
            "version": "1.0.0",
        }),
    )

    // 注册服务
    timeService := &TimeService{}
    userService := &UserService{}

    server.RegisterService(timeService)
    server.RegisterService(userService)

    // 也可以注册单个函数
    server.RegisterTool(func(message string) string {
        return fmt.Sprintf("Echo: %s", message)
    })

    // 注册到 Nacos
    ctx := context.Background()
    serverId, err := registry.Register(ctx, server, "127.0.0.1:8848",
        registry.WithAuth("nacos", "nacos"),
        registry.WithNamespace(""),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("✅ Registered with ID: %s\n", serverId)

    // 优雅关闭时注销
    defer registry.Deregister(ctx, serverId, "127.0.0.1:8848",
        registry.WithAuth("nacos", "nacos"),
        registry.WithNamespace(""),
    )
}
```

## MCP Tag 语法

要声明一个作为 MCP 工具，您需要使用带有以下语法的 `mcp` 标签，以下面的结构体为例
```go
type MyMCPService struct {
    GetTime func() string              `mcp:"tool;name=get_current_time;description=获取服务器当前时间"`
    Search  func(string, int) []string `mcp:"tool;name=search_users;description=搜索用户;paramNames=keyword,limit"`
    Echo    func(string) string        `mcp:"tool;name=echo_message;description=回显消息;paramNames=message"`
}
```

`mcp` 标签支持以下选项：
- `tool`: 表示此字段应被视为 MCP 工具（必需）
- `name=tool_name`: 设置工具名称（可选，默认为字段名称）
- `description=tool description`: 设置工具描述（可选）
- `paramNames=param1,param2`: 设置函数的参数名称（可选）

## 安装

```bash
go get nacos-mcp-go
```

## API 参考

### Server 选项

```go
// WithNamespace 设置命名空间
nacosmcp.WithNamespace("dev")

// WithGroup 设置服务组
nacosmcp.WithGroup("my-group")

// WithAddress 设置服务地址
nacosmcp.WithAddress("127.0.0.1", 8080)

// WithMetadata 设置元数据
nacosmcp.WithMetadata(map[string]string{
    "version": "1.0.0",
    "env": "production",
})
```

### Registry 选项

```go
// WithAuth 设置认证信息
registry.WithAuth("username", "password")

// WithNamespace 设置命名空间
registry.WithNamespace("namespace-id")

// WithTimeout 设置超时时间
registry.WithTimeout(30 * time.Second)
```

## 类型映射

Go 类型自动映射到 JSON Schema：

| Go 类型 | JSON Schema 类型 |
|---------|------------------|
| string | string |
| int, int32, int64 | integer |
| float32, float64 | number |
| bool | boolean |
| []T | array |
| struct | object |

## 环境变量设置

| 参数 | 描述 | 默认值 | 是否必需 | 备注 |
|-----------|-------------------------|---------------|----------|------------------------------------------------------------------------------------------------|
| NACOS_ADDR | Nacos 服务器地址 | 127.0.0.1:8848 | 否 | Nacos 服务器地址，例如：192.168.1.1:8848。注意：需包含端口。 |
| NACOS_USERNAME | Nacos 用户名 | nacos | 否 | Nacos 用户名，例如：nacos。 |
| NACOS_PASSWORD | Nacos 密码 | - | 是 | Nacos 密码，例如：nacos。 |
| NACOS_NAMESPACE | Nacos 命名空间 | public | 否 | Nacos 命名空间，例如：public |
| TRANSPORT_TYPE | 传输协议类型 | stdio | 否 | 传输协议类型。可选项：stdio, sse, streamable_http。 |
| PROXIED_MCP_NAME | 被代理的 MCP 服务器名称 | - | 否 | 在代理模式下，指定要转换的 MCP 服务器名称。必须先在 Nacos 中注册。 |
| MODE | 工作模式 | router | 否 | 可选项：router, proxy。 |
| ACCESS_KEY_ID | 阿里云 RAM 访问密钥 ID | - | 否 | |
| ACCESS_KEY_SECRET | 阿里云 RAM 访问密钥 Secret | - | 否 | |

## 开发

如果您正在进行本地开发，请按照以下步骤操作：

1. 将此仓库克隆到您的本地环境。
2. 修改项目中的代码以实现您想要的功能。
3. 使用 Claude 桌面应用或其他 MCP 兼容应用进行测试。

## 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件。