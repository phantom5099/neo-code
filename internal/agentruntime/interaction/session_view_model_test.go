package interaction

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSessionViewModelSnapshotCapturesRuntimeState(t *testing.T) {
	model := NewSessionViewModel("D:/neo-code")
	model.Messages = []SessionViewMessage{{
		Role:      "assistant",
		Content:   "hello",
		Kind:      MessageKindPlain,
		Transient: true,
	}}
	model.ActiveModel = "gpt-5.4"
	model.APIKeyReady = true
	model.Generating = true
	model.PendingApproval = &SessionPendingApproval{
		Call:     ToolCall{Tool: "bash"},
		ToolType: "shell",
		Target:   "dir",
	}

	snapshot := model.Snapshot(true)

	if snapshot.ActiveModel != "gpt-5.4" || !snapshot.APIKeyReady || !snapshot.Generating || !snapshot.ApprovalRunning {
		t.Fatalf("unexpected snapshot header: %+v", snapshot)
	}
	if snapshot.PendingApproval == nil || snapshot.PendingApproval.ToolType != "shell" {
		t.Fatalf("expected pending approval in snapshot, got %+v", snapshot.PendingApproval)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "hello" || !snapshot.Messages[0].Transient {
		t.Fatalf("unexpected snapshot messages: %+v", snapshot.Messages)
	}
}

func TestSessionViewModelApplyMemoryFeedbackClearsSessionState(t *testing.T) {
	now := time.Unix(123, 0)
	model := NewSessionViewModel("D:/neo-code")
	model.Messages = []SessionViewMessage{{Role: "user", Content: "keep"}}
	model.TouchedFiles = []string{"README.md"}
	model.WorkspaceSummary = "summary"

	model.ApplyMemoryFeedback(&MemoryFeedback{
		Action:           MemoryActionClearSession,
		AssistantMessage: "session cleared",
		Stats:            &MemoryStats{TotalItems: 3},
	}, now)

	if len(model.Messages) != 1 || model.Messages[0].Content != "session cleared" || !model.Messages[0].Transient {
		t.Fatalf("expected reset session plus assistant notice, got %+v", model.Messages)
	}
	if len(model.TouchedFiles) != 0 || model.WorkspaceSummary != "" {
		t.Fatalf("expected session-only fields to be cleared, got files=%v summary=%q", model.TouchedFiles, model.WorkspaceSummary)
	}
	if model.MemoryStats.TotalItems != 3 {
		t.Fatalf("expected memory stats refresh, got %+v", model.MemoryStats)
	}
}

func TestSessionViewModelApplyTurnResolutionStartsStreamingAndTracksTouchedFiles(t *testing.T) {
	stream := make(chan string)
	model := NewSessionViewModel("D:/neo-code")

	gotStream := model.ApplyTurnResolution(TurnResolution{
		Messages: []SessionMessage{{Role: "system", Content: "tool", Kind: MessageKindToolContext}},
		TouchedPaths: []string{
			"README.md",
			"README.md",
			"cmd/main.go",
		},
		Stream: stream,
	}, time.Unix(123, 0))

	if gotStream != stream {
		t.Fatal("expected stream to be returned")
	}
	if !model.Generating {
		t.Fatal("expected generating state after stream start")
	}
	if len(model.Messages) != 2 || model.Messages[1].Role != "assistant" || !model.Messages[1].Streaming {
		t.Fatalf("expected tool context plus streaming assistant placeholder, got %+v", model.Messages)
	}
	if len(model.TouchedFiles) != 2 {
		t.Fatalf("expected unique touched files, got %+v", model.TouchedFiles)
	}
}

func TestSessionViewModelApplyStreamErrorReplacesPlaceholder(t *testing.T) {
	model := NewSessionViewModel("D:/neo-code")
	model.Messages = []SessionViewMessage{{Role: "assistant", Content: "", Streaming: true}}
	model.Generating = true

	model.ApplyStreamError(errors.New("boom"), time.Unix(123, 0))

	if model.Generating {
		t.Fatal("expected generating to stop on stream error")
	}
	if len(model.Messages) != 1 || !model.Messages[0].Error || model.Messages[0].Streaming {
		t.Fatalf("expected placeholder to turn into error, got %+v", model.Messages)
	}
	if !strings.Contains(model.Messages[0].Content, "boom") {
		t.Fatalf("expected error content to mention cause, got %q", model.Messages[0].Content)
	}
}
