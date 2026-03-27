package interaction

import (
	"encoding/json"
	"fmt"
	"strings"

	"neo-code/internal/config"
)

type MessageKind string

const (
	MessageKindPlain         MessageKind = "plain"
	MessageKindResumeSummary MessageKind = "resume_summary"
	MessageKindToolStatus    MessageKind = "tool_status"
	MessageKindToolContext   MessageKind = "tool_context"
)

type SessionMessage struct {
	Role      string
	Content   string
	Kind      MessageKind
	Transient bool
}

type ToolExecutionPlan struct {
	Call          ToolCall
	StatusContext SessionMessage
	TouchedPaths  []string
}

type ToolApprovalRequest struct {
	Call             ToolCall
	ToolType         string
	Target           string
	AssistantMessage string
}

type ToolCompletionPlan struct {
	PendingApproval      *ToolApprovalRequest
	SystemContextMessage SessionMessage
	StatusMessage        string
	TouchedPaths         []string
	ContinueConversation bool
}

func ResumeSummaryMessage(summary string) SessionMessage {
	return SessionMessage{
		Role:    "system",
		Content: strings.TrimSpace(summary),
		Kind:    MessageKindResumeSummary,
	}
}

func BuildRequestMessages(messages []SessionMessage) []Message {
	filtered := make([]Message, 0, len(messages))
	keepToolContextIndex := RecentToolContextIndexes(messages, maxToolContextMessages())

	for idx, msg := range messages {
		if msg.Transient {
			continue
		}
		if msg.Kind == MessageKindResumeSummary || msg.Kind == MessageKindToolStatus {
			continue
		}
		if msg.Kind == MessageKindToolContext {
			if _, ok := keepToolContextIndex[idx]; !ok {
				continue
			}
		}
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		filtered = append(filtered, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return trimRequestMessages(filtered, shortTermTurns())
}

func FirstToolExecutionPlan(assistantContent string) *ToolExecutionPlan {
	calls := ParseAssistantToolCalls(assistantContent)
	if len(calls) == 0 {
		return nil
	}
	plan := PlanToolExecution(calls[0])
	return &plan
}

func PlanToolExecution(call ToolCall) ToolExecutionPlan {
	return ToolExecutionPlan{
		Call:          call,
		StatusContext: FormatToolStatusMessage(call.Tool, call.Params),
		TouchedPaths:  ToolPathsFromCall(call),
	}
}

func PlanToolResult(call ToolCall, result *ToolResult) ToolCompletionPlan {
	if toolType, target, ok := IsSecurityAskResult(result); ok {
		return ToolCompletionPlan{
			PendingApproval: &ToolApprovalRequest{
				Call:             call,
				ToolType:         toolType,
				Target:           target,
				AssistantMessage: FormatPendingApprovalMessage(call, target),
			},
			StatusMessage: "等待审批",
		}
	}

	touched := append([]string{}, ToolPathsFromCall(call)...)
	touched = append(touched, ToolPathsFromResult(result)...)
	return ToolCompletionPlan{
		SystemContextMessage: FormatToolContextMessage(result),
		StatusMessage:        "Generating...",
		TouchedPaths:         touched,
		ContinueConversation: true,
	}
}

func PlanToolError(err error) ToolCompletionPlan {
	return ToolCompletionPlan{
		SystemContextMessage: FormatToolErrorContext(err),
		StatusMessage:        "Generating...",
		ContinueConversation: true,
	}
}

func RecentToolContextIndexes(messages []SessionMessage, keep int) map[int]struct{} {
	result := map[int]struct{}{}
	if keep <= 0 || len(messages) == 0 {
		return result
	}

	for i := len(messages) - 1; i >= 0 && len(result) < keep; i-- {
		if messages[i].Kind == MessageKindToolContext {
			result[i] = struct{}{}
		}
	}
	return result
}

func FormatToolStatusMessage(toolName string, params map[string]interface{}) SessionMessage {
	detailKey := ""
	detailValue := ""
	if filePath, ok := params["filePath"].(string); ok && strings.TrimSpace(filePath) != "" {
		detailKey = "file"
		detailValue = strings.TrimSpace(filePath)
	} else if path, ok := params["path"].(string); ok && strings.TrimSpace(path) != "" {
		detailKey = "path"
		detailValue = strings.TrimSpace(path)
	} else if workdir, ok := params["workdir"].(string); ok && strings.TrimSpace(workdir) != "" {
		detailKey = "workdir"
		detailValue = strings.TrimSpace(workdir)
	}

	var builder strings.Builder
	builder.WriteString("Tool status\n")
	builder.WriteString("tool=")
	builder.WriteString(strings.TrimSpace(toolName))
	if detailKey != "" {
		builder.WriteString("\n")
		builder.WriteString(detailKey)
		builder.WriteString("=")
		builder.WriteString(detailValue)
	}

	return SessionMessage{
		Role:    "system",
		Content: builder.String(),
		Kind:    MessageKindToolStatus,
	}
}

func IsSecurityAskResult(result *ToolResult) (string, string, bool) {
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

func FormatPendingApprovalMessage(call ToolCall, target string) string {
	toolName := strings.TrimSpace(call.Tool)
	if toolName == "" {
		toolName = "unknown"
	}
	return fmt.Sprintf("工具 %s 需要安全审批。\nTarget: %s\n使用 /y 单次批准，或使用 /n 拒绝。", toolName, strings.TrimSpace(target))
}

func FormatToolContextMessage(result *ToolResult) SessionMessage {
	var builder strings.Builder
	builder.WriteString("Tool result\n")

	if result == nil {
		builder.WriteString("tool=unknown\nsuccess=false\nerror:\nTool returned empty result")
		return SessionMessage{
			Role:    "system",
			Content: builder.String(),
			Kind:    MessageKindToolContext,
		}
	}

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
			builder.WriteString(TruncateForContext(output, maxToolContextOutputSize()))
		}
	} else {
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = strings.TrimSpace(result.Output)
		}
		if errText != "" {
			builder.WriteString("error:\n")
			builder.WriteString(TruncateForContext(errText, maxToolContextOutputSize()))
		}
	}

	return SessionMessage{
		Role:    "system",
		Content: builder.String(),
		Kind:    MessageKindToolContext,
	}
}

