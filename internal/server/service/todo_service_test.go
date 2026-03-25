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

	todo, err := service.AddTodo(context.Background(), "测试任务1", domain.TodoPriorityHigh)
	if err != nil {
		t.Fatalf("添加任务失败: %v", err)
	}

	if todo.ID == "" {
		t.Fatal("任务 ID 不应为空")
	}
	if todo.Content != "测试任务1" {
		t.Errorf("期望内容为 '测试任务1', 得到 '%s'", todo.Content)
	}
	if todo.Status != domain.TodoPending {
		t.Errorf("期望状态为 'pending', 得到 '%s'", todo.Status)
	}
}

func TestTodoService_ListTodos(t *testing.T) {
	service, _ := setupTodoService()
	_, _ = service.AddTodo(context.Background(), "任务1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "任务2", domain.TodoPriorityLow)

	todos, err := service.ListTodos(context.Background())
	if err != nil {
		t.Fatalf("列出任务失败: %v", err)
	}

	if len(todos) != 2 {
		t.Fatalf("期望有 2 个任务, 得到 %d", len(todos))
	}
	if todos[0].Content != "任务1" || todos[1].Content != "任务2" {
		t.Error("任务排序或内容不正确")
	}
}

func TestTodoService_UpdateTodoStatus(t *testing.T) {
	service, _ := setupTodoService()
	todo, _ := service.AddTodo(context.Background(), "待办任务", domain.TodoPriorityMedium)

	err := service.UpdateTodoStatus(context.Background(), todo.ID, domain.TodoCompleted)
	if err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if todos[0].Status != domain.TodoCompleted {
		t.Errorf("期望状态为 'completed', 得到 '%s'", todos[0].Status)
	}
}

func TestTodoService_RemoveTodo(t *testing.T) {
	service, _ := setupTodoService()
	todo1, _ := service.AddTodo(context.Background(), "任务1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "任务2", domain.TodoPriorityLow)

	err := service.RemoveTodo(context.Background(), todo1.ID)
	if err != nil {
		t.Fatalf("移除任务失败: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if len(todos) != 1 {
		t.Fatalf("期望剩余 1 个任务, 得到 %d", len(todos))
	}
	if todos[0].Content != "任务2" {
		t.Error("移除的任务不正确")
	}
}

func TestTodoService_ClearTodos(t *testing.T) {
	service, _ := setupTodoService()
	_, _ = service.AddTodo(context.Background(), "任务1", domain.TodoPriorityHigh)
	_, _ = service.AddTodo(context.Background(), "任务2", domain.TodoPriorityLow)

	err := service.ClearTodos(context.Background())
	if err != nil {
		t.Fatalf("清空任务失败: %v", err)
	}

	todos, _ := service.ListTodos(context.Background())
	if len(todos) != 0 {
		t.Fatalf("期望任务清单为空, 得到 %d 个任务", len(todos))
	}
}
