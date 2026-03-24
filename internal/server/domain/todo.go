package domain

import "context"

// TodoStatus 表示任务状态
type TodoStatus string

const (
	TodoPending    TodoStatus = "pending"
	TodoInProgress TodoStatus = "in_progress"
	TodoCompleted  TodoStatus = "completed"
)

// Todo 表示任务清单中的一项
type Todo struct {
	ID       string     `json:"id"`
	Content  string     `json:"content"`
	Status   TodoStatus `json:"status"`
	Priority string     `json:"priority"` // high, medium, low
}

// TodoService 定义任务清单服务接口
type TodoService interface {
	// AddTodo 添加一个新任务
	AddTodo(ctx context.Context, content string, priority string) (*Todo, error)
	// UpdateTodoStatus 更新任务状态
	UpdateTodoStatus(ctx context.Context, id string, status TodoStatus) error
	// ListTodos 获取所有任务
	ListTodos(ctx context.Context) ([]Todo, error)
	// ClearTodos 清空所有任务
	ClearTodos(ctx context.Context) error
	// RemoveTodo 移除特定任务
	RemoveTodo(ctx context.Context, id string) error
}

// TodoRepository 定义任务清单存储接口
type TodoRepository interface {
	Add(ctx context.Context, todo Todo) (*Todo, error)
	UpdateStatus(ctx context.Context, id string, status TodoStatus) error
	List(ctx context.Context) ([]Todo, error)
	Clear(ctx context.Context) error
	Remove(ctx context.Context, id string) error
}
