package runtime

import (
	"context"
	"testing"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/controlplane"
	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
)

func TestCollectCompletionStateKeepsUnverifiedWrites(t *testing.T) {
	t.Parallel()

	state := newRunState("run-verify-silent", newRuntimeSession("session-verify-silent"))
	state.completion = controlplane.CompletionState{
		HasUnverifiedWrites: true,
	}

	got := collectCompletionState(&state, providertypes.Message{Role: providertypes.RoleAssistant}, false)
	if got.HasUnverifiedWrites != true {
		t.Fatalf("expected unverified writes to remain blocked, got %+v", got)
	}
}

func TestApplyToolExecutionCompletionTracksWriteAndVerification(t *testing.T) {
	t.Parallel()

	written := applyToolExecutionCompletion(controlplane.CompletionState{}, toolExecutionSummary{
		HasSuccessfulWorkspaceWrite: true,
	})
	if !written.HasUnverifiedWrites {
		t.Fatalf("expected successful write to require verification, got %+v", written)
	}

	verified := applyToolExecutionCompletion(written, toolExecutionSummary{
		HasSuccessfulVerification: true,
	})
	if verified.HasUnverifiedWrites {
		t.Fatalf("expected explicit verification to clear pending write, got %+v", verified)
	}
}

func TestApplyToolExecutionCompletionDoesNotClearWhenVerifyPrecedesLatestWrite(t *testing.T) {
	t.Parallel()

	current := controlplane.CompletionState{HasUnverifiedWrites: true}
	next := applyToolExecutionCompletion(current, toolExecutionSummary{
		HasSuccessfulWorkspaceWrite: true,
		HasSuccessfulVerification:   true,
		LastSuccessfulWriteIndex:    1,
		LastSuccessfulVerifyIndex:   0,
	})
	if !next.HasUnverifiedWrites {
		t.Fatalf("expected unverified writes to remain when verify precedes latest write, got %+v", next)
	}
}

func TestApplyToolExecutionCompletionClearsWhenVerifyAfterLatestWrite(t *testing.T) {
	t.Parallel()

	current := controlplane.CompletionState{HasUnverifiedWrites: true}
	next := applyToolExecutionCompletion(current, toolExecutionSummary{
		HasSuccessfulWorkspaceWrite: true,
		HasSuccessfulVerification:   true,
		LastSuccessfulWriteIndex:    0,
		LastSuccessfulVerifyIndex:   1,
	})
	if next.HasUnverifiedWrites {
		t.Fatalf("expected verify after write to clear unverified writes, got %+v", next)
	}
}

func TestHasPendingAgentTodosBlocksOnAnyNonTerminalTodo(t *testing.T) {
	t.Parallel()

	todos := []agentsession.TodoItem{
		{
			ID:       "subagent-1",
			Content:  "delegate",
			Status:   agentsession.TodoStatusPending,
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}
	if !hasPendingAgentTodos(todos) {
		t.Fatalf("expected pending subagent todo to block completion")
	}

	completed := []agentsession.TodoItem{
		{
			ID:       "subagent-2",
			Content:  "done",
			Status:   agentsession.TodoStatusCompleted,
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}
	if hasPendingAgentTodos(completed) {
		t.Fatalf("expected terminal todo to not block completion")
	}
}

func TestTransitionRunPhaseInvalidTransitionReturnsError(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 4)}
	state := newRunState("run-invalid-phase", newRuntimeSession("session-invalid-phase"))
	state.lifecycle = controlplane.RunStatePlan

	err := service.transitionRunState(context.Background(), &state, controlplane.RunStateVerify)
	if err == nil {
		t.Fatalf("expected invalid transition to return error")
	}
	if state.lifecycle != controlplane.RunStatePlan {
		t.Fatalf("expected lifecycle to remain unchanged, got %q", state.lifecycle)
	}
	if events := collectRuntimeEvents(service.Events()); len(events) != 0 {
		t.Fatalf("expected no phase events on invalid transition, got %+v", events)
	}
}

func TestHasSuccessfulVerificationResultRequiresStructuredFacts(t *testing.T) {
	t.Parallel()

	if !hasSuccessfulVerificationResult([]tools.ToolResult{
		{Facts: tools.ToolExecutionFacts{VerificationPerformed: true, VerificationPassed: true}},
	}) {
		t.Fatalf("expected verification facts to count as verify passed")
	}
	if hasSuccessfulVerificationResult([]tools.ToolResult{
		{Facts: tools.ToolExecutionFacts{VerificationPerformed: true, VerificationPassed: false}},
		{Facts: tools.ToolExecutionFacts{VerificationPerformed: false, VerificationPassed: true}},
	}) {
		t.Fatalf("expected incomplete verification facts to be ignored")
	}
}
