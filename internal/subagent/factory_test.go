package subagent

import (
	"context"
	"testing"
)

func TestApplyFactoryOptionsAndExecutionContext(t *testing.T) {
	t.Parallel()

	policy, err := DefaultRolePolicy(RoleCoder)
	if err != nil {
		t.Fatalf("DefaultRolePolicy() error = %v", err)
	}

	executor := noopToolExecutor{}
	var capturedExecutor ToolExecutor
	factory := NewWorkerFactory(func(role Role, policy RolePolicy) Engine {
		_ = role
		_ = policy
		return EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			_ = ctx
			capturedExecutor = input.Executor
			return StepOutput{
				Done: true,
				Output: Output{
					Summary:     "ok",
					Findings:    []string{"f1"},
					Patches:     []string{"p1"},
					Risks:       []string{"r1"},
					NextActions: []string{"n1"},
					Artifacts:   []string{"a1"},
				},
			}, nil
		})
	}, nil, WithExecutionContext(ExecutionContext{ToolExecutor: executor}))
	worker, err := factory.Create(RoleCoder)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := worker.Start(Task{ID: "task-factory-ctx", Goal: "goal"}, policy.DefaultBudget, Capability{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := worker.Step(context.Background()); err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if capturedExecutor == nil {
		t.Fatalf("expected execution context tool executor to be injected")
	}
}

func TestNewWorkerFactoryNilBuilderUsesDefaultEngine(t *testing.T) {
	t.Parallel()

	factory := NewWorkerFactory(nil)
	worker, err := factory.Create(RoleReviewer)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := worker.Start(Task{ID: "task-default-engine", Goal: "review"}, Budget{MaxSteps: 1}, Capability{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	step, err := worker.Step(context.Background())
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if !step.Done {
		t.Fatalf("default engine should finish in one step")
	}
}

func TestWithToolExecutorOptionOverridesExecutionContext(t *testing.T) {
	t.Parallel()

	var captured ToolExecutor
	factory := NewWorkerFactory(func(role Role, policy RolePolicy) Engine {
		_ = role
		_ = policy
		return EngineFunc(func(ctx context.Context, input StepInput) (StepOutput, error) {
			_ = ctx
			captured = input.Executor
			return StepOutput{
				Done: true,
				Output: Output{
					Summary:     "ok",
					Findings:    []string{"f"},
					Patches:     []string{"p"},
					Risks:       []string{"r"},
					NextActions: []string{"n"},
					Artifacts:   []string{"a"},
				},
			}, nil
		})
	}, WithExecutionContext(ExecutionContext{}), WithToolExecutor(noopToolExecutor{}))

	worker, err := factory.Create(RoleCoder)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := worker.Start(Task{ID: "task-tool-executor", Goal: "goal"}, Budget{MaxSteps: 1}, Capability{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := worker.Step(context.Background()); err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if captured == nil {
		t.Fatalf("expected executor to be injected")
	}
}

func TestFactoryOptionsIgnoreNilReceiver(t *testing.T) {
	t.Parallel()

	var nilFactory *WorkerFactory
	WithExecutionContext(ExecutionContext{ToolExecutor: noopToolExecutor{}})(nilFactory)
	WithToolExecutor(noopToolExecutor{})(nilFactory)
	applyFactoryOptions(nilFactory, nil, WithToolExecutor(noopToolExecutor{}))
}
