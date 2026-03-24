package tools

import (
	"fmt"
	"sort"

	"go-llm-demo/internal/server/domain"
)

type ToolResult = domain.ToolResult
type Tool = domain.Tool
type ToolDefinition = domain.ToolDefinition
type ToolParamSpec = domain.ToolParamSpec

// GlobalRegistry 是ToolRegistry的单例实例。
var GlobalRegistry = NewToolRegistry()

// ToolRegistry 管理工具的注册和检索。
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry 创建并注册默认工具集合。
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&BashTool{})
	r.Register(&ListTool{})
	r.Register(&GrepTool{})
	return r
}

// Register 向注册表添加一个工具。
func (r *ToolRegistry) Register(tool Tool) {
	def := tool.Definition()
	r.tools[def.Name] = tool
}

// Get 根据名称从注册表中检索一个工具。
func (r *ToolRegistry) Get(name string) Tool {
	return r.tools[name]
}

// ListDefinitions 返回所有已注册工具定义。
func (r *ToolRegistry) ListDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

// ListTools 返回所有已注册工具名称的切片。
func (r *ToolRegistry) ListTools() []string {
	defs := r.ListDefinitions()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Name)
	}
	return keys
}

// Execute 执行指定工具并补齐基础元数据。
func (r *ToolRegistry) Execute(call domain.ToolCall) *ToolResult {
	tool := r.Get(call.Tool)
	if tool == nil {
		return &ToolResult{ToolName: call.Tool, Success: false, Error: fmt.Sprintf("不支持的工具: %s", call.Tool)}
	}
	// 统一规范参数命名，避免模型同时输出 snake_case 与 camelCase 时各工具重复兼容。
	params := NormalizeParams(call.Params)
	result := tool.Run(params)
	if result == nil {
		return &ToolResult{ToolName: call.Tool, Success: false, Error: "工具未返回结果"}
	}
	if result.ToolName == "" {
		result.ToolName = call.Tool
	}
	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}
	// 为后续日志、UI 展示和上下文回灌补齐最基础的工具元信息。
	result.Metadata["tool"] = call.Tool
	return result
}
