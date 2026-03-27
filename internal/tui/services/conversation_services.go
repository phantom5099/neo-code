package services

import "neo-code/internal/agentruntime/interaction"

type MessageKind = interaction.MessageKind

const (
	MessageKindPlain         = interaction.MessageKindPlain
	MessageKindResumeSummary = interaction.MessageKindResumeSummary
	MessageKindToolStatus    = interaction.MessageKindToolStatus
	MessageKindToolContext   = interaction.MessageKindToolContext
)

type SessionMessage = interaction.SessionMessage
type ToolExecutionPlan = interaction.ToolExecutionPlan
type ToolApprovalRequest = interaction.ToolApprovalRequest
type ToolCompletionPlan = interaction.ToolCompletionPlan

func ResumeSummaryMessage(summary string) SessionMessage {
	return interaction.ResumeSummaryMessage(summary)
}

func BuildRequestMessages(messages []SessionMessage) []Message {
	return interaction.BuildRequestMessages(messages)
}

func FirstToolExecutionPlan(assistantContent string) *ToolExecutionPlan {
	return interaction.FirstToolExecutionPlan(assistantContent)
}

func PlanToolExecution(call ToolCall) ToolExecutionPlan {
	return interaction.PlanToolExecution(call)
}

func PlanToolResult(call ToolCall, result *ToolResult) ToolCompletionPlan {
	return interaction.PlanToolResult(call, result)
}

func PlanToolError(err error) ToolCompletionPlan {
	return interaction.PlanToolError(err)
}

func RecentToolContextIndexes(messages []SessionMessage, keep int) map[int]struct{} {
	return interaction.RecentToolContextIndexes(messages, keep)
}

func FormatToolStatusMessage(toolName string, params map[string]interface{}) SessionMessage {
	return interaction.FormatToolStatusMessage(toolName, params)
}

func IsSecurityAskResult(result *ToolResult) (string, string, bool) {
	return interaction.IsSecurityAskResult(result)
}

func FormatPendingApprovalMessage(call ToolCall, target string) string {
	return interaction.FormatPendingApprovalMessage(call, target)
}

func FormatToolContextMessage(result *ToolResult) SessionMessage {
	return interaction.FormatToolContextMessage(result)
}

func FormatToolErrorContext(err error) SessionMessage {
	return interaction.FormatToolErrorContext(err)
}

func TruncateForContext(text string, maxLen int) string {
	return interaction.TruncateForContext(text, maxLen)
}

func ToolPathsFromCall(call ToolCall) []string {
	return interaction.ToolPathsFromCall(call)
}

func ToolPathsFromResult(result *ToolResult) []string {
	return interaction.ToolPathsFromResult(result)
}

func MergeTouchedPaths(current []string, paths ...string) []string {
	return interaction.MergeTouchedPaths(current, paths...)
}

func EstimateMessageTokens(messages []SessionMessage) int {
	return interaction.EstimateMessageTokens(messages)
}
