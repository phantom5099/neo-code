package types

import "encoding/json"

// RoleSystem 标识系统消息。
const RoleSystem = "system"

// RoleUser 标识用户消息。
const RoleUser = "user"

// RoleAssistant 标识助手消息。
const RoleAssistant = "assistant"

// RoleTool 标识工具结果消息。
const RoleTool = "tool"

// Message 表示对话中的单条消息。
type Message struct {
	Role         string            `json:"role"`
	Parts        []ContentPart     `json:"parts,omitempty"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty"`
	ToolCallID   string            `json:"tool_call_id,omitempty"`
	IsError      bool              `json:"is_error,omitempty"`
	ToolMetadata map[string]string `json:"tool_metadata,omitempty"`
}

// IsEmpty checks if the message has no content parts and no tool calls.
func (m *Message) IsEmpty() bool {
	return len(m.Parts) == 0 && len(m.ToolCalls) == 0
}

// Validate ensures the message is well-formed.
func (m *Message) Validate() error {
	return ValidateParts(m.Parts)
}

// UnmarshalJSON 兼容旧版 content 字段，确保历史消息在升级后仍可读。
func (m *Message) UnmarshalJSON(data []byte) error {
	type messageAlias Message
	var raw struct {
		messageAlias
		LegacyContent *string `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = Message(raw.messageAlias)
	if len(m.Parts) == 0 && raw.LegacyContent != nil && *raw.LegacyContent != "" {
		m.Parts = []ContentPart{NewTextPart(*raw.LegacyContent)}
	}
	return nil
}

// ToolCall 表示模型发起的工具调用请求。
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSpec 表示暴露给模型的可调用工具描述。
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}
