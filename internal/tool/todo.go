package tool

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/agentruntime/todo"
)

// TodoTool
type TodoTool struct {
	todoSvc todo.TodoService
}

func NewTodoTool(todoSvc todo.TodoService) *TodoTool {
	return &TodoTool{todoSvc: todoSvc}
}

func (t *TodoTool) Definition() ToolDefinition {
	return ToolDefinition{
		Category:    "runtime",
		Name:        "todo",
		Description: "Manage an explicit todo list: add tasks, update status, list items, remove items, or clear the list.",
		Parameters: []ToolParamSpec{
			{Name: "action", Type: "string", Required: true, Description: "Action type: add, update, list, remove, clear."},
			{Name: "content", Type: "string", Description: "Task content, used by add."},
			{Name: "priority", Type: "string", Description: "Task priority: high, medium, low. Used by add, default is medium."},
			{Name: "id", Type: "string", Description: "Task id, used by update and remove."},
			{Name: "status", Type: "string", Description: "Task status: pending, in_progress, completed. Used by update."},
		},
	}
}

func (t *TodoTool) Run(params map[string]interface{}) *ToolResult {
	action, errRes := requiredString(params, "action")
	if errRes != nil {
		errRes.ToolName = t.Definition().Name
		return errRes
	}

	ctx := context.Background()
	actionType, ok := todo.ParseTodoAction(strings.ToLower(action))
	if !ok {
		return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: fmt.Sprintf("unsupported action: %s", action)}
	}

	switch actionType {
	case todo.TodoActionAdd:
		content, errRes := requiredString(params, "content")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		priorityStr := strings.ToLower(optionalStringDefault(params, "priority", string(todo.TodoPriorityMedium)))
		priority, ok := todo.ParseTodoPriority(priorityStr)
		if !ok {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: "invalid priority value"}
		}
		created, err := t.todoSvc.AddTodo(ctx, content, priority)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("Added task: %s (%s)", created.ID, created.Content)}

	case todo.TodoActionUpdate:
		id, errRes := requiredString(params, "id")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		statusStr, errRes := requiredString(params, "status")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		status, ok := todo.ParseTodoStatus(strings.ToLower(statusStr))
		if !ok {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: "invalid status value"}
		}
		if err := t.todoSvc.UpdateTodoStatus(ctx, id, status); err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("Task %s status updated to %s", id, status)}

	case todo.TodoActionList:
		items, err := t.todoSvc.ListTodos(ctx)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		if len(items) == 0 {
			return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: "Todo list is empty"}
		}
		var sb strings.Builder
		sb.WriteString("Todo list:\n")
		for _, item := range items {
			statusIcon := "[ ]"
			if item.Status == todo.TodoInProgress {
				statusIcon = "[/]"
			} else if item.Status == todo.TodoCompleted {
				statusIcon = "[x]"
			}
			sb.WriteString(fmt.Sprintf("%s %s: %s (priority: %s)\n", statusIcon, item.ID, item.Content, item.Priority))
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: sb.String()}

	case todo.TodoActionRemove:
		id, errRes := requiredString(params, "id")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		if err := t.todoSvc.RemoveTodo(ctx, id); err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("Removed task %s", id)}

	case todo.TodoActionClear:
		if err := t.todoSvc.ClearTodos(ctx); err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: "Todo list cleared"}
	}

	return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: fmt.Sprintf("unsupported action: %s", action)}
}

func optionalStringDefault(params map[string]interface{}, key, fallback string) string {
	val, ok := params[key].(string)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
