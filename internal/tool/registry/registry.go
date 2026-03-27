package registry

import (
	"fmt"
	"sort"

	"neo-code/internal/tool"
	"neo-code/internal/tool/filesystem"
	"neo-code/internal/tool/shell"
	toolweb "neo-code/internal/tool/web"
)

var GlobalRegistry = NewToolRegistry()

type ToolRegistry struct {
	tools map[string]tool.Tool
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]tool.Tool)}
	r.Register(filesystem.NewReadTool())
	r.Register(filesystem.NewWriteTool())
	r.Register(filesystem.NewEditTool())
	r.Register(filesystem.NewListTool())
	r.Register(filesystem.NewGrepTool())
	r.Register(shell.NewBashTool())
	r.Register(toolweb.NewFetchTool())
	r.Register(toolweb.NewSearchTool())
	return r
}

func (r *ToolRegistry) Register(t tool.Tool) {
	def := t.Definition()
	r.tools[def.Name] = t
}

func (r *ToolRegistry) Get(name string) tool.Tool {
	return r.tools[name]
}

func (r *ToolRegistry) ListDefinitions() []tool.ToolDefinition {
	defs := make([]tool.ToolDefinition, 0, len(r.tools))
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

func (r *ToolRegistry) ListTools() []string {
	defs := r.ListDefinitions()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

func (r *ToolRegistry) Execute(call tool.ToolCall) *tool.ToolResult {
	t := r.Get(call.Tool)
	if t == nil {
		return &tool.ToolResult{
			ToolName: call.Tool,
			Success:  false,
			Error:    fmt.Sprintf("unsupported tool: %s", call.Tool),
		}
	}

	params := tool.NormalizeParams(call.Params)
	result := t.Run(params)
	if result == nil {
		return &tool.ToolResult{
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
