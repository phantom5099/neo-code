package runtime

import (
	"testing"

	"neo-code/internal/subagent"
)

type fakeSubAgentFactory struct{}

func (fakeSubAgentFactory) Create(role subagent.Role) (subagent.WorkerRuntime, error) {
	return subagent.NewWorkerFactory(nil).Create(role)
}

func TestServiceSubAgentFactoryRegistration(t *testing.T) {
	t.Parallel()

	svc := NewWithFactory(nil, nil, nil, nil, nil)
	if svc.SubAgentFactory() == nil {
		t.Fatalf("expected default sub-agent factory")
	}

	custom := fakeSubAgentFactory{}
	svc.SetSubAgentFactory(custom)
	if svc.SubAgentFactory() == nil {
		t.Fatalf("expected custom sub-agent factory")
	}

	svc.SetSubAgentFactory(nil)
	if svc.SubAgentFactory() == nil {
		t.Fatalf("expected reset to default sub-agent factory")
	}
}
