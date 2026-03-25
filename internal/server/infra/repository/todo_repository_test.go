package repository

import (
	"context"
	"testing"

	"go-llm-demo/internal/server/domain"
)

func TestInMemoryTodoRepository_AddGeneratesIncrementingIDs(t *testing.T) {
	repo := NewInMemoryTodoRepository()

	t1, err := repo.Add(context.Background(), domain.Todo{Content: "task 1", Status: domain.TodoPending, Priority: domain.TodoPriorityHigh})
	if err != nil {
		t.Fatalf("failed to add todo: %v", err)
	}
	t2, err := repo.Add(context.Background(), domain.Todo{Content: "task 2", Status: domain.TodoPending, Priority: domain.TodoPriorityLow})
	if err != nil {
		t.Fatalf("failed to add todo: %v", err)
	}

	if t1.ID != "todo-1" {
		t.Fatalf("expected todo-1, got %q", t1.ID)
	}
	if t2.ID != "todo-2" {
		t.Fatalf("expected todo-2, got %q", t2.ID)
	}
}

func TestInMemoryTodoRepository_ClearResetsIDCounter(t *testing.T) {
	repo := NewInMemoryTodoRepository()
	_, _ = repo.Add(context.Background(), domain.Todo{Content: "task 1", Status: domain.TodoPending, Priority: domain.TodoPriorityHigh})

	if err := repo.Clear(context.Background()); err != nil {
		t.Fatalf("failed to clear todos: %v", err)
	}

	t1, err := repo.Add(context.Background(), domain.Todo{Content: "task 2", Status: domain.TodoPending, Priority: domain.TodoPriorityLow})
	if err != nil {
		t.Fatalf("failed to add todo: %v", err)
	}
	if t1.ID != "todo-1" {
		t.Fatalf("expected todo-1 after clear, got %q", t1.ID)
	}
}

func TestInMemoryTodoRepository_UpdateStatusUnknownReturnsError(t *testing.T) {
	repo := NewInMemoryTodoRepository()

	err := repo.UpdateStatus(context.Background(), "todo-404", domain.TodoCompleted)
	if err == nil {
		t.Fatal("expected error for unknown todo id")
	}
}

func TestInMemoryTodoRepository_UpdateStatusUpdatesExistingItem(t *testing.T) {
	repo := NewInMemoryTodoRepository()

	added, err := repo.Add(context.Background(), domain.Todo{Content: "task 1", Status: domain.TodoPending, Priority: domain.TodoPriorityHigh})
	if err != nil {
		t.Fatalf("failed to add todo: %v", err)
	}

	if err := repo.UpdateStatus(context.Background(), added.ID, domain.TodoCompleted); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	todos, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Status != domain.TodoCompleted {
		t.Fatalf("expected status %q, got %q", domain.TodoCompleted, todos[0].Status)
	}
}

func TestInMemoryTodoRepository_RemoveUnknownReturnsError(t *testing.T) {
	repo := NewInMemoryTodoRepository()

	err := repo.Remove(context.Background(), "todo-404")
	if err == nil {
		t.Fatal("expected error for unknown todo id")
	}
}

func TestInMemoryTodoRepository_RemoveDeletesExistingItem(t *testing.T) {
	repo := NewInMemoryTodoRepository()

	added, err := repo.Add(context.Background(), domain.Todo{Content: "task 1", Status: domain.TodoPending, Priority: domain.TodoPriorityHigh})
	if err != nil {
		t.Fatalf("failed to add todo: %v", err)
	}

	if err := repo.Remove(context.Background(), added.ID); err != nil {
		t.Fatalf("failed to remove todo: %v", err)
	}

	todos, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list todos: %v", err)
	}
	if len(todos) != 0 {
		t.Fatalf("expected 0 todos, got %d", len(todos))
	}
}

func TestInMemoryTodoRepository_ListReturnsAllItems(t *testing.T) {
	repo := NewInMemoryTodoRepository()
	_, _ = repo.Add(context.Background(), domain.Todo{Content: "task 1", Status: domain.TodoPending, Priority: domain.TodoPriorityHigh})
	_, _ = repo.Add(context.Background(), domain.Todo{Content: "task 2", Status: domain.TodoInProgress, Priority: domain.TodoPriorityMedium})

	todos, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list todos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}

	contents := map[string]bool{}
	for _, todo := range todos {
		contents[todo.Content] = true
	}
	if !contents["task 1"] || !contents["task 2"] {
		t.Fatalf("unexpected todos: %#v", todos)
	}
}
