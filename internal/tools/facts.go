package tools

import (
	"strings"

	"neo-code/internal/security"
)

const (
	metadataKeyWorkspaceWrite        = "workspace_write"
	metadataKeyVerificationPerformed = "verification_performed"
	metadataKeyVerificationPassed    = "verification_passed"
	metadataKeyVerificationScope     = "verification_scope"
)

// EnrichToolResultFacts 基于权限动作与工具返回元数据补齐结构化执行事实。
func EnrichToolResultFacts(action security.Action, result ToolResult) ToolResult {
	facts := result.Facts
	metadata := result.Metadata

	if value, ok := metadataBool(metadata, metadataKeyWorkspaceWrite); ok {
		facts.WorkspaceWrite = value
	} else {
		facts.WorkspaceWrite = facts.WorkspaceWrite || defaultWorkspaceWriteFromAction(action)
	}

	performed, hasPerformed := metadataBool(metadata, metadataKeyVerificationPerformed)
	passed, hasPassed := metadataBool(metadata, metadataKeyVerificationPassed)
	scope, hasScope := metadataString(metadata, metadataKeyVerificationScope)
	if hasPerformed {
		facts.VerificationPerformed = performed
	}
	if hasPassed {
		facts.VerificationPassed = passed
	}
	if hasScope {
		facts.VerificationScope = scope
	}
	if facts.VerificationPassed {
		facts.VerificationPerformed = true
	}
	if !facts.VerificationPerformed {
		facts.VerificationPassed = false
		facts.VerificationScope = ""
	}

	result.Facts = facts
	return result
}

// defaultWorkspaceWriteFromAction 按权限动作类型推导默认写入事实，未知能力按可写处理。
func defaultWorkspaceWriteFromAction(action security.Action) bool {
	switch action.Type {
	case security.ActionTypeRead:
		return false
	case security.ActionTypeWrite, security.ActionTypeMCP, security.ActionTypeBash:
		return true
	default:
		return true
	}
}

// metadataBool 读取结果元数据中的布尔键值，并做大小写兼容。
func metadataBool(metadata map[string]any, key string) (bool, bool) {
	if len(metadata) == 0 {
		return false, false
	}
	raw, ok := metadata[key]
	if !ok {
		return false, false
	}
	value, ok := raw.(bool)
	return value, ok
}

// metadataString 读取结果元数据中的字符串键值，并在空白值时返回未设置。
func metadataString(metadata map[string]any, key string) (string, bool) {
	if len(metadata) == 0 {
		return "", false
	}
	raw, ok := metadata[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}
