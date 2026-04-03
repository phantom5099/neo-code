package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"

	domain "neo-code/internal/provider"
)

func (p *Provider) consumeStream(
	ctx context.Context,
	stream *ssestream.Stream[responses.ResponseStreamEventUnion],
	events chan<- domain.StreamEvent,
) (domain.ChatResponse, error) {
	var finalResponse *responses.Response

	for stream.Next() {
		event := stream.Current()

		switch event.Type {
		case "response.output_text.delta":
			delta := event.AsResponseOutputTextDelta()
			if err := emitTextDelta(ctx, events, delta.Delta); err != nil {
				return domain.ChatResponse{}, err
			}

		case "response.output_item.added":
			added := event.AsResponseOutputItemAdded()
			if added.Item.Type != "function_call" {
				continue
			}
			call := added.Item.AsFunctionCall()
			if err := emitToolCallStart(
				ctx,
				events,
				int(added.OutputIndex),
				call.CallID,
				call.Name,
			); err != nil {
				return domain.ChatResponse{}, err
			}

		case "response.function_call_arguments.delta":
			delta := event.AsResponseFunctionCallArgumentsDelta()
			if err := emitToolCallDelta(ctx, events, int(delta.OutputIndex), delta.Delta); err != nil {
				return domain.ChatResponse{}, err
			}

		case "response.reasoning_summary_text.delta":
			delta := event.AsResponseReasoningSummaryTextDelta()
			if err := emitReasoningDelta(ctx, events, delta.Delta); err != nil {
				return domain.ChatResponse{}, err
			}

		case "response.completed":
			response := event.AsResponseCompleted().Response
			finalResponse = &response

		case "response.incomplete":
			response := event.AsResponseIncomplete().Response
			finalResponse = &response

		case "response.failed":
			response := event.AsResponseFailed().Response
			return domain.ChatResponse{}, mapResponseFailure(response)

		case "error":
			streamErr := event.AsError()
			return domain.ChatResponse{}, &domain.ProviderError{
				Code:      domain.ErrorCodeUnknown,
				Message:   strings.TrimSpace(streamErr.Message),
				Retryable: false,
			}
		}
	}

	if err := stream.Err(); err != nil {
		return domain.ChatResponse{}, mapProviderError(err)
	}
	if finalResponse == nil {
		return domain.ChatResponse{}, fmt.Errorf("openai provider: stream ended without final response")
	}

	resp, err := mapResponseToChatResponse(*finalResponse)
	if err != nil {
		return domain.ChatResponse{}, err
	}
	if err := emitMessageDone(ctx, events, resp.FinishReason, resp.Message.ResponseID, &resp.Usage); err != nil {
		return domain.ChatResponse{}, err
	}
	return resp, nil
}

func emitTextDelta(ctx context.Context, events chan<- domain.StreamEvent, text string) error {
	if events == nil || text == "" {
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
	if events == nil || strings.TrimSpace(name) == "" {
		return nil
	}

	select {
	case events <- domain.StreamEvent{
		Type:          domain.StreamEventToolCallStart,
		ToolCallID:    id,
		ToolName:      name,
		ToolCallIndex: index,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func emitToolCallDelta(ctx context.Context, events chan<- domain.StreamEvent, index int, argumentsDelta string) error {
	if events == nil || argumentsDelta == "" {
		return nil
	}

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

func emitReasoningDelta(ctx context.Context, events chan<- domain.StreamEvent, text string) error {
	if events == nil || text == "" {
		return nil
	}

	select {
	case events <- domain.StreamEvent{
		Type:          domain.StreamEventReasoningDelta,
		ReasoningText: text,
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
	responseID string,
	usage *domain.Usage,
) error {
	if events == nil {
		return nil
	}

	select {
	case events <- domain.StreamEvent{
		Type:         domain.StreamEventMessageDone,
		FinishReason: finishReason,
		ResponseID:   responseID,
		Usage:        usage,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
