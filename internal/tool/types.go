package tool

import (
	"encoding/json"
	"slices"
	"strings"
)

type ToolCall struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

type ToolName string

const (
	ToolRead      ToolName = "Read"
	ToolWrite     ToolName = "Write"
	ToolEdit      ToolName = "Edit"
	ToolBash      ToolName = "Bash"
	ToolList      ToolName = "List"
	ToolGrep      ToolName = "Grep"
	ToolWebFetch  ToolName = "Webfetch"
	ToolWebSearch ToolName = "Websearch"
	ToolTodo      ToolName = "Todo"
)

func ParseToolName(input string) (ToolName, bool) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "read":
		return ToolRead, true
	case "write":
		return ToolWrite, true
	case "edit":
		return ToolEdit, true
	case "bash":
		return ToolBash, true
	case "list":
		return ToolList, true
	case "grep":
		return ToolGrep, true
	case "webfetch", "web_fetch":
		return ToolWebFetch, true
	case "websearch", "web_search":
		return ToolWebSearch, true
	case "todo":
		return ToolTodo, true
	default:
		return "", false
	}
}

type Tool interface {
	Definition() ToolDefinition
	Run(params map[string]interface{}) *ToolResult
}

type ToolResult struct {
	ToolName string                 `json:"tool"`
	Success  bool                   `json:"success"`
	Output   string                 `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ToolDefinition struct {
	Category    string          `json:"category,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  []ToolParamSpec `json:"parameters"`
	InputSchema map[string]any  `json:"inputSchema,omitempty"`
}

type ToolParamSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func (d ToolDefinition) JSONSchema() map[string]any {
	if len(d.InputSchema) > 0 {
		return cloneMap(d.InputSchema)
	}
	return ParameterSpecsToJSONSchema(d.Parameters)
}

func ParameterSpecsToJSONSchema(params []ToolParamSpec) map[string]any {
	properties := map[string]any{}
	required := make([]string, 0, len(params))

	for _, param := range params {
		schema := map[string]any{
			"type":        normalizeSchemaType(param.Type),
			"description": strings.TrimSpace(param.Description),
		}
		properties[param.Name] = schema
		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		slices.Sort(required)
		schema["required"] = required
	}

	return schema
}

func normalizeSchemaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "int", "integer":
		return "integer"
	case "number", "float":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}

	out := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		case []string:
			out[key] = append([]string(nil), typed...)
		case []any:
			items := make([]any, 0, len(typed))
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items = append(items, cloneMap(nested))
					continue
				}
				items = append(items, item)
			}
			out[key] = items
		default:
			out[key] = value
		}
	}
	return out
}

func (tr *ToolResult) MarshalJSON() ([]byte, error) {
	type Alias ToolResult
	return json.Marshal(&struct {
		*Alias
		Output string `json:"output,omitempty"`
		Error  string `json:"error,omitempty"`
	}{
		Alias:  (*Alias)(tr),
		Output: tr.Output,
		Error:  tr.Error,
	})
}
