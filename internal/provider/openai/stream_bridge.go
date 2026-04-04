package openai

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	domain "neo-code/internal/provider"
)

// consumeStream reads SSE chunks from a Chat Completions streaming connection and
// emits domain.StreamEvents to the caller's channel.
//
// Chat Completions streaming event format (per chunk):
//   - choices[0].delta.content       → text_delta
//   - choices[0].delta.tool_calls[i] → tool_call_start (new index) / tool_call_delta (existing index)
//   - final chunk: choices[0].finish_reason + .usage → message_done
func (p *Provider) consumeStream(
	ctx context.Context,
	stream *ssestream.Stream[sdk.ChatCompletionChunk],
	events chan<- domain.StreamEvent,
) (domain.ChatResponse, error) {
	seenToolCallIndex := make(map[int]bool)

	var (
		finalChunk *sdk.ChatCompletionChunk
		textBuf    strings.Builder
		toolCalls  []partialToolCall
	)

	for stream.Next() {
		chunk := stream.Current()

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		// Text delta
		if delta.Content != "" {
			if err := emitTextDelta(ctx, events, delta.Content); err != nil {
				return domain.ChatResponse{}, err
			}
			textBuf.WriteString(delta.Content)
		}

		// Tool call deltas — each chunk may contain incremental data for multiple indices
		for _, tc := range delta.ToolCalls {
			idx := int(tc.Index)
			if !seenToolCallIndex[idx] {
				seenToolCallIndex[idx] = true
				// New tool_call → emit start event
				if err := emitToolCallStart(ctx, events, idx, tc.ID, tc.Function.Name); err != nil {
					return domain.ChatResponse{}, err
				}
				toolCalls = append(toolCalls, partialToolCall{
					Index: idx,
					ID:    tc.ID,
					Name:  tc.Function.Name,
				})
			}

			// Arguments delta (may arrive across multiple chunks for the same index)
			if tc.Function.Arguments != "" {
				if err := emitToolCallDelta(ctx, events, idx, tc.Function.Arguments); err != nil {
					return domain.ChatResponse{}, err
				}
				// Accumulate arguments for the final response assembly
				for i := range toolCalls {
					if toolCalls[i].Index == idx {
						toolCalls[i].Arguments += tc.Function.Arguments
						break
					}
				}
			}
		}

		// Capture the last non-empty chunk for its finish_reason / usage
		if string(chunk.Choices[0].FinishReason) != "" || chunk.JSON.Usage.Valid() {
			copied := chunk
			finalChunk = &copied
		}
	}

	if err := stream.Err(); err != nil {
		return domain.ChatResponse{}, mapProviderError(err)
	}

	if finalChunk == nil {
		return domain.ChatResponse{}, fmt.Errorf("openai provider: stream ended without final chunk")
	}

	// Assemble the final ChatResponse from accumulated state
	finishReason := string(finalChunk.Choices[0].FinishReason)
	if finishReason == "" || finishReason == "null" {
		finishReason = "stop"
	}
	if len(toolCalls) > 0 && finishReason != "tool_calls" {
		finishReason = "tool_calls"
	}

	response := domain.ChatResponse{
		Message: domain.Message{
			Role:    domain.RoleAssistant,
			Content: textBuf.String(),
		},
		FinishReason: finishReason,
	}
	if finalChunk.JSON.Usage.Valid() {
		response.Usage = mapUsageCC(finalChunk.Usage)
	}
	for _, tc := range toolCalls {
		response.Message.ToolCalls = append(response.Message.ToolCalls, domain.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		})
	}

	if err := emitMessageDone(ctx, events, finishReason, &response.Usage); err != nil {
		return domain.ChatResponse{}, err
	}
	return response, nil
}

type partialToolCall struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

func emitTextDelta(ctx context.Context, events chan<- domain.StreamEvent, text string) error {
	if text == "" {
		return nil
	}
	select {
	case events <- domain.StreamEvent{
		Type: domain.StreamEventTextDelta,
		Text: text,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func emitToolCallStart(ctx context.Context, events chan<- domain.StreamEvent, index int, id, name string) error {
	if name == "" {
		return nil
	}
	select {
	case events <- domain.StreamEvent{
		Type:          domain.StreamEventToolCallStart,
		ToolCallIndex: index,
		ToolCallID:    id,
		ToolName:      name,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func emitToolCallDelta(ctx context.Context, events chan<- domain.StreamEvent, index int, argumentsDelta string) error {
	select {
	case events <- domain.StreamEvent{
		Type:               domain.StreamEventToolCallDelta,
		ToolCallIndex:      index,
		ToolArgumentsDelta: argumentsDelta,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func emitMessageDone(
	ctx context.Context,
	events chan<- domain.StreamEvent,
	finishReason string,
	usage *domain.Usage,
) error {
	select {
	case events <- domain.StreamEvent{
		Type:         domain.StreamEventMessageDone,
		FinishReason: finishReason,
		Usage:        usage,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
