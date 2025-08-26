package scanner

import (
	"fmt"
	"reflect"
	"strings"
)

// ToolInfo 工具信息
type ToolInfo struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     interface{}
}

// ScanTool 扫描函数并解析MCP工具信息
func ScanTool(handler interface{}) (*ToolInfo, error) {
	handlerValue := reflect.ValueOf(handler)
	handlerType := reflect.TypeOf(handler)

	if handlerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler must be a function")
	}

	// 解析函数名作为默认工具名
	funcName := getFunctionName(handlerValue)
	toolName := strings.ToLower(funcName)

	// 解析函数参数，构建输入schema
	inputSchema, err := buildInputSchema(handlerType)
	if err != nil {
		return nil, fmt.Errorf("build input schema failed: %w", err)
	}

	return &ToolInfo{
		Name:        toolName,
		Description: fmt.Sprintf("Auto-generated tool for %s", funcName),
		InputSchema: inputSchema,
		Handler:     handler,
	}, nil
}

// ScanStruct 扫描结构体字段并解析MCP工具信息
// 支持形如: GetTime func() string `mcp:"tool;name=get_current_time;description=获取服务器当前时间"`
func ScanStruct(obj interface{}) ([]*ToolInfo, error) {
	objValue := reflect.ValueOf(obj)
	objType := reflect.TypeOf(obj)

	if objType.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
		objType = objType.Elem()
	}

	if objType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("object must be a struct or pointer to struct")
	}

	var tools []*ToolInfo

	// 遍历结构体字段
	for i := 0; i < objType.NumField(); i++ {
		field := objType.Field(i)
		fieldValue := objValue.Field(i)

		// 检查字段是否是函数类型且有mcp tag
		if field.Type.Kind() == reflect.Func {
			mcpTag := field.Tag.Get("mcp")
			if mcpTag != "" && strings.Contains(mcpTag, "tool") {
				if tool, err := parseFieldAsTool(fieldValue.Interface(), field, mcpTag); err == nil {
					tools = append(tools, tool)
				}
			}
		}
	}

	// 如果没有找到函数字段，则尝试扫描方法（向后兼容）
	if len(tools) == 0 {
		return scanStructMethods(objValue, objType)
	}

	return tools, nil
}

