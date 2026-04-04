package openai

import (
	sdk "github.com/openai/openai-go/v3"

	domain "neo-code/internal/provider"
)

// mapResponseToChatResponse converts a Chat Completions response into our domain type.
func mapResponseToChatResponse(cc sdk.ChatCompletion) (domain.ChatResponse, error) {
	message := domain.Message{
		Role: domain.RoleAssistant,
	}

	if choice, ok := firstValidChoice(cc.Choices); ok {
		msg := choice.Message
		if msg.Content != "" {
			message.Content = msg.Content
		}
		for _, tc := range msg.ToolCalls {
			fn := tc.Function
			message.ToolCalls = append(message.ToolCalls, domain.ToolCall{
				ID:        tc.ID,
				Name:      fn.Name,
				Arguments: fn.Arguments,
			})
		}
	}

	finishReason := finishReasonForCC(cc)
	usage := mapUsageCC(cc.Usage)

	return domain.ChatResponse{
		Message:      message,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

// finishReasonForCC extracts the canonical finish reason from a Chat Completions response.
func finishReasonForCC(cc sdk.ChatCompletion) string {
	if choice, ok := firstValidChoice(cc.Choices); ok {
		reason := string(choice.FinishReason)
		if reason != "" && reason != "null" {
			return reason
		}
	}
	for _, choice := range cc.Choices {
		if len(choice.Message.ToolCalls) > 0 {
			return "tool_calls"
		}
	}
	return "stop"
}

// mapUsageCC converts SDK usage stats into our domain Usage struct.
// cc.Usage is a CompletionUsage struct (value type), not a pointer.
func mapUsageCC(usage sdk.CompletionUsage) domain.Usage {
	u := domain.Usage{
		InputTokens:  int(usage.PromptTokens),
		OutputTokens: int(usage.CompletionTokens),
		TotalTokens:  int(usage.TotalTokens),
	}
	if usage.PromptTokensDetails.CachedTokens > 0 {
		u.CachedInputTokens = int(usage.PromptTokensDetails.CachedTokens)
	}
	if usage.CompletionTokensDetails.ReasoningTokens > 0 {
		u.ReasoningTokens = int(usage.CompletionTokensDetails.ReasoningTokens)
	}
	return u
}

// firstValidChoice returns the first non-nil choice from the choices slice.
func firstValidChoice(choices []sdk.ChatCompletionChoice) (sdk.ChatCompletionChoice, bool) {
	for _, c := range choices {
		return c, true
	}
	return sdk.ChatCompletionChoice{}, false
}
