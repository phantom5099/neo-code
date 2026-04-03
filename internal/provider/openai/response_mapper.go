package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	domain "neo-code/internal/provider"
)

func descriptorFromSDKModel(model sdk.Model) (domain.ModelDescriptor, bool) {
	rawJSON := strings.TrimSpace(model.RawJSON())
	if rawJSON == "" {
		return domain.ModelDescriptor{}, false
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return domain.ModelDescriptor{}, false
	}

	return domain.DescriptorFromRawModel(raw)
}

func mapResponseToChatResponse(resp responses.Response) (domain.ChatResponse, error) {
	message := domain.Message{
		Role:       domain.RoleAssistant,
		ResponseID: strings.TrimSpace(resp.ID),
	}

	for i, item := range resp.Output {
		switch item.Type {
		case "message":
			outputMessage := item.AsMessage()
			text, err := collectOutputMessageText(outputMessage)
			if err != nil {
				return domain.ChatResponse{}, fmt.Errorf("openai provider: map output message[%d]: %w", i, err)
			}
			message.Content += text

		case "function_call":
			call := item.AsFunctionCall()
			message.ToolCalls = append(message.ToolCalls, domain.ToolCall{
				ID:        strings.TrimSpace(call.CallID),
				Name:      strings.TrimSpace(call.Name),
				Arguments: call.Arguments,
			})
		}
	}

	return domain.ChatResponse{
		Message:      message,
		FinishReason: finishReasonForResponse(resp, message),
		Usage:        mapUsage(resp.Usage),
	}, nil
}

func collectOutputMessageText(message responses.ResponseOutputMessage) (string, error) {
	var builder strings.Builder
	for i, content := range message.Content {
		switch content.Type {
		case "output_text":
			builder.WriteString(content.AsOutputText().Text)
		case "refusal":
			builder.WriteString(content.AsRefusal().Refusal)
		case "":
			continue
		default:
			return "", fmt.Errorf("unsupported message content type %q at index %d", content.Type, i)
		}
	}
	return builder.String(), nil
}

func finishReasonForResponse(resp responses.Response, message domain.Message) string {
	if len(message.ToolCalls) > 0 {
		return "tool_calls"
	}
	if reason := strings.TrimSpace(resp.IncompleteDetails.Reason); reason != "" {
		return reason
	}
	return "stop"
}

func mapUsage(usage responses.ResponseUsage) domain.Usage {
	return domain.Usage{
		InputTokens:       int(usage.InputTokens),
		OutputTokens:      int(usage.OutputTokens),
		TotalTokens:       int(usage.TotalTokens),
		CachedInputTokens: int(usage.InputTokensDetails.CachedTokens),
		ReasoningTokens:   int(usage.OutputTokensDetails.ReasoningTokens),
	}
}
