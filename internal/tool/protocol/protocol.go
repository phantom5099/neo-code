package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"neo-code/internal/tool"
)

type singleCallEnvelope struct {
	Type      string         `json:"type,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
}

type multiCallEnvelope struct {
	Type  string               `json:"type,omitempty"`
	Calls []singleCallEnvelope `json:"calls,omitempty"`
}

func ParseAssistantToolCalls(text string) []tool.ToolCall {
	payload := unwrapJSONPayload(text)
	if payload == "" {
		return nil
	}

	var multiple multiCallEnvelope
	if err := json.Unmarshal([]byte(payload), &multiple); err == nil {
		if strings.EqualFold(strings.TrimSpace(multiple.Type), "tool_calls") && len(multiple.Calls) > 0 {
			return normalizeCalls(multiple.Calls)
		}
	}

	var single singleCallEnvelope
	if err := json.Unmarshal([]byte(payload), &single); err != nil {
		return nil
	}

	call := normalizeSingleCall(single)
	if strings.TrimSpace(call.Tool) == "" {
		return nil
	}
	return []tool.ToolCall{call}
}

func RenderInstructionBlock(definitions []tool.ToolDefinition) string {
	if len(definitions) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("[TOOLS]\n")
	builder.WriteString("If you need a tool, respond with JSON only. Prefer this envelope:\n")
	builder.WriteString("{\"type\":\"tool_call\",\"name\":\"read\",\"arguments\":{\"filePath\":\"README.md\"}}\n")
	builder.WriteString("Legacy compatibility is still accepted:\n")
	builder.WriteString("{\"tool\":\"read\",\"params\":{\"filePath\":\"README.md\"}}\n")
	builder.WriteString("Use at most one tool call per turn unless multiple calls are truly independent.\n")
	builder.WriteString("Available tools:\n")

	currentCategory := ""
	for _, definition := range definitions {
		schema, err := json.Marshal(definition.JSONSchema())
		if err != nil {
			schema = []byte("{}")
		}
		category := strings.TrimSpace(definition.Category)
		if category == "" {
			category = "general"
		}
		if category != currentCategory {
			builder.WriteString(fmt.Sprintf("* %s\n", category))
			currentCategory = category
		}
		builder.WriteString(fmt.Sprintf("- %s: %s\n  schema=%s\n", definition.Name, definition.Description, string(schema)))
	}

	builder.WriteString("[/TOOLS]")
	return builder.String()
}

func unwrapJSONPayload(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed
	}
	return ""
}

func normalizeCalls(calls []singleCallEnvelope) []tool.ToolCall {
	result := make([]tool.ToolCall, 0, len(calls))
	for _, call := range calls {
		normalized := normalizeSingleCall(call)
		if strings.TrimSpace(normalized.Tool) == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func normalizeSingleCall(call singleCallEnvelope) tool.ToolCall {
	name := strings.TrimSpace(call.Name)
	params := call.Arguments
	if name == "" {
		name = strings.TrimSpace(call.Tool)
		params = call.Params
	}

	return tool.ToolCall{
		Tool:   name,
		Params: tool.NormalizeParams(params),
	}
}
