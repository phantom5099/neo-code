package service

import (
	"context"
	"go-llm-demo/internal/server/domain"
	"go-llm-demo/internal/server/infra/repository"
	"testing"
)

func setupTodoService() (domain.TodoService, *repository.InMemoryTodoRepository) {
	repo := repository.NewInMemoryTodoRepository()
	service := NewTodoService(repo)
	return service, repo
}

func TestTodoService_AddTodo(t *testing.T) {
	service, _ := setupTodoService()

	todo, err := service.AddTodo(context.Background(), "test task 1", domain.TodoPriorityHigh)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	if todo.ID == "" {
		t.Fatal("task ID should not be empty")
	}
	if todo.Content != "test task 1" {
		t.Errorf("expected content 'test task 1', got '%s'", todo.Content)
	}
	if todo.Status != domain.TodoPending {
		t.Errorf("expected status 'pending', got '%s'", todo.Status)
	}
}

func TestTodoService_ListTodos(t *testing.T) {
	service, _ := setupTodoService()
	_, _ = service.AddTodo(context.Background(), "task 1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "task 2", domain.TodoPriorityLow)

	todos, err := service.ListTodos(context.Background())
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}

	if len(todos) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(todos))
	}
	if todos[0].Content != "task 1" || todos[1].Content != "task 2" {
		t.Error("task sorting or content incorrect")
	}
}

func TestTodoService_UpdateTodoStatus(t *testing.T) {
	service, _ := setupTodoService()
	todo, _ := service.AddTodo(context.Background(), "pending task", domain.TodoPriorityMedium)

	err := service.UpdateTodoStatus(context.Background(), todo.ID, domain.TodoCompleted)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if todos[0].Status != domain.TodoCompleted {
		t.Errorf("expected status 'completed', got '%s'", todos[0].Status)
	}
}

func TestTodoService_RemoveTodo(t *testing.T) {
	service, _ := setupTodoService()
	todo1, _ := service.AddTodo(context.Background(), "task 1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "task 2", domain.TodoPriorityLow)

	err := service.RemoveTodo(context.Background(), todo1.ID)
	if err != nil {
		t.Fatalf("failed to remove task: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if len(todos) != 1 {
		t.Fatalf("expected 1 task remaining, got %d", len(todos))
	}
	if todos[0].Content != "task 2" {
		t.Error("removed incorrect task")
	}
}

func TestTodoService_ClearTodos(t *testing.T) {
	service, _ := setupTodoService()
	_, _ = service.AddTodo(context.Background(), "task 1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "task 2", domain.TodoPriorityLow)

	err := service.ClearTodos(context.Background())
	if err != nil {
		t.Fatalf("failed to clear tasks: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if len(todos) != 0 {
		t.Fatalf("expected empty task list, got %d tasks", len(todos))
	}
}

func TestTodoService_UpdateTodoStatus_UnknownIDReturnsError(t *testing.T) {
	service, _ := setupTodoService()

	err := service.UpdateTodoStatus(context.Background(), "todo-404", domain.TodoCompleted)
	if err == nil {
		t.Fatal("expected error for unknown todo id")
	}
}

func TestTodoService_RemoveTodo_UnknownIDReturnsError(t *testing.T) {
	service, _ := setupTodoService()

	err := service.RemoveTodo(context.Background(), "todo-404")
	if err == nil {
		t.Fatal("expected error for unknown todo id")
	}
}

func TestTodoService_ListTodos_SortsByID(t *testing.T) {
	service, _ := setupTodoService()

	t1, _ := service.AddTodo(context.Background(), "task 1", domain.TodoPriorityHigh)
	t2, _ := service.AddTodo(context.Background(), "task 2", domain.TodoPriorityMedium)
	t3, _ := service.AddTodo(context.Background(), "task 3", domain.TodoPriorityLow)

	_ = service.RemoveTodo(context.Background(), t2.ID)
	_, _ = service.AddTodo(context.Background(), "task 4", domain.TodoPriorityLow)

	todos, err := service.ListTodos(context.Background())
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(todos) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(todos))
	}

	if todos[0].ID != t1.ID {
		t.Fatalf("expected first todo id %q, got %q", t1.ID, todos[0].ID)
	}
	if todos[1].ID != t3.ID {
		t.Fatalf("expected second todo id %q, got %q", t3.ID, todos[1].ID)
	}
}
