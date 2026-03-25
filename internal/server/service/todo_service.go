package service

import (
	"context"
	"go-llm-demo/internal/server/domain"
	"sort"
	"strconv"
	"strings"
)

type todoServiceImpl struct {
	repo domain.TodoRepository
}

func NewTodoService(repo domain.TodoRepository) domain.TodoService {
	return &todoServiceImpl{repo: repo}
}

func (s *todoServiceImpl) AddTodo(ctx context.Context, content string, priority domain.TodoPriority) (*domain.Todo, error) {
	todo := domain.Todo{
		Content:  content,
		Status:   domain.TodoPending,
		Priority: priority,
	}
	return s.repo.Add(ctx, todo)
}

func (s *todoServiceImpl) UpdateTodoStatus(ctx context.Context, id string, status domain.TodoStatus) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *todoServiceImpl) ListTodos(ctx context.Context) ([]domain.Todo, error) {
	todos, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	// 按 ID 排序以保证输出稳定性
	sort.Slice(todos, func(i, j int) bool {
		return todoIDNum(todos[i].ID) < todoIDNum(todos[j].ID)
	})
	return todos, nil
}

func (s *todoServiceImpl) ClearTodos(ctx context.Context) error {
	return s.repo.Clear(ctx)
}

func todoIDNum(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "todo-"))
	return n
}

func (s *todoServiceImpl) RemoveTodo(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}
