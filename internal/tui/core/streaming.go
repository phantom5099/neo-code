package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	toolStatusPrefix         = "[TOOL_STATUS]"
	toolContextPrefix        = "[TOOL_CONTEXT]"
	maxToolContextOutputSize = 4000
	maxToolContextMessages   = 3
)

func (m *Model) buildMessages() []services.Message {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	result := make([]services.Message, 0, len(m.chat.Messages))
	keepToolContextIndex := recentToolContextIndexes(m.chat.Messages, maxToolContextMessages)

	for idx, msg := range m.chat.Messages {
		if msg.Role == "system" && isResumeSummaryMessage(msg.Content) {
			continue
		}
		if msg.Role == "system" && isTransientToolStatusMessage(msg.Content) {
			continue
		}
		if msg.Role == "system" && isToolContextMessage(msg.Content) {
			if _, ok := keepToolContextIndex[idx]; !ok {
				continue
			}
		}
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		result = append(result, services.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return result
}

func (m *Model) streamResponse(messages []services.Message) tea.Cmd {
	stream, err := m.client.Chat(context.Background(), messages, m.chat.ActiveModel)
	if err != nil {
		return func() tea.Msg { return StreamErrorMsg{Err: err} }
	}

	m.streamChan = stream
	return func() tea.Msg {
		chunk, ok := <-stream
		if !ok {
			return StreamDoneMsg{}
		}
		return StreamChunkMsg{Content: chunk}
	}
}

func (m *Model) streamResponseFromChannel() tea.Cmd {
	if m.streamChan == nil {
		return nil
	}

	return func() tea.Msg {
		chunk, ok := <-m.streamChan
		if !ok {
			return StreamDoneMsg{}
		}
		return StreamChunkMsg{Content: chunk}
	}
}

func isTransientToolStatusMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), toolStatusPrefix)
}

func isToolContextMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), toolContextPrefix)
}

func recentToolContextIndexes(messages []state.Message, keep int) map[int]struct{} {
	result := map[int]struct{}{}
	if keep <= 0 || len(messages) == 0 {
		return result
	}

	for i := len(messages) - 1; i >= 0 && len(result) < keep; i-- {
		msg := messages[i]
		if msg.Role == "system" && isToolContextMessage(msg.Content) {
			result[i] = struct{}{}
		}
	}
	return result
}

func formatToolStatusMessage(toolName string, params map[string]interface{}) string {
	detail := ""
	if filePath, ok := params["filePath"].(string); ok && strings.TrimSpace(filePath) != "" {
		detail = " file=" + strings.TrimSpace(filePath)
	} else if path, ok := params["path"].(string); ok && strings.TrimSpace(path) != "" {
		detail = " path=" + strings.TrimSpace(path)
	} else if workdir, ok := params["workdir"].(string); ok && strings.TrimSpace(workdir) != "" {
		detail = " workdir=" + strings.TrimSpace(workdir)
	}
	return fmt.Sprintf("%s tool=%s%s", toolStatusPrefix, strings.TrimSpace(toolName), detail)
}

func isSecurityAskResult(result *services.ToolResult) (string, string, bool) {
	if result == nil || result.Success || result.Metadata == nil {
		return "", "", false
	}

	action, _ := result.Metadata["securityAction"].(string)
	if strings.TrimSpace(strings.ToLower(action)) != "ask" {
		return "", "", false
	}

	toolType, _ := result.Metadata["securityToolType"].(string)
	target, _ := result.Metadata["securityTarget"].(string)
	if strings.TrimSpace(toolType) == "" || strings.TrimSpace(target) == "" {
		return "", "", false
	}

	return strings.TrimSpace(toolType), strings.TrimSpace(target), true
}

func formatPendingApprovalMessage(pending *state.PendingApproval) string {
	if pending == nil {
		return "Security approval is required. Use /y to allow once or /n to reject."
	}

	toolName := strings.TrimSpace(pending.Call.Tool)
	if toolName == "" {
		toolName = "unknown"
	}
	return fmt.Sprintf("Security approval required for %s.\nTarget: %s\nUse /y to allow once, or /n to reject.", toolName, pending.Target)
}

func formatToolContextMessage(result *services.ToolResult) string {
	if result == nil {
		return toolContextPrefix + "\n" + "tool=unknown\n" + "success=false\n" + "error:\nTool returned empty result"
	}

	builder := strings.Builder{}
	builder.WriteString(toolContextPrefix)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("tool=%s\n", strings.TrimSpace(result.ToolName)))
	builder.WriteString(fmt.Sprintf("success=%t\n", result.Success))

	if len(result.Metadata) > 0 {
		if encoded, err := json.Marshal(result.Metadata); err == nil {
			builder.WriteString("metadata=")
			builder.WriteString(string(encoded))
			builder.WriteString("\n")
		}
	}

	if result.Success {
		output := strings.TrimSpace(result.Output)
		if output != "" {
			builder.WriteString("output:\n")
			builder.WriteString(truncateForContext(output, maxToolContextOutputSize))
		}
	} else {
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = strings.TrimSpace(result.Output)
		}
		if errText != "" {
			builder.WriteString("error:\n")
			builder.WriteString(truncateForContext(errText, maxToolContextOutputSize))
		}
	}

	return builder.String()
}

func formatToolErrorContext(err error) string {
	errText := "Unknown error"
	if err != nil {
		errText = err.Error()
	}
	return toolContextPrefix + "\n" + "tool=unknown\n" + "success=false\n" + "error:\n" + truncateForContext(errText, maxToolContextOutputSize)
}

func truncateForContext(text string, maxLen int) string {
	trimmed := strings.TrimSpace(text)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}

	suffix := fmt.Sprintf("\n... (truncated, total=%d chars)", len(trimmed))
	keep := maxLen - len(suffix)
	if keep < 0 {
		keep = 0
	}
	return trimmed[:keep] + suffix
}
