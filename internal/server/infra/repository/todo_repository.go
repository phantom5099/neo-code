package repository

import (
	"context"
	"fmt"
	"go-llm-demo/internal/server/domain"
	"sync"
)

type InMemoryTodoRepository struct {
	todos  map[string]domain.Todo
	nextID int
	mu     sync.RWMutex
}

func NewInMemoryTodoRepository() *InMemoryTodoRepository {
	return &InMemoryTodoRepository{
		todos:  make(map[string]domain.Todo),
		nextID: 1,
	}
}

func (r *InMemoryTodoRepository) Add(ctx context.Context, todo domain.Todo) (*domain.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	todo.ID = fmt.Sprintf("todo-%d", r.nextID)
	r.nextID++
	r.todos[todo.ID] = todo
	return &todo, nil
}

func (r *InMemoryTodoRepository) UpdateStatus(ctx context.Context, id string, status domain.TodoStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	todo, ok := r.todos[id]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", id)
	}
	todo.Status = status
	r.todos[id] = todo
	return nil
}

func (r *InMemoryTodoRepository) List(ctx context.Context) ([]domain.Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]domain.Todo, 0, len(r.todos))
	for _, todo := range r.todos {
		list = append(list, todo)
	}
	return list, nil
}

func (r *InMemoryTodoRepository) Clear(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.todos = make(map[string]domain.Todo)
	r.nextID = 1
	return nil
}

func (r *InMemoryTodoRepository) Remove(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.todos[id]; !ok {
		return fmt.Errorf("任务 %s 不存在", id)
	}
	delete(r.todos, id)
	return nil
}
