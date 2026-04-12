package runtime

import (
	"context"
	"fmt"

	"neo-code/internal/provider"
	"neo-code/internal/provider/streaming"
	providertypes "neo-code/internal/provider/types"
)

// streamGenerateResult 统一承载一次流式生成的消息、用量与消费错误。
type streamGenerateResult struct {
	message      providertypes.Message
	inputTokens  int
	outputTokens int
	err          error
}

// generateStreamingMessage 负责执行一次基于流式事件的生成调用，并收敛最终 assistant 消息与 usage。
func generateStreamingMessage(
	ctx context.Context,
	modelProvider provider.Provider,
	req providertypes.GenerateRequest,
	hooks streaming.Hooks,
) streamGenerateResult {
	acc := streaming.NewAccumulator()
	streamEvents := make(chan providertypes.StreamEvent, 32)
	streamDone := make(chan streamGenerateResult, 1)

	go func() {
		outcome := streamGenerateResult{}
		defer func() {
			streamDone <- outcome
		}()

		for {
			select {
			case event, ok := <-streamEvents:
				if !ok {
					return
				}
				if err := streaming.HandleEvent(event, acc, streaming.Hooks{
					OnTextDelta:     hooks.OnTextDelta,
					OnToolCallStart: hooks.OnToolCallStart,
					OnMessageDone: func(payload providertypes.MessageDonePayload) {
						if payload.Usage != nil {
							outcome.inputTokens = payload.Usage.InputTokens
							outcome.outputTokens = payload.Usage.OutputTokens
						}
						if hooks.OnMessageDone != nil {
							hooks.OnMessageDone(payload)
						}
					},
				}); err != nil && outcome.err == nil {
					outcome.err = err
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	generateErr := modelProvider.Generate(ctx, req, streamEvents)
	close(streamEvents)
	outcome := <-streamDone
	if outcome.err != nil {
		if generateErr != nil {
			outcome.err = fmt.Errorf("runtime: provider stream handling failed after provider error: %v: %w", generateErr, outcome.err)
		}
		return outcome
	}
	if generateErr != nil {
		outcome.err = generateErr
		return outcome
	}
	if !acc.MessageDone() {
		outcome.err = fmt.Errorf("%w: provider stream ended without message_done event", provider.ErrStreamInterrupted)
		return outcome
	}

	message, err := acc.BuildMessage()
	if err != nil {
		outcome.err = err
		return outcome
	}
	outcome.message = message
	return outcome
}
