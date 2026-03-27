package session

import (
	"context"
	"strings"
	"testing"
)

func TestWorkingMemoryServiceBuildsCheckpointFields(t *testing.T) {
	svc := NewWorkingMemoryService(NewWorkingMemoryStore(), 6, "D:/neo-code")
	messages := []Message{
		{Role: "user", Content: "Please update internal/agentruntime/memory/service.go and summarize the current task."},
		{Role: "assistant", Content: "Implemented the first pass. Next I will clean up the working memory summary."},
		{Role: "user", Content: "Also keep the recent file path in the working memory state."},
	}

	if err := svc.Refresh(context.Background(), messages); err != nil {
		t.Fatalf("refresh state: %v", err)
	}
	state, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if state.CurrentTask == "" || state.NextStep == "" {
		t.Fatalf("expected checkpoint fields to be populated, got %+v", state)
	}
	if len(state.RecentFiles) == 0 || state.RecentFiles[0] != "internal/agentruntime/memory/service.go" {
		t.Fatalf("expected recent files to be collected, got %+v", state.RecentFiles)
	}
}

func TestWorkingMemoryServiceFormatsExtendedContext(t *testing.T) {
	state := &WorkingMemoryState{
		CurrentTask:         "Refactor the working memory service",
		LastCompletedAction: "Added runtime package split",
		CurrentInProgress:   "Cleaning tests",
		NextStep:            "Run go test ./...",
	}

	got := formatWorkingMemoryContext(state, "D:/neo-code")
	for _, want := range []string{"Current task:", "Last completed action:", "Current in progress:", "Next step:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in formatted context, got %q", want, got)
		}
	}
}
