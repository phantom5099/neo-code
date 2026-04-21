package chatcompletions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

func TestConsumeStreamSupportsWeakSSEFormat(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	events := make(chan providertypes.StreamEvent, 4)
	if err := ConsumeStream(context.Background(), strings.NewReader(body), events); err != nil {
		t.Fatalf("ConsumeStream() error = %v", err)
	}

	drained := drainEvents(events)
	if len(drained) != 2 {
		t.Fatalf("expected 2 events, got %d", len(drained))
	}
	text, err := drained[0].TextDeltaValue()
	if err != nil || text.Text != "ok" {
		t.Fatalf("expected text delta 'ok', got err=%v event=%+v", err, drained[0])
	}
	done, err := drained[1].MessageDoneValue()
	if err != nil {
		t.Fatalf("expected message done, got err=%v", err)
	}
	if done.Usage == nil || done.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage: %+v", done.Usage)
	}
}

func TestConsumeStreamParsesMultilineDataEvent(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_file"`,
		`data: ,"arguments":"{\"path\":\"README.md\"}"}}]},"finish_reason":"stop"}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	events := make(chan providertypes.StreamEvent, 8)
	if err := ConsumeStream(context.Background(), strings.NewReader(body), events); err != nil {
		t.Fatalf("ConsumeStream() error = %v", err)
	}

	drained := drainEvents(events)
	if len(drained) != 3 {
		t.Fatalf("expected 3 events, got %d (%+v)", len(drained), drained)
	}
	if _, err := drained[0].ToolCallStartValue(); err != nil {
		t.Fatalf("expected tool call start, got err=%v", err)
	}
	delta, err := drained[1].ToolCallDeltaValue()
	if err != nil || !strings.Contains(delta.ArgumentsDelta, "README.md") {
		t.Fatalf("expected tool call delta, got err=%v event=%+v", err, drained[1])
	}
}

func TestConsumeStreamEOFWithoutDoneAndWithoutFinishReason(t *testing.T) {
	t.Parallel()

	events := make(chan providertypes.StreamEvent, 4)
	err := ConsumeStream(context.Background(), strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"), events)
	if err == nil {
		t.Fatal("expected stream interruption error")
	}
	if !errors.Is(err, provider.ErrStreamInterrupted) {
		t.Fatalf("expected ErrStreamInterrupted, got %v", err)
	}
}

func drainEvents(events <-chan providertypes.StreamEvent) []providertypes.StreamEvent {
	out := make([]providertypes.StreamEvent, 0, len(events))
	for {
		select {
		case evt := <-events:
			out = append(out, evt)
		default:
			return out
		}
	}
}
