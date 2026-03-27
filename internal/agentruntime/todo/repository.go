package todo

import (
	"context"
	"fmt"
	"sync"
)

type InMemoryTodoRepository struct {
	todos  map[string]Todo
	nextID int
	mu     sync.RWMutex
}

func NewInMemoryTodoRepository() *InMemoryTodoRepository {
	return &InMemoryTodoRepository{
		todos:  make(map[string]Todo),
		nextID: 1,
	}
}

func (r *InMemoryTodoRepository) Add(ctx context.Context, todo Todo) (*Todo, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	todo.ID = fmt.Sprintf("todo-%d", r.nextID)
	r.nextID++
	r.todos[todo.ID] = todo
	return &todo, nil
}

func (r *InMemoryTodoRepository) UpdateStatus(ctx context.Context, id string, status TodoStatus) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	todo, ok := r.todos[id]
	if !ok {
		return fmt.Errorf("todo %s not found", id)
	}
	todo.Status = status
	r.todos[id] = todo
	return nil
}

func (r *InMemoryTodoRepository) List(ctx context.Context) ([]Todo, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Todo, 0, len(r.todos))
	for _, todo := range r.todos {
		list = append(list, todo)
	}
	return list, nil
}

func (r *InMemoryTodoRepository) Clear(ctx context.Context) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	r.todos = make(map[string]Todo)
	r.nextID = 1
	return nil
}

func (r *InMemoryTodoRepository) Remove(ctx context.Context, id string) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.todos[id]; !ok {
		return fmt.Errorf("todo %s not found", id)
	}
	delete(r.todos, id)
	return nil
}
