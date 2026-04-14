package subagent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerLifecycleCompleted(t *testing.T) {
	t.Parallel()

	policy, err := DefaultRolePolicy(RoleCoder)
	if err != nil {
		t.Fatalf("DefaultRolePolicy() error = %v", err)
	}

	w, err := NewWorker(RoleCoder, policy, EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
		return StepOutput{
			Delta: "patched files",
			Done:  true,
			Output: Output{
				Summary:     "done",
				Patches:     []string{"a.go"},
				NextActions: []string{"run tests"},
			},
		}, nil
	}))
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	err = w.Start(Task{ID: "t1", Goal: "fix bug"}, Budget{MaxSteps: 3}, Capability{
		AllowedTools: []string{"bash", "bash", " "},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	step, err := w.Step(context.Background())
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if !step.Done || step.State != StateSucceeded {
		t.Fatalf("unexpected step result: %+v", step)
	}

	result, err := w.Result()
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %q, want %q", result.StopReason, StopReasonCompleted)
	}
	if result.StepCount != 1 {
		t.Fatalf("step count = %d, want 1", result.StepCount)
	}
	if len(result.Capability.AllowedTools) != 1 {
		t.Fatalf("expected capability dedupe, got %+v", result.Capability)
	}
}

func TestWorkerLifecycleFailures(t *testing.T) {
	t.Parallel()

	policy, err := DefaultRolePolicy(RoleResearcher)
	if err != nil {
		t.Fatalf("DefaultRolePolicy() error = %v", err)
	}

	t.Run("engine error", func(t *testing.T) {
		t.Parallel()

		w, err := NewWorker(RoleResearcher, policy, EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			return StepOutput{}, errors.New("boom")
		}))
		if err != nil {
			t.Fatalf("NewWorker() error = %v", err)
		}
		if err := w.Start(Task{ID: "t2", Goal: "research"}, Budget{MaxSteps: 2}, Capability{}); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if _, err := w.Step(context.Background()); err == nil {
			t.Fatalf("expected step error")
		}
		result, err := w.Result()
		if err != nil {
			t.Fatalf("Result() error = %v", err)
		}
		if result.State != StateFailed || result.StopReason != StopReasonError {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("max steps", func(t *testing.T) {
		t.Parallel()

		w, err := NewWorker(RoleResearcher, policy, EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			return StepOutput{Delta: "not done", Done: false}, nil
		}))
		if err != nil {
			t.Fatalf("NewWorker() error = %v", err)
		}
		if err := w.Start(Task{ID: "t3", Goal: "research"}, Budget{MaxSteps: 1}, Capability{}); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		step, err := w.Step(context.Background())
		if err != nil {
			t.Fatalf("Step() error = %v", err)
		}
		if !step.Done || step.State != StateFailed {
			t.Fatalf("expected first step to finish by max steps, got %+v", step)
		}

		result, err := w.Result()
		if err != nil {
			t.Fatalf("Result() error = %v", err)
		}
		if result.StopReason != StopReasonMaxSteps {
			t.Fatalf("stop reason = %q, want %q", result.StopReason, StopReasonMaxSteps)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		w, err := NewWorker(RoleResearcher, policy, EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			return StepOutput{Done: false}, nil
		}))
		if err != nil {
			t.Fatalf("NewWorker() error = %v", err)
		}
		if err := w.Start(Task{ID: "t4", Goal: "research"}, Budget{MaxSteps: 5, Timeout: time.Nanosecond}, Capability{}); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		time.Sleep(2 * time.Millisecond)

		step, err := w.Step(context.Background())
		if err != nil {
			t.Fatalf("Step() error = %v", err)
		}
		if !step.Done || step.State != StateFailed {
			t.Fatalf("unexpected timeout step: %+v", step)
		}
		result, err := w.Result()
		if err != nil {
			t.Fatalf("Result() error = %v", err)
		}
		if result.StopReason != StopReasonTimeout {
			t.Fatalf("stop reason = %q, want %q", result.StopReason, StopReasonTimeout)
		}
	})
}

func TestWorkerStopAndGuards(t *testing.T) {
	t.Parallel()

	policy, err := DefaultRolePolicy(RoleReviewer)
	if err != nil {
		t.Fatalf("DefaultRolePolicy() error = %v", err)
	}

	w, err := NewWorker(RoleReviewer, policy, nil)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	if _, err := w.Result(); err == nil {
		t.Fatalf("expected result before finish to fail")
	}
	if _, err := w.Step(context.Background()); err == nil {
		t.Fatalf("expected step before start to fail")
	}
	if err := w.Start(Task{ID: "review", Goal: "review"}, Budget{}, Capability{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := w.Start(Task{ID: "review2", Goal: "review2"}, Budget{}, Capability{}); err == nil {
		t.Fatalf("expected double start to fail")
	}
	if err := w.Stop(StopReasonCanceled); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if w.State() != StateCanceled {
		t.Fatalf("state = %q, want %q", w.State(), StateCanceled)
	}
	if err := w.Stop(StopReasonCanceled); err != nil {
		t.Fatalf("terminal stop should be idempotent, got %v", err)
	}
}

func TestWorkerFactoryCreate(t *testing.T) {
	t.Parallel()

	factory := NewWorkerFactory(func(role Role, policy RolePolicy) Engine {
		return EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			return StepOutput{Done: true, Output: Output{Summary: "ok"}}, nil
		})
	})

	w, err := factory.Create(RoleCoder)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if w.Policy().Role != RoleCoder {
		t.Fatalf("policy role = %q, want %q", w.Policy().Role, RoleCoder)
	}

	if _, err := factory.Create(Role("invalid")); err == nil {
		t.Fatalf("expected invalid role create to fail")
	}
}

func TestWorkerRejectsInvalidOutputContract(t *testing.T) {
	t.Parallel()

	policy, err := DefaultRolePolicy(RoleCoder)
	if err != nil {
		t.Fatalf("DefaultRolePolicy() error = %v", err)
	}
	w, err := NewWorker(RoleCoder, policy, EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
		return StepOutput{
			Done: true,
			Output: Output{
				Summary: "   ",
			},
		}, nil
	}))
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	if err := w.Start(Task{ID: "t-invalid-output", Goal: "goal"}, Budget{MaxSteps: 3}, Capability{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if _, err := w.Step(context.Background()); err == nil {
		t.Fatalf("expected invalid output contract error")
	}
	result, err := w.Result()
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if result.State != StateFailed || result.StopReason != StopReasonError {
		t.Fatalf("unexpected result: %+v", result)
	}
}
