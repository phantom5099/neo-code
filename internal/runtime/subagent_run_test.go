package runtime

import (
	"context"
	"errors"
	"testing"

	"neo-code/internal/subagent"
)

type failingSubAgentFactory struct {
	err error
}

func (f failingSubAgentFactory) Create(role subagent.Role) (subagent.WorkerRuntime, error) {
	return nil, f.err
}

func TestServiceRunSubAgentTaskSuccess(t *testing.T) {
	t.Parallel()

	service := NewWithFactory(nil, nil, nil, nil, nil)
	service.SetSubAgentFactory(subagent.NewWorkerFactory(func(role subagent.Role, policy subagent.RolePolicy) subagent.Engine {
		return subagent.EngineFunc(func(ctx context.Context, input subagent.StepInput) (subagent.StepOutput, error) {
			if input.StepIndex == 1 {
				return subagent.StepOutput{
					Delta: "step-1",
					Done:  false,
				}, nil
			}
			return subagent.StepOutput{
				Delta: "step-2",
				Done:  true,
				Output: subagent.Output{
					Summary:     "task completed",
					Findings:    []string{"f1"},
					Patches:     []string{"p1"},
					Risks:       []string{"r1"},
					NextActions: []string{"n1"},
					Artifacts:   []string{"a1"},
				},
			}, nil
		})
	}))

	result, err := service.RunSubAgentTask(context.Background(), SubAgentTaskInput{
		RunID:     "sub-run-success",
		SessionID: "session-1",
		Role:      subagent.RoleCoder,
		Task: subagent.Task{
			ID:   "task-1",
			Goal: "implement feature",
		},
		Budget: subagent.Budget{
			MaxSteps: 3,
		},
	})
	if err != nil {
		t.Fatalf("RunSubAgentTask() error = %v", err)
	}
	if result.State != subagent.StateSucceeded {
		t.Fatalf("result state = %q, want %q", result.State, subagent.StateSucceeded)
	}
	if result.StepCount != 2 {
		t.Fatalf("result step count = %d, want 2", result.StepCount)
	}

	events := collectRuntimeEvents(service.Events())
	assertEventSequence(t, events, []EventType{
		EventSubAgentStarted,
		EventSubAgentProgress,
		EventSubAgentProgress,
		EventSubAgentCompleted,
	})
	assertEventsRunID(t, events, "sub-run-success")
}

func TestServiceRunSubAgentTaskFailureFlows(t *testing.T) {
	t.Parallel()

	t.Run("factory create failed", func(t *testing.T) {
		t.Parallel()

		service := NewWithFactory(nil, nil, nil, nil, nil)
		service.SetSubAgentFactory(failingSubAgentFactory{err: errors.New("create failed")})
		_, err := service.RunSubAgentTask(context.Background(), SubAgentTaskInput{
			RunID: "sub-run-factory-failed",
			Role:  subagent.RoleResearcher,
			Task: subagent.Task{
				ID:   "task-f",
				Goal: "research",
			},
		})
		if err == nil {
			t.Fatalf("expected create error")
		}
		events := collectRuntimeEvents(service.Events())
		assertEventSequence(t, events, []EventType{EventSubAgentFailed})
	})

	t.Run("worker step failed", func(t *testing.T) {
		t.Parallel()

		service := NewWithFactory(nil, nil, nil, nil, nil)
		service.SetSubAgentFactory(subagent.NewWorkerFactory(func(role subagent.Role, policy subagent.RolePolicy) subagent.Engine {
			return subagent.EngineFunc(func(ctx context.Context, input subagent.StepInput) (subagent.StepOutput, error) {
				return subagent.StepOutput{}, errors.New("step failed")
			})
		}))
		_, err := service.RunSubAgentTask(context.Background(), SubAgentTaskInput{
			RunID: "sub-run-step-failed",
			Role:  subagent.RoleReviewer,
			Task: subagent.Task{
				ID:   "task-step-f",
				Goal: "review",
			},
		})
		if err == nil {
			t.Fatalf("expected step error")
		}
		events := collectRuntimeEvents(service.Events())
		assertEventSequence(t, events, []EventType{
			EventSubAgentStarted,
			EventSubAgentProgress,
			EventSubAgentFailed,
		})
	})
}

func TestServiceRunSubAgentTaskInputValidation(t *testing.T) {
	t.Parallel()

	service := NewWithFactory(nil, nil, nil, nil, nil)
	if _, err := service.RunSubAgentTask(context.Background(), SubAgentTaskInput{
		Role: subagent.RoleCoder,
		Task: subagent.Task{
			ID:   "task",
			Goal: "goal",
		},
	}); err == nil {
		t.Fatalf("expected empty run id error")
	}

	if _, err := service.RunSubAgentTask(context.Background(), SubAgentTaskInput{
		RunID: "sub-run-invalid-role",
		Role:  subagent.Role("x"),
		Task: subagent.Task{
			ID:   "task",
			Goal: "goal",
		},
	}); err == nil {
		t.Fatalf("expected invalid role error")
	}
}
