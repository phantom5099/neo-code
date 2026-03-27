package todo

import (
	"context"
	"sort"
	"strconv"
	"strings"
)

type todoServiceImpl struct {
	repo TodoRepository
}

func NewTodoService(repo TodoRepository) TodoService {
	return &todoServiceImpl{repo: repo}
}

func (s *todoServiceImpl) AddTodo(ctx context.Context, content string, priority TodoPriority) (*Todo, error) {
	todo := Todo{
		Content:  content,
		Status:   TodoPending,
		Priority: priority,
	}
	return s.repo.Add(ctx, todo)
}

func (s *todoServiceImpl) UpdateTodoStatus(ctx context.Context, id string, status TodoStatus) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *todoServiceImpl) ListTodos(ctx context.Context) ([]Todo, error) {
	todos, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(todos, func(i, j int) bool {
		return todoIDNum(todos[i].ID) < todoIDNum(todos[j].ID)
	})
	return todos, nil
}

func (s *todoServiceImpl) ClearTodos(ctx context.Context) error {
	return s.repo.Clear(ctx)
}

func (s *todoServiceImpl) RemoveTodo(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}

func todoIDNum(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "todo-"))
	return n
}
