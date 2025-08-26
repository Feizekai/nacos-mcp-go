package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"nacos-mcp-go/types"
)

// HTTPHandler 封装 MCP HTTP 接口
type HTTPHandler struct {
	server types.ServerInterface
	mu     sync.RWMutex
}

// NewHTTPHandler 创建新的处理器
func NewHTTPHandler(server types.ServerInterface) *HTTPHandler {
	return &HTTPHandler{
		server: server,
	}
}

// RegisterRoutes 注册 MCP 路由到 http.ServeMux
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/mcp/tools", h.listTools)
	mux.HandleFunc("/mcp/tools/", h.invokeTool)
	mux.HandleFunc("/mcp/info", h.serverInfo)
}

// serverInfo 处理 /mcp/info - 返回服务器信息
func (h *HTTPHandler) serverInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	info := map[string]interface{}{
		"name":      h.server.GetName(),
		"protocol":  string(h.server.GetProtocol()),
		"namespace": h.server.GetNamespace(),
		"group":     h.server.GetGroup(),
		"metadata":  h.server.GetMetadata(),
		"toolCount": len(h.server.GetTools()),
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("Error encoding server info: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// listTools 处理 /mcp/tools - 返回工具列表
func (h *HTTPHandler) listTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	tools := h.server.GetTools()
	h.mu.RUnlock()

	// 转换为标准MCP工具格式
	mcpTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		mcpTools[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
	}

	response := map[string]interface{}{
		"tools": mcpTools,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding tools list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// invokeTool 处理 /mcp/tools/{name}/invoke - 调用工具
func (h *HTTPHandler) invokeTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析工具名
	path := strings.TrimPrefix(r.URL.Path, "/mcp/tools/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "invoke" {
		http.Error(w, "Bad Request: Invalid path format", http.StatusBadRequest)
		return
	}
	toolName := parts[0]

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request: Cannot read body", http.StatusBadRequest)
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

	// 查找并调用工具
	result, err := h.callTool(toolName, req.Arguments)
	if err != nil {
		log.Printf("Error calling tool %s: %v", toolName, err)
		http.Error(w, fmt.Sprintf("Tool execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("%v", result),
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// callTool 调用指定的工具
func (h *HTTPHandler) callTool(toolName string, arguments map[string]interface{}) (interface{}, error) {
	h.mu.RLock()
	tools := h.server.GetTools()
	h.mu.RUnlock()

	// 查找工具
	var targetTool *types.Tool
	for i := range tools {
		if tools[i].Name == toolName {
			targetTool = &tools[i]
			break
		}
	}

	if targetTool == nil {
		return nil, fmt.Errorf("tool '%s' not found", toolName)
	}

	// 调用工具函数
	return h.invokeHandler(targetTool.Handler, arguments)
}

// invokeHandler 通过反射调用处理器函数
func (h *HTTPHandler) invokeHandler(handler interface{}, arguments map[string]interface{}) (interface{}, error) {
	handlerValue := reflect.ValueOf(handler)
	handlerType := reflect.TypeOf(handler)

	if handlerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler is not a function")
	}

	// 准备参数
	numIn := handlerType.NumIn()
	args := make([]reflect.Value, numIn)

	// 根据函数签名转换参数
	for i := 0; i < numIn; i++ {
		paramType := handlerType.In(i)

		// 尝试从arguments中获取参数
		var paramValue interface{}
		if i == 0 && numIn == 1 {
			// 单参数情况，可能是整个arguments对象
			if paramType.Kind() == reflect.Struct {
				paramValue = arguments
			} else {
				// 尝试获取第一个参数值
				for _, v := range arguments {
					paramValue = v
					break
				}
			}
		} else {
			// 多参数情况，按顺序获取
			paramName := fmt.Sprintf("param%d", i+1)
			if val, exists := arguments[paramName]; exists {
				paramValue = val
			} else {
				// 尝试按参数名获取
				for key, val := range arguments {
					if strings.EqualFold(key, paramName) {
						paramValue = val
						break
					}
				}
			}
		}

		// 转换参数类型
		convertedValue, err := h.convertValue(paramValue, paramType)
		if err != nil {
			return nil, fmt.Errorf("convert parameter %d failed: %w", i, err)
		}
		args[i] = convertedValue
	}

	// 调用函数
	results := handlerValue.Call(args)

	// 处理返回值
	if len(results) == 0 {
		return nil, nil
	}

	// 返回第一个结果
	result := results[0].Interface()
	return result, nil
}

// convertValue 转换参数值到指定类型
func (h *HTTPHandler) convertValue(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(targetType), nil
	}

	valueType := reflect.TypeOf(value)

	// 如果类型匹配，直接返回
	if valueType == targetType {
		return reflect.ValueOf(value), nil
	}

	// 处理基本类型转换
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("%v", value)), nil
	case reflect.Int, reflect.Int32, reflect.Int64:
		if num, ok := value.(float64); ok {
			return reflect.ValueOf(int(num)).Convert(targetType), nil
		}
		if num, ok := value.(int); ok {
			return reflect.ValueOf(num).Convert(targetType), nil
		}
	case reflect.Float32, reflect.Float64:
		if num, ok := value.(float64); ok {
			return reflect.ValueOf(num).Convert(targetType), nil
		}
		if num, ok := value.(int); ok {
			return reflect.ValueOf(float64(num)).Convert(targetType), nil
		}
	case reflect.Bool:
		if b, ok := value.(bool); ok {
			return reflect.ValueOf(b), nil
		}
	case reflect.Slice:
		if slice, ok := value.([]interface{}); ok {
			elemType := targetType.Elem()
			result := reflect.MakeSlice(targetType, len(slice), len(slice))
			for i, item := range slice {
				convertedItem, err := h.convertValue(item, elemType)
				if err != nil {
					return reflect.Value{}, err
				}
				result.Index(i).Set(convertedItem)
			}
			return result, nil
		}
	case reflect.Struct:
		// 处理结构体参数
		if argMap, ok := value.(map[string]interface{}); ok {
			return h.mapToStruct(argMap, targetType)
		}
	}

	// 尝试直接转换
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Type().ConvertibleTo(targetType) {
		return valueReflect.Convert(targetType), nil
	}

	return reflect.Zero(targetType), fmt.Errorf("cannot convert %T to %s", value, targetType)
}

// mapToStruct 将map转换为结构体
func (h *HTTPHandler) mapToStruct(argMap map[string]interface{}, structType reflect.Type) (reflect.Value, error) {
	structValue := reflect.New(structType).Elem()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// 获取字段名（优先使用json tag）
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// 从map中获取值
		if value, exists := argMap[fieldName]; exists {
			convertedValue, err := h.convertValue(value, field.Type)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("convert field %s failed: %w", fieldName, err)
			}
			fieldValue.Set(convertedValue)
		}
	}

	return structValue, nil
}