// scanStructMethods 扫描结构体方法（向后兼容）
func scanStructMethods(objValue reflect.Value, objType reflect.Type) ([]*ToolInfo, error) {
	var tools []*ToolInfo

	// 遍历结构体方法
	for i := 0; i < objValue.NumMethod(); i++ {
		method := objValue.Method(i)
		methodType := objType.Method(i)

		// 检查是否有mcp tag
		if tool, err := parseMethodAsTool(method.Interface(), methodType); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools, nil
}

// parseMethodAsTool 解析方法为MCP工具
func parseMethodAsTool(method interface{}, methodType reflect.Method) (*ToolInfo, error) {
	// 检查方法是否导出
	if !methodType.IsExported() {
		return nil, fmt.Errorf("method %s is not exported", methodType.Name)
	}

	// 解析方法签名
	funcType := methodType.Type
	inputSchema, err := buildInputSchema(funcType)
	if err != nil {
		return nil, fmt.Errorf("build input schema for method %s failed: %w", methodType.Name, err)
	}

	return &ToolInfo{
		Name:        strings.ToLower(methodType.Name),
		Description: fmt.Sprintf("Auto-generated tool for method %s", methodType.Name),
		InputSchema: inputSchema,
		Handler:     method,
	}, nil
}

// buildInputSchema 构建输入schema
func buildInputSchema(funcType reflect.Type) (map[string]interface{}, error) {
	properties := make(map[string]interface{})
	required := []string{}

	// 跳过receiver参数（如果是方法）
	startIdx := 0
	if funcType.NumIn() > 0 {
		// 检查第一个参数是否是receiver
		firstParam := funcType.In(0)
		if firstParam.Kind() == reflect.Ptr || firstParam.Kind() == reflect.Struct {
			startIdx = 1
		}
	}

	// 解析函数参数
	for i := startIdx; i < funcType.NumIn(); i++ {
		paramType := funcType.In(i)
		paramName := fmt.Sprintf("param%d", i-startIdx+1)

		// 解析参数类型为JSON Schema
		paramSchema, err := typeToJSONSchema(paramType)
		if err != nil {
			return nil, fmt.Errorf("convert parameter %d to JSON schema failed: %w", i, err)
		}

		properties[paramName] = paramSchema
		required = append(required, paramName)
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// typeToJSONSchema 将Go类型转换为JSON Schema
func typeToJSONSchema(t reflect.Type) (map[string]interface{}, error) {
	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{
			"type":        "string",
			"description": "String parameter",
		}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]interface{}{
			"type":        "integer",
			"description": "Integer parameter",
		}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{
			"type":        "number",
			"description": "Number parameter",
		}, nil
	case reflect.Bool:
		return map[string]interface{}{
			"type":        "boolean",
			"description": "Boolean parameter",
		}, nil
	case reflect.Slice, reflect.Array:
		elemSchema, err := typeToJSONSchema(t.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"type":        "array",
			"items":       elemSchema,
			"description": "Array parameter",
		}, nil
	case reflect.Struct:
		return parseStructToSchema(t)
	case reflect.Ptr:
		return typeToJSONSchema(t.Elem())
	default:
		return map[string]interface{}{
			"type":        "object",
			"description": "Complex parameter",
		}, nil
	}
}

// parseStructToSchema 解析结构体为JSON Schema
func parseStructToSchema(t reflect.Type) (map[string]interface{}, error) {
	properties := make(map[string]interface{})
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		mcpTag := field.Tag.Get("mcp")

		// 解析json tag
		if jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// 解析字段类型
		fieldSchema, err := typeToJSONSchema(field.Type)
		if err != nil {
			return nil, fmt.Errorf("parse field %s failed: %w", field.Name, err)
		}

		// 解析mcp tag中的描述信息
		if mcpTag != "" {
			parts := strings.Split(mcpTag, ",")
			for _, part := range parts {
				if strings.HasPrefix(part, "desc=") {
					fieldSchema["description"] = strings.TrimPrefix(part, "desc=")
				}
				if part == "required" {
					required = append(required, fieldName)
				}
			}
		}

		properties[fieldName] = fieldSchema
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// parseFieldAsTool 解析函数字段为MCP工具
func parseFieldAsTool(fn interface{}, field reflect.StructField, mcpTag string) (*ToolInfo, error) {
	// 解析mcp tag
	toolName, description, paramNames, err := parseMcpTag(mcpTag)
	if err != nil {
		return nil, fmt.Errorf("parse mcp tag failed: %w", err)
	}

	// 如果没有指定工具名，使用字段名
	if toolName == "" {
		toolName = strings.ToLower(field.Name)
	}

	// 构建输入schema
	inputSchema, err := buildFunctionInputSchema(field.Type, paramNames)
	if err != nil {
		return nil, fmt.Errorf("build input schema failed: %w", err)
	}

	return &ToolInfo{
		Name:        toolName,
		Description: description,
		InputSchema: inputSchema,
		Handler:     fn,
	}, nil
}

// parseMcpTag 解析mcp tag
// 格式: "tool;name=get_current_time;description=获取服务器当前时间;paramNames=keyword,limit"
func parseMcpTag(tag string) (name, description string, paramNames []string, err error) {
	parts := strings.Split(tag, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "tool" {
			continue
		}

		if strings.HasPrefix(part, "name=") {
			name = strings.TrimPrefix(part, "name=")
		} else if strings.HasPrefix(part, "description=") {
			description = strings.TrimPrefix(part, "description=")
		} else if strings.HasPrefix(part, "paramNames=") {
			paramNamesStr := strings.TrimPrefix(part, "paramNames=")
			if paramNamesStr != "" {
				paramNames = strings.Split(paramNamesStr, ",")
				for i, pname := range paramNames {
					paramNames[i] = strings.TrimSpace(pname)
				}
			}
		}
	}

	return name, description, paramNames, nil
}

// buildFunctionInputSchema 构建函数输入schema
func buildFunctionInputSchema(funcType reflect.Type, paramNames []string) (map[string]interface{}, error) {
	properties := make(map[string]interface{})
	required := []string{}

	// 解析函数参数
	for i := 0; i < funcType.NumIn(); i++ {
		paramType := funcType.In(i)

		// 确定参数名
		var paramName string
		if i < len(paramNames) && paramNames[i] != "" {
			paramName = paramNames[i]
		} else {
			paramName = fmt.Sprintf("param%d", i+1)
		}

		// 解析参数类型为JSON Schema
		paramSchema, err := typeToJSONSchema(paramType)
		if err != nil {
			return nil, fmt.Errorf("convert parameter %d to JSON schema failed: %w", i, err)
		}

		properties[paramName] = paramSchema
		required = append(required, paramName)
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// getFunctionName 获取函数名
func getFunctionName(fn reflect.Value) string {
	fullName := fn.Type().String()
	if strings.Contains(fullName, ".") {
		parts := strings.Split(fullName, ".")
		return parts[len(parts)-1]
	}
	return fullName
}
