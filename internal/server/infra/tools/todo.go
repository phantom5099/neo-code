package tools

import (
	"context"
	"fmt"
	"go-llm-demo/internal/server/domain"
	"strings"
)

// TodoTool 允许 Agent 管理显式任务清单
type TodoTool struct {
	todoSvc domain.TodoService
}

func NewTodoTool(todoSvc domain.TodoService) *TodoTool {
	return &TodoTool{todoSvc: todoSvc}
}

func (t *TodoTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "todo",
		Description: "管理显式任务清单。可以添加任务、更新任务状态（pending, in_progress, completed）、列出所有任务或移除任务。",
		Parameters: []ToolParamSpec{
			{Name: "action", Type: "string", Required: true, Description: "操作类型: add, update, list, remove, clear"},
			{Name: "content", Type: "string", Description: "任务内容（仅用于 add 操作）"},
			{Name: "priority", Type: "string", Description: "优先级: high, medium, low（仅用于 add 操作，默认为 medium）"},
			{Name: "id", Type: "string", Description: "任务 ID（用于 update 和 remove 操作）"},
			{Name: "status", Type: "string", Description: "新状态: pending, in_progress, completed（仅用于 update 操作）"},
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

	switch strings.ToLower(action) {
	case "add":
		content, errRes := requiredString(params, "content")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		priority := optionalStringDefault(params, "priority", "medium")
		todo, err := t.todoSvc.AddTodo(ctx, content, priority)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("已添加任务: %s (%s)", todo.ID, todo.Content)}

	case "update":
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
		status := domain.TodoStatus(strings.ToLower(statusStr))
		if status != domain.TodoPending && status != domain.TodoInProgress && status != domain.TodoCompleted {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: "无效的状态值"}
		}
		err := t.todoSvc.UpdateTodoStatus(ctx, id, status)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("任务 %s 状态已更新为 %s", id, status)}

	case "list":
		todos, err := t.todoSvc.ListTodos(ctx)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		if len(todos) == 0 {
			return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: "当前任务清单为空"}
		}
		var sb strings.Builder
		sb.WriteString("当前任务清单:\n")
		for _, todo := range todos {
			statusIcon := "[ ]"
			if todo.Status == domain.TodoInProgress {
				statusIcon = "[/]"
			} else if todo.Status == domain.TodoCompleted {
				statusIcon = "[x]"
			}
			sb.WriteString(fmt.Sprintf("%s %s: %s (优先级: %s)\n", statusIcon, todo.ID, todo.Content, todo.Priority))
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: sb.String()}

	case "remove":
		id, errRes := requiredString(params, "id")
		if errRes != nil {
			errRes.ToolName = t.Definition().Name
			return errRes
		}
		err := t.todoSvc.RemoveTodo(ctx, id)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: fmt.Sprintf("已移除任务 %s", id)}

	case "clear":
		err := t.todoSvc.ClearTodos(ctx)
		if err != nil {
			return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: err.Error()}
		}
		return &ToolResult{ToolName: t.Definition().Name, Success: true, Output: "任务清单已清空"}

	default:
		return &ToolResult{ToolName: t.Definition().Name, Success: false, Error: fmt.Sprintf("不支持的操作: %s", action)}
	}
}

func optionalStringDefault(params map[string]interface{}, key, fallback string) string {
	val, ok := params[key].(string)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
