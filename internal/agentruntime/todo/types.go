package todo

import (
	"context"
	"strings"
)

type TodoStatus string

const (
	TodoPending    TodoStatus = "pending"
	TodoInProgress TodoStatus = "in_progress"
	TodoCompleted  TodoStatus = "completed"
)

func ParseTodoStatus(input string) (TodoStatus, bool) {
	normalized := TodoStatus(strings.ToLower(strings.TrimSpace(input)))
	switch normalized {
	case TodoPending, TodoInProgress, TodoCompleted:
		return normalized, true
	default:
		return "", false
	}
}

type TodoPriority string

const (
	TodoPriorityHigh   TodoPriority = "high"
	TodoPriorityMedium TodoPriority = "medium"
	TodoPriorityLow    TodoPriority = "low"
)

func ParseTodoPriority(input string) (TodoPriority, bool) {
	normalized := TodoPriority(strings.ToLower(strings.TrimSpace(input)))
	switch normalized {
	case TodoPriorityHigh, TodoPriorityMedium, TodoPriorityLow:
		return normalized, true
	default:
		return "", false
	}
}

type TodoAction string

const (
	TodoActionAdd    TodoAction = "add"
	TodoActionUpdate TodoAction = "update"
	TodoActionList   TodoAction = "list"
	TodoActionRemove TodoAction = "remove"
	TodoActionClear  TodoAction = "clear"
)

func ParseTodoAction(input string) (TodoAction, bool) {
	normalized := TodoAction(strings.ToLower(strings.TrimSpace(input)))
	switch normalized {
	case TodoActionAdd, TodoActionUpdate, TodoActionList, TodoActionRemove, TodoActionClear:
		return normalized, true
	default:
		return "", false
	}
}

type Todo struct {
	ID       string       `json:"id"`
	Content  string       `json:"content"`
	Status   TodoStatus   `json:"status"`
	Priority TodoPriority `json:"priority"`
}

type TodoService interface {
	AddTodo(ctx context.Context, content string, priority TodoPriority) (*Todo, error)
	UpdateTodoStatus(ctx context.Context, id string, status TodoStatus) error
	ListTodos(ctx context.Context) ([]Todo, error)
	ClearTodos(ctx context.Context) error
	RemoveTodo(ctx context.Context, id string) error
}

type TodoRepository interface {
	Add(ctx context.Context, todo Todo) (*Todo, error)
	UpdateStatus(ctx context.Context, id string, status TodoStatus) error
	List(ctx context.Context) ([]Todo, error)
	Clear(ctx context.Context) error
	Remove(ctx context.Context, id string) error
}
