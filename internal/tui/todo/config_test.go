package todo

import "testing"

func TestTodoConfigBasics(t *testing.T) {
	if PageSize <= 0 {
		t.Fatalf("expected PageSize > 0, got %d", PageSize)
	}
	if APIPathList == "" || APIPathAdd == "" || APIPathUpdate == "" || APIPathRemove == "" {
		t.Fatalf("expected API paths to be non-empty")
	}
	if TitleText == "" || EmptyText == "" || HelpFooterText == "" {
		t.Fatalf("expected UI text to be non-empty")
	}
	if IconPending == "" || IconInProgress == "" || IconCompleted == "" {
		t.Fatalf("expected status icons to be non-empty")
	}

	if len(Keys.Up.Keys()) == 0 || len(Keys.Down.Keys()) == 0 || len(Keys.Add.Keys()) == 0 || len(Keys.Done.Keys()) == 0 || len(Keys.Delete.Keys()) == 0 || len(Keys.Back.Keys()) == 0 {
		t.Fatalf("expected key bindings to have at least one key")
	}
}
