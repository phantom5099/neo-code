package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	// DefaultOutputLimitBytes is the default max size of tool output content.
	DefaultOutputLimitBytes = 64 * 1024
	truncatedSuffix         = "\n...[truncated]"
)

// ApplyOutputLimit truncates tool output content and adds a truncated metadata flag.
func ApplyOutputLimit(result ToolResult, limit int) ToolResult {
	if limit <= 0 {
		return result
	}
	if len(result.Content) <= limit {
		return result
	}

	result.Content = result.Content[:limit] + truncatedSuffix
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	if existing, ok := result.Metadata["truncated"].(bool); !ok || !existing {
		result.Metadata["truncated"] = true
	}
	return result
}

// FormatToolResultForModel 将工具执行结果渲染为稳定的结构化文本，便于模型准确消费回灌信息。
func FormatToolResultForModel(result ToolResult) string {
	lines := []string{"tool result"}

	if toolName := strings.TrimSpace(result.Name); toolName != "" {
		lines = append(lines, "tool: "+toolName)
	}

	status := "ok"
	if result.IsError {
		status = "error"
	}
	lines = append(lines, "status: "+status)

	if toolCallID := strings.TrimSpace(result.ToolCallID); toolCallID != "" {
		lines = append(lines, "tool_call_id: "+toolCallID)
	}

	lines = append(lines, fmt.Sprintf("truncated: %t", toolResultTruncated(result.Metadata)))
	lines = append(lines, formatToolResultMetadataLines(result.Metadata)...)

	if strings.TrimSpace(result.Content) != "" {
		lines = append(lines, "", "content:", result.Content)
	}

	return strings.Join(lines, "\n")
}

// FormatError builds a consistent error payload for tool failures.
func FormatError(toolName string, reason string, details string) string {
	toolName = strings.TrimSpace(toolName)
	reason = strings.TrimSpace(reason)
	details = strings.TrimSpace(details)

	lines := []string{"tool error"}
	if toolName != "" {
		lines = append(lines, "tool: "+toolName)
	}
	if reason != "" {
		lines = append(lines, "reason: "+reason)
	}
	if details != "" {
		lines = append(lines, "details: "+details)
	}

	return strings.Join(lines, "\n")
}

// NormalizeErrorReason strips the tool name prefix from an error message, if present.
func NormalizeErrorReason(toolName string, err error) string {
	if err == nil {
		return ""
	}
	reason := strings.TrimSpace(err.Error())
	if toolName == "" {
		return reason
	}

	prefix := strings.ToLower(strings.TrimSpace(toolName)) + ":"
	lower := strings.ToLower(reason)
	if strings.HasPrefix(lower, prefix) {
		return strings.TrimSpace(reason[len(prefix):])
	}
	return reason
}

// NewErrorResult returns a standardized error ToolResult.
func NewErrorResult(toolName string, reason string, details string, metadata map[string]any) ToolResult {
	return ToolResult{
		Name:     toolName,
		Content:  FormatError(toolName, reason, details),
		IsError:  true,
		Metadata: metadata,
	}
}

// toolResultTruncated 从 metadata 中提取统一的截断标记，缺省时返回 false。
func toolResultTruncated(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	truncated, _ := metadata["truncated"].(bool)
	return truncated
}

// formatToolResultMetadataLines 以稳定顺序输出供模型消费的 metadata 行，并跳过已提升为顶层字段的键。
func formatToolResultMetadataLines(metadata map[string]any) []string {
	if len(metadata) == 0 {
		return nil
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		key = strings.TrimSpace(key)
		if key == "" || key == "truncated" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, "meta."+key+": "+formatToolResultValue(metadata[key]))
	}
	return lines
}

// formatToolResultValue 将 metadata 值收敛为单行文本，避免破坏工具结果包络格式。
func formatToolResultValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return strings.NewReplacer("\r\n", "\\n", "\n", "\\n", "\r", "\\n").Replace(v)
	case fmt.Stringer:
		return strings.NewReplacer("\r\n", "\\n", "\n", "\\n", "\r", "\\n").Replace(v.String())
	}

	payload, err := json.Marshal(value)
	if err == nil {
		return string(payload)
	}
	return fmt.Sprint(value)
}
