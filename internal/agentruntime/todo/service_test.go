package todo

import (
	"context"
	"testing"
)

func setupTodoService() (TodoService, *InMemoryTodoRepository) {
	repo := NewInMemoryTodoRepository()
	svc := NewTodoService(repo)
	return svc, repo
}

func TestTodoServiceAddListUpdateRemoveAndClear(t *testing.T) {
	svc, _ := setupTodoService()

	first, err := svc.AddTodo(context.Background(), "task 1", TodoPriorityHigh)
	if err != nil {
		t.Fatalf("add todo: %v", err)
	}
	second, err := svc.AddTodo(context.Background(), "task 2", TodoPriorityLow)
	if err != nil {
		t.Fatalf("add second todo: %v", err)
	}

	todos, err := svc.ListTodos(context.Background())
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(todos) != 2 || todos[0].ID != first.ID || todos[1].ID != second.ID {
		t.Fatalf("unexpected todos: %+v", todos)
	}

	if err := svc.UpdateTodoStatus(context.Background(), first.ID, TodoCompleted); err != nil {
		t.Fatalf("update status: %v", err)
	}
	todos, _ = svc.ListTodos(context.Background())
	if todos[0].Status != TodoCompleted {
		t.Fatalf("expected completed status, got %+v", todos[0])
	}

	if err := svc.RemoveTodo(context.Background(), first.ID); err != nil {
		t.Fatalf("remove todo: %v", err)
	}
	todos, _ = svc.ListTodos(context.Background())
	if len(todos) != 1 || todos[0].ID != second.ID {
		t.Fatalf("unexpected todos after remove: %+v", todos)
	}

	if err := svc.ClearTodos(context.Background()); err != nil {
		t.Fatalf("clear todos: %v", err)
	}
	todos, _ = svc.ListTodos(context.Background())
	if len(todos) != 0 {
		t.Fatalf("expected empty todo list, got %+v", todos)
	}
}

func TestTodoServiceReturnsErrorsForUnknownIDs(t *testing.T) {
	svc, _ := setupTodoService()

	if err := svc.UpdateTodoStatus(context.Background(), "todo-404", TodoCompleted); err == nil {
		t.Fatal("expected update error for unknown todo")
	}
	if err := svc.RemoveTodo(context.Background(), "todo-404"); err == nil {
		t.Fatal("expected remove error for unknown todo")
	}
}
