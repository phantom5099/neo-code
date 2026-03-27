package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkingMemoryStorePersistsStateToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "session_state.json")
	store := NewWorkingMemoryStore(path)

	state := &WorkingMemoryState{
		CurrentTask:         "Refactor runtime state persistence",
		LastCompletedAction: "Created working memory store",
		NextStep:            "Reload the state from disk",
		RecentFiles:         []string{"internal/agentruntime/memory/service.go"},
		UpdatedAt:           time.Now().UTC(),
	}
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	reloaded := NewWorkingMemoryStore(path)
	got, err := reloaded.Get(context.Background())
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if got.CurrentTask != state.CurrentTask || got.NextStep != state.NextStep {
		t.Fatalf("expected persisted state, got %+v", got)
	}
}

func TestWorkingMemoryStoreClearRemovesPersistedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "session_state.json")
	store := NewWorkingMemoryStore(path)
	if err := store.Save(context.Background(), &WorkingMemoryState{CurrentTask: "task"}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("clear state: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected persisted file to be removed, got %v", err)
	}
}
