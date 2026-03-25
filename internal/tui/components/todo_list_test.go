package components

import (
	"strings"
	"testing"

	"go-llm-demo/internal/tui/services"
	"go-llm-demo/internal/tui/todo"
)

func TestTodoListRenderEmptyShowsEmptyText(t *testing.T) {
	rendered := TodoList{Width: 80}.Render()
	if !strings.Contains(rendered, todo.EmptyText) {
		t.Fatalf("expected empty render to contain %q, got %q", todo.EmptyText, rendered)
	}
}

func TestTodoListRenderIncludesTitleItemsAndFooter(t *testing.T) {
	rendered := TodoList{
		Width:   80,
		Focused: true,
		Cursor:  0,
		Todos: []services.Todo{
			{ID: "todo-1", Content: "task 1", Status: services.TodoPending, Priority: services.TodoPriorityHigh},
			{ID: "todo-2", Content: "task 2", Status: services.TodoInProgress, Priority: services.TodoPriorityMedium},
			{ID: "todo-3", Content: "task 3", Status: services.TodoCompleted, Priority: services.TodoPriorityLow},
		},
	}.Render()

	for _, want := range []string{
		todo.TitleText,
		todo.HelpFooterText,
		"task 1",
		"task 2",
		"task 3",
		todo.IconPending,
		todo.IconInProgress,
		todo.IconCompleted,
		todo.PriorityHigh,
		todo.PriorityMedium,
		todo.PriorityLow,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected render to contain %q, got %q", want, rendered)
		}
	}
}
