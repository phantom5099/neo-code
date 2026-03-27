package memory

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryServiceSavesAndRecallsPersistentMemory(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/memory.json"

	svc := NewMemoryService(
		NewFileMemoryStore(path, 100),
		NewSessionMemoryStore(100),
		5,
		2.2,
		1800,
		path,
		[]string{"user_preference", "project_rule", "code_fact", "fix_recipe"},
	)

	if err := svc.Save(ctx, "Always answer in Chinese and keep explanations concise.", "Understood."); err != nil {
		t.Fatalf("save preference: %v", err)
	}
	if err := svc.Save(ctx, "Where is the file memory_repository.go used?", "memory_repository.go stores persistent memory items on disk."); err != nil {
		t.Fatalf("save code fact: %v", err)
	}

	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.PersistentItems == 0 {
		t.Fatalf("expected persistent memory items, got %+v", stats)
	}

	prompt, err := svc.BuildContext(ctx, "Explain how memory_repository.go works and remember my preference.")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if !strings.Contains(prompt, "user_preference") {
		t.Fatalf("expected recalled user_preference in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "code_fact") {
		t.Fatalf("expected recalled code_fact in prompt, got %q", prompt)
	}
}

func TestMemoryServiceSkipsToolCallPayload(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/memory.json"

	svc := NewMemoryService(
		NewFileMemoryStore(path, 100),
		NewSessionMemoryStore(100),
		5,
		2.2,
		1800,
		path,
		[]string{"user_preference", "project_rule", "code_fact", "fix_recipe"},
	)

	reply := `{"tool":"read","params":{"filePath":"internal/agentruntime/memory/service.go"}}`
	if err := svc.Save(ctx, "Please inspect memory_service.go", reply); err != nil {
		t.Fatalf("save tool payload: %v", err)
	}

	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.TotalItems != 0 {
		t.Fatalf("expected tool payload to be skipped, got %+v", stats)
	}
}
