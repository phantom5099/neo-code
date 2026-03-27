package tool

import (
	"fmt"
	"strconv"
	"strings"
)

func requiredString(params map[string]interface{}, key string) (string, *ToolResult) {
	if params == nil {
		return "", invalidParamResult("missing required parameter: %s", key)
	}

	value, ok := params[key]
	if !ok {
		return "", invalidParamResult("missing required parameter: %s", key)
	}

	str, ok := value.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return "", invalidParamResult("parameter %q must be a non-empty string", key)
	}

	return str, nil
}

func optionalString(params map[string]interface{}, key, fallback string) (string, *ToolResult) {
	if params == nil {
		return fallback, nil
	}

	value, ok := params[key]
	if !ok {
		return fallback, nil
	}

	str, ok := value.(string)
	if !ok {
		return "", invalidParamResult("parameter %q must be a string", key)
	}
	if strings.TrimSpace(str) == "" {
		return fallback, nil
	}

	return str, nil
}

func optionalInt(params map[string]interface{}, key string, fallback int) (int, *ToolResult) {
	if params == nil {
		return fallback, nil
	}

	value, ok := params[key]
	if !ok {
		return fallback, nil
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, invalidParamResult("parameter %q must be an integer", key)
		}
		return parsed, nil
	default:
		return 0, invalidParamResult("parameter %q must be an integer", key)
	}
}

func optionalBool(params map[string]interface{}, key string, fallback bool) (bool, *ToolResult) {
	if params == nil {
		return fallback, nil
	}

	value, ok := params[key]
	if !ok {
		return fallback, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "on":
			return true, nil
		case "false", "0", "no", "n", "off":
			return false, nil
		default:
			return false, invalidParamResult("parameter %q must be a boolean", key)
		}
	default:
		return false, invalidParamResult("parameter %q must be a boolean", key)
	}
}

func NormalizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{}, len(params))
	for key, value := range params {
		result[snakeToCamel(strings.TrimSpace(key))] = normalizeParamValue(value)
	}
	return result
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) <= 1 {
		return s
	}

	var builder strings.Builder
	builder.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

func normalizeParamValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return NormalizeParams(typed)
	case []interface{}:
		items := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeParamValue(item))
		}
		return items
	default:
		return value
	}
}

func invalidParamResult(format string, args ...interface{}) *ToolResult {
	return &ToolResult{
		Success: false,
		Error:   fmt.Sprintf(format, args...),
	}
}
