package tool

import (
	"fmt"
	"sort"
)

// GlobalRegistry is the shared tool registry used by the local runtime.
var GlobalRegistry = NewToolRegistry()

// ToolRegistry registers tools and dispatches tool calls.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a registry with the built-in file and shell tools.
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]Tool)}
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&ListTool{})
	r.Register(&GrepTool{})
	r.Register(&BashTool{})
	r.Register(&WebFetchTool{})
	r.Register(&WebSearchTool{})
	return r
}

// Register adds a tool definition to the registry.
func (r *ToolRegistry) Register(t Tool) {
	def := t.Definition()
	r.tools[def.Name] = t
}

// Get returns a registered tool by name.
func (r *ToolRegistry) Get(name string) Tool {
	return r.tools[name]
}

// ListDefinitions returns all registered tool definitions in stable order.
func (r *ToolRegistry) ListDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Category == defs[j].Category {
			return defs[i].Name < defs[j].Name
		}
		return defs[i].Category < defs[j].Category
	})
	return defs
}

// ListTools returns the registered tool names.
func (r *ToolRegistry) ListTools() []string {
	defs := r.ListDefinitions()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

// Execute normalizes params and runs the named tool.
func (r *ToolRegistry) Execute(call ToolCall) *ToolResult {
	t := r.Get(call.Tool)
	if t == nil {
		return &ToolResult{
			ToolName: call.Tool,
			Success:  false,
			Error:    fmt.Sprintf("unsupported tool: %s", call.Tool),
		}
	}

	params := NormalizeParams(call.Params)
	result := t.Run(params)
	if result == nil {
		return &ToolResult{
			ToolName: call.Tool,
			Success:  false,
			Error:    "tool did not return a result",
		}
	}
	if result.ToolName == "" {
		result.ToolName = call.Tool
	}
	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}
	result.Metadata["tool"] = call.Tool
	return result
}
