package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dust/neo-code/internal/provider"
)

func TestJSONSessionStoreSaveLoadAndListSummaries(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := NewJSONSessionStore(baseDir)

	older := &Session{
		ID:        "session-old",
		Title:     "Old Session",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		Messages: []provider.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	newer := &Session{
		ID:        "session-new",
		Title:     "New Session",
		CreatedAt: time.Now().Add(-30 * time.Minute),
		UpdatedAt: time.Now(),
		Messages: []provider.Message{
			{Role: "user", Content: "new"},
		},
	}

	if err := store.Save(context.Background(), older); err != nil {
		t.Fatalf("Save older session: %v", err)
	}
	if err := store.Save(context.Background(), newer); err != nil {
		t.Fatalf("Save newer session: %v", err)
	}

	loaded, err := store.Load(context.Background(), older.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Title != older.Title {
		t.Fatalf("expected title %q, got %q", older.Title, loaded.Title)
	}
	if len(loaded.Messages) != 2 || loaded.Messages[1].Content != "world" {
		t.Fatalf("unexpected loaded messages: %+v", loaded.Messages)
	}

	mustWriteRuntimeFile(t, filepath.Join(baseDir, sessionsDirName, "invalid.json"), "{invalid")
	if err := os.MkdirAll(filepath.Join(baseDir, sessionsDirName, "directory"), 0o755); err != nil {
		t.Fatalf("mkdir stray directory: %v", err)
	}

	summaries, err := store.ListSummaries(context.Background())
	if err != nil {
		t.Fatalf("ListSummaries() error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].ID != newer.ID || summaries[1].ID != older.ID {
		t.Fatalf("expected summaries sorted by UpdatedAt desc, got %+v", summaries)
	}
}

func TestJSONSessionStoreErrors(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := NewJSONSessionStore(baseDir)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.Save(cancelledCtx, &Session{ID: "x"}); err == nil {
		t.Fatalf("expected cancelled save to fail")
	}
	if err := store.Save(context.Background(), nil); err == nil {
		t.Fatalf("expected nil session save to fail")
	}
	if _, err := store.Load(cancelledCtx, "missing"); err == nil {
		t.Fatalf("expected cancelled load to fail")
	}
	if _, err := store.ListSummaries(cancelledCtx); err == nil {
		t.Fatalf("expected cancelled list to fail")
	}
}

func mustWriteRuntimeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