func FormatToolErrorContext(err error) SessionMessage {
	errText := "Unknown error"
	if err != nil {
		errText = err.Error()
	}
	return SessionMessage{
		Role:    "system",
		Content: "Tool result\ntool=unknown\nsuccess=false\nerror:\n" + TruncateForContext(errText, maxToolContextOutputSize()),
		Kind:    MessageKindToolContext,
	}
}

func TruncateForContext(text string, maxLen int) string {
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

func ToolPathsFromCall(call ToolCall) []string {
	if call.Params == nil {
		return nil
	}
	paths := make([]string, 0, 2)
	for _, key := range []string{"filePath", "path", "workdir"} {
		if value, ok := call.Params[key].(string); ok && strings.TrimSpace(value) != "" {
			paths = append(paths, strings.TrimSpace(value))
		}
	}
	return paths
}

func ToolPathsFromResult(result *ToolResult) []string {
	if result == nil || result.Metadata == nil {
		return nil
	}
	paths := make([]string, 0, 2)
	for _, key := range []string{"filePath", "path"} {
		if value, ok := result.Metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			paths = append(paths, strings.TrimSpace(value))
		}
	}
	return paths
}

func MergeTouchedPaths(current []string, paths ...string) []string {
	merged := append([]string{}, current...)
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		exists := false
		for _, existing := range merged {
			if strings.EqualFold(existing, trimmed) {
				exists = true
				break
			}
		}
		if !exists {
			merged = append(merged, trimmed)
		}
	}
	return merged
}

func EstimateMessageTokens(messages []SessionMessage) int {
	total := 0
	for _, msg := range BuildRequestMessages(messages) {
		total += len([]rune(msg.Content))
	}
	if total == 0 {
		return 0
	}
	return total/4 + len(messages)*4
}

func trimRequestMessages(messages []Message, turns int) []Message {
	if turns <= 0 || len(messages) <= turns*2 {
		return messages
	}

	system := make([]Message, 0, len(messages))
	others := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			system = append(system, msg)
			continue
		}
		others = append(others, msg)
	}

	if len(others) > turns*2 {
		others = others[len(others)-turns*2:]
	}
	return append(system, others...)
}

func shortTermTurns() int {
	if cfg := config.GlobalAppConfig; cfg != nil && cfg.History.ShortTermTurns > 0 {
		return cfg.History.ShortTermTurns
	}
	return 6
}

func maxToolContextMessages() int {
	if cfg := config.GlobalAppConfig; cfg != nil && cfg.History.MaxToolContextMessages > 0 {
		return cfg.History.MaxToolContextMessages
	}
	return 3
}

func maxToolContextOutputSize() int {
	if cfg := config.GlobalAppConfig; cfg != nil && cfg.History.MaxToolContextOutputSize > 0 {
		return cfg.History.MaxToolContextOutputSize
	}
	return 4000
}
