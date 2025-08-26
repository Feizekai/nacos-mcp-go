# Nacos MCP Go

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)

Nacos MCP Go is a sdk for registering Go applications as MCP (Model Context Protocol) services to Nacos. It supports automatic scanning of Go struct methods and generating MCP tool specifications based on `mcp` tags.

[切换到中文版](README_zh.md)

## Overview

[Nacos](https://nacos.io) is an easy-to-use platform designed for dynamic service discovery and configuration and service management. It helps you to build cloud native applications and microservices platform easily.

This MCP(Model Context Protocol) Server provides tools to search, install, proxy other MCP servers.

Nacos-MCP-Go has two working modes:

Router mode: The default mode, which recommends, distributes, installs, and proxies the functions of other MCP Servers through the MCP Server, helping users more conveniently utilize MCP Server services.

Proxy mode: Specified by the environment variable MODE=proxy, it can convert SSE and stdio protocol MCP Servers into streamable HTTP protocol MCP Servers through simple configuration.

## Features

- 🚀 **Easy to use**: API design similar to traditional microservice registration
- 🏷️ **Tag driven**: Automatic parsing of tool descriptions and parameters based on `mcp` tags
- 🔧 **Flexible registration**: Support for registering individual functions or entire service structs
- 🌐 **Nacos integration**: Full support for Nacos MCP registry
- ⚙️ **Option pattern**: Elegant configuration option design
- 🛡️ **Type safety**: Complete Go type to JSON Schema conversion

## Quick Start

### 1. Define Service

```go
package main

import (
    "time"
    nacosmcp "nacos-mcp-go"
    "nacos-mcp-go/registry"
)

// TimeService time service
type TimeService struct{}

// GetCurrentTime get current time
func (ts *TimeService) GetCurrentTime(format string) string {
    if format == "" {
        format = "2006-01-02 15:04:05"
    }
    return time.Now().Format(format)
}

// UserService user service
type UserService struct{}

// SearchUser search user
type SearchUserRequest struct {
    Keyword string `json:"keyword" mcp:"desc=search keyword,required"`
    Limit   int    `json:"limit" mcp:"desc=limit of results"`
}

func (us *UserService) SearchUser(req SearchUserRequest) []string {
    // business logic
    return []string{"user1", "user2", "user3"}
}
```

### 2. Create and Register MCP Server

```go
func main() {
    // Create MCP server
    server := nacosmcp.NewServer("my-mcp-service",
        nacosmcp.WithNamespace(""),
        nacosmcp.WithGroup("DEFAULT_GROUP"),
        nacosmcp.WithAddress("127.0.0.1", 8080),
        nacosmcp.WithMetadata(map[string]string{
            "version": "1.0.0",
        }),
    )

    // Register services
    timeService := &TimeService{}
    userService := &UserService{}

    server.RegisterService(timeService)
    server.RegisterService(userService)

    // You can also register individual functions
    server.RegisterTool(func(message string) string {
        return fmt.Sprintf("Echo: %s", message)
    })

    // Register to Nacos
    ctx := context.Background()
    serverId, err := registry.Register(ctx, server, "127.0.0.1:8848",
        registry.WithAuth("nacos", "nacos"),
        registry.WithNamespace(""),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("✅ Registered with ID: %s\n", serverId)

    // Deregister on graceful shutdown
    defer registry.Deregister(ctx, serverId, "127.0.0.1:8848",
        registry.WithAuth("nacos", "nacos"),
        registry.WithNamespace(""),
    )
}
```

## MCP Tag Syntax

To declare a function as an MCP tool, you need to use the `mcp` tag with the following syntax:

### For Function Fields in Structs

```go
type MyMCPService struct {
    GetTime func() string              `mcp:"tool;name=get_current_time;description=Get current server time"`
    Search  func(string, int) []string `mcp:"tool;name=search_users;description=Search users;paramNames=keyword,limit"`
    Echo    func(string) string        `mcp:"tool;name=echo_message;description=Echo message;paramNames=message"`
}
```

The `mcp` tag supports the following options:
- `tool`: Indicates this field should be treated as an MCP tool (required)
- `name=tool_name`: Sets the tool name (optional, defaults to field name)
- `description=tool description`: Sets the tool description (optional)
- `paramNames=param1,param2`: Sets parameter names for the function (optional)

### For Struct Fields with mcp tags

```go
type Request struct {
    Name     string `json:"name" mcp:"desc=user name,required"`
    Age      int    `json:"age" mcp:"desc=user age"`
    Email    string `json:"email" mcp:"desc=email address,required"`
    Optional string `json:"optional" mcp:"desc=optional parameter"`
}
```

Supported tag options for struct fields:
- `desc=description`: Parameter description
- `required`: Mark as required parameter

## Installation

```bash
go get nacos-mcp-go
```

## API Reference

### Server Options

```go
// WithNamespace set namespace
nacosmcp.WithNamespace("dev")

// WithGroup set service group
nacosmcp.WithGroup("my-group")

// WithAddress set service address
nacosmcp.WithAddress("127.0.0.1", 8080)

// WithMetadata set metadata
nacosmcp.WithMetadata(map[string]string{
    "version": "1.0.0",
    "env": "production",
})
```

### Registry Options

```go
// WithAuth set authentication information
registry.WithAuth("username", "password")

// WithNamespace set namespace
registry.WithNamespace("namespace-id")

// WithTimeout set timeout
registry.WithTimeout(30 * time.Second)
```

## Type Mapping

Go types automatically map to JSON Schema:

| Go Type | JSON Schema Type |
|---------|------------------|
| string | string |
| int, int32, int64 | integer |
| float32, float64 | number |
| bool | boolean |
| []T | array |
| struct | object |

## Environment Variable Settings

| Parameter | Description | Default Value | Required | Remarks |
|-----------|-------------|---------------|----------|---------|
| NACOS_ADDR | Nacos server address | 127.0.0.1:8848 | No | the Nacos server address, e.g., 192.168.1.1:8848. Note: Include the port. |
| NACOS_USERNAME | Nacos username | nacos | No | the Nacos username, e.g., nacos. |
| NACOS_PASSWORD | Nacos password | - | Yes | the Nacos password, e.g., nacos. |
| NACOS_NAMESPACE | Nacos Namespace | public | No | Nacos namespace, e.g. public |
| TRANSPORT_TYPE | Transport protocol type | stdio | No | transport protocol type. Options: stdio, sse, streamable_http. |
| PROXIED_MCP_NAME | Proxied MCP server name | - | No | In proxy mode, specify the MCP server name to be converted. Must be registered in Nacos first. |
| MODE | Working mode | router | No | Available options: router, proxy. |
| ACCESS_KEY_ID | Aliyun ram access key id | - | No | |
| ACCESS_KEY_SECRET | Aliyun ram access key secret | - | No | |

## Development

If you are doing local development, simply follow the steps:

1. Clone this repo into your local environment.
2. Modify codes in the project to implement your wanted features.
3. Test using the Claude desktop app or other MCP compatible applications.

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.