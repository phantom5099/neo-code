package context

import (
	"strings"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/tools"
)

// ProjectToolMessagesForModel 原地投影 tool 消息，复用主链路对模型可见的只读格式化规则。
func ProjectToolMessagesForModel(messages []providertypes.Message) []providertypes.Message {
	for i := range messages {
		message := messages[i]
		if message.Role != providertypes.RoleTool {
			continue
		}
		if len(message.ToolMetadata) == 0 {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" || content == microCompactClearedMessage {
			continue
		}
		messages[i].Content = tools.FormatToolMessageForModel(message)
		messages[i].ToolMetadata = nil
	}
	return messages
}

// BuildRecentMessagesForModel 从会话尾部构造 provider-safe 的最近消息窗口，避免保留非法 tool call 片段。
func BuildRecentMessagesForModel(messages []providertypes.Message, limit int) []providertypes.Message {
	if len(messages) == 0 || limit <= 0 {
		return nil
	}

	keep := make([]bool, len(messages))
	anchors := 0

	for index := len(messages) - 1; index >= 0 && anchors < limit; index-- {
		message := messages[index]
		if message.Role == providertypes.RoleTool {
			continue
		}

		if message.Role == providertypes.RoleAssistant && len(message.ToolCalls) > 0 {
			span := matchedToolCallSpan(messages, index)
			for _, spanIndex := range span {
				keep[spanIndex] = true
			}
			if len(span) > 0 {
				anchors++
			}
			continue
		}

		keep[index] = true
		anchors++
	}

	selected := make([]providertypes.Message, 0, limit)
	for index, message := range messages {
		if !keep[index] {
			continue
		}
		selected = append(selected, message)
	}
	if len(selected) == 0 {
		return nil
	}

	return ProjectToolMessagesForModel(cloneContextMessages(selected))
}

// matchedToolCallSpan 返回 assistant tool call 与其完整 tool 响应组成的合法窗口下标集合。
func matchedToolCallSpan(messages []providertypes.Message, assistantIndex int) []int {
	if assistantIndex < 0 || assistantIndex >= len(messages) {
		return nil
	}

	message := messages[assistantIndex]
	if message.Role != providertypes.RoleAssistant || len(message.ToolCalls) == 0 {
		return nil
	}

	required := make(map[string]struct{}, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		callID := strings.TrimSpace(call.ID)
		if callID == "" {
			return nil
		}
		required[callID] = struct{}{}
	}
	if len(required) == 0 {
		return nil
	}

	matched := make(map[string]int, len(required))
	for index := assistantIndex + 1; index < len(messages); index++ {
		toolMessage := messages[index]
		if toolMessage.Role != providertypes.RoleTool {
			break
		}
		if !isInjectableToolMessage(toolMessage) {
			continue
		}

		callID := strings.TrimSpace(toolMessage.ToolCallID)
		if _, ok := required[callID]; !ok {
			continue
		}
		if _, exists := matched[callID]; exists {
			continue
		}
		matched[callID] = index
	}

	if len(matched) != len(required) {
		return nil
	}

	span := make([]int, 0, len(matched)+1)
	span = append(span, assistantIndex)
	for index := assistantIndex + 1; index < len(messages); index++ {
		toolMessage := messages[index]
		if toolMessage.Role != providertypes.RoleTool {
			break
		}
		callID := strings.TrimSpace(toolMessage.ToolCallID)
		if matchedIndex, ok := matched[callID]; ok && matchedIndex == index {
			span = append(span, index)
		}
	}
	return span
}

// isInjectableToolMessage 判断 tool 消息是否仍适合作为模型可见上下文继续注入。
func isInjectableToolMessage(message providertypes.Message) bool {
	if message.Role != providertypes.RoleTool {
		return false
	}
	content := strings.TrimSpace(message.Content)
	return content != "" && content != microCompactClearedMessage
}
