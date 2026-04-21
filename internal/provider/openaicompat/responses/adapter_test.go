package responses

import (
	"context"
	"fmt"
	"strings"
	"testing"

	providertypes "neo-code/internal/provider/types"
)

func TestEmitFromStreamSupportsMultilineSSEData(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta",`,
		`data: "delta":"hello"}`,
		"",
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	events := make(chan providertypes.StreamEvent, 6)
	if err := EmitFromStream(context.Background(), strings.NewReader(body), events); err != nil {
		t.Fatalf("EmitFromStream() error = %v", err)
	}

	drained := drainResponseEvents(events)
	if len(drained) != 2 {
		t.Fatalf("expected 2 events, got %d (%+v)", len(drained), drained)
	}
	text, err := drained[0].TextDeltaValue()
	if err != nil || text.Text != "hello" {
		t.Fatalf("expected text delta hello, got err=%v event=%+v", err, drained[0])
	}
	done, err := drained[1].MessageDoneValue()
	if err != nil {
		t.Fatalf("expected message done event, got err=%v", err)
	}
	if done.Usage == nil || done.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage in done event: %+v", done.Usage)
	}
}

func TestEmitFromStreamSupportsLongDataLine(t *testing.T) {
	t.Parallel()

	largeDelta := strings.Repeat("a", 70*1024)
	body := strings.Join([]string{
		fmt.Sprintf(`data: {"type":"response.output_text.delta","delta":"%s"}`,
			largeDelta,
		),
		"",
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	events := make(chan providertypes.StreamEvent, 6)
	if err := EmitFromStream(context.Background(), strings.NewReader(body), events); err != nil {
		t.Fatalf("EmitFromStream() long line error = %v", err)
	}

	drained := drainResponseEvents(events)
	if len(drained) != 2 {
		t.Fatalf("expected 2 events, got %d", len(drained))
	}
	text, err := drained[0].TextDeltaValue()
	if err != nil {
		t.Fatalf("expected text delta event, got err=%v", err)
	}
	if len(text.Text) != len(largeDelta) {
		t.Fatalf("expected delta length %d, got %d", len(largeDelta), len(text.Text))
	}
}

func drainResponseEvents(events <-chan providertypes.StreamEvent) []providertypes.StreamEvent {
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
