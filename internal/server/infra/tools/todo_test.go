package tools

import (
	"context"
	"strings"
	"testing"

	"go-llm-demo/internal/server/domain"
	"go-llm-demo/internal/server/infra/repository"
	"go-llm-demo/internal/server/service"
)

func newTodoTool() (*TodoTool, domain.TodoService) {
	repo := repository.NewInMemoryTodoRepository()
	todoSvc := service.NewTodoService(repo)
	return NewTodoTool(todoSvc), todoSvc
}

func TestTodoTool_Run_RejectsUnsupportedAction(t *testing.T) {
	tool, _ := newTodoTool()

	res := tool.Run(map[string]interface{}{
		"action": "unknown",
	})

	if res == nil {
		t.Fatal("result should not be nil")
	}
	if res.Success {
		t.Fatal("expected failure for unsupported action")
	}
	if !strings.Contains(res.Error, "unsupported action") {
		t.Fatalf("unexpected error: %q", res.Error)
	}
}

func TestTodoTool_Run_AddMissingContentReturnsError(t *testing.T) {
	tool, _ := newTodoTool()

	res := tool.Run(map[string]interface{}{
		"action":   "add",
		"priority": "high",
	})

	if res == nil {
		t.Fatal("result should not be nil")
	}
	if res.Success {
		t.Fatal("expected failure for missing content")
	}
	if res.ToolName != "todo" {
		t.Fatalf("expected tool name 'todo', got %q", res.ToolName)
	}
}

func TestTodoTool_Run_AddInvalidPriorityReturnsError(t *testing.T) {
	tool, _ := newTodoTool()

	res := tool.Run(map[string]interface{}{
		"action":   "add",
		"content":  "task 1",
		"priority": "urgent",
	})

	if res == nil {
		t.Fatal("result should not be nil")
	}
	if res.Success {
		t.Fatal("expected failure for invalid priority")
	}
	if !strings.Contains(res.Error, "invalid priority") {
		t.Fatalf("unexpected error: %q", res.Error)
	}
}

func TestTodoTool_Run_AddListUpdateRemoveClear(t *testing.T) {
	tool, todoSvc := newTodoTool()

	addRes := tool.Run(map[string]interface{}{
		"action":   "add",
		"content":  "task 1",
		"priority": "high",
	})
	if addRes == nil || !addRes.Success {
		t.Fatalf("expected add success, got %#v", addRes)
	}
	if addRes.ToolName != "todo" {
		t.Fatalf("expected tool name 'todo', got %q", addRes.ToolName)
	}
	if !strings.Contains(addRes.Output, "Added task:") {
		t.Fatalf("unexpected add output: %q", addRes.Output)
	}

	addDefaultPriorityRes := tool.Run(map[string]interface{}{
		"action":  "add",
		"content": "task 2",
	})
	if addDefaultPriorityRes == nil || !addDefaultPriorityRes.Success {
		t.Fatalf("expected add success, got %#v", addDefaultPriorityRes)
	}

	todos, err := todoSvc.ListTodos(context.Background())
	if err != nil {
		t.Fatalf("failed to list todos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}
	idsByContent := map[string]string{}
	for _, todo := range todos {
		idsByContent[todo.Content] = todo.ID
	}
	task1ID := idsByContent["task 1"]
	task2ID := idsByContent["task 2"]
	if task1ID == "" || task2ID == "" {
		t.Fatalf("unexpected todos: %#v", todos)
	}

	updateRes := tool.Run(map[string]interface{}{
		"action": "update",
		"id":     task1ID,
		"status": "completed",
	})
	if updateRes == nil || !updateRes.Success {
		t.Fatalf("expected update success, got %#v", updateRes)
	}
	if !strings.Contains(updateRes.Output, "status updated") {
		t.Fatalf("unexpected update output: %q", updateRes.Output)
	}

	updateInProgressRes := tool.Run(map[string]interface{}{
		"action": "update",
		"id":     task2ID,
		"status": "in_progress",
	})
	if updateInProgressRes == nil || !updateInProgressRes.Success {
		t.Fatalf("expected update success, got %#v", updateInProgressRes)
	}

	listRes := tool.Run(map[string]interface{}{
		"action": "list",
	})
	if listRes == nil || !listRes.Success {
		t.Fatalf("expected list success, got %#v", listRes)
	}
	if !strings.Contains(listRes.Output, "Todo list:") {
		t.Fatalf("unexpected list output: %q", listRes.Output)
	}
	if !strings.Contains(listRes.Output, task1ID) || !strings.Contains(listRes.Output, task2ID) {
		t.Fatalf("expected list output to contain ids %q and %q, got %q", task1ID, task2ID, listRes.Output)
	}
	if !strings.Contains(listRes.Output, "[x]") {
		t.Fatalf("expected completed status icon in output, got %q", listRes.Output)
	}
	if !strings.Contains(listRes.Output, "[/]") {
		t.Fatalf("expected in_progress status icon in output, got %q", listRes.Output)
	}
	if !strings.Contains(listRes.Output, "priority: medium") {
		t.Fatalf("expected default priority medium in output, got %q", listRes.Output)
	}

	removeRes := tool.Run(map[string]interface{}{
		"action": "remove",
		"id":     task1ID,
	})
	if removeRes == nil || !removeRes.Success {
		t.Fatalf("expected remove success, got %#v", removeRes)
	}
	if !strings.Contains(removeRes.Output, "Removed task") {
		t.Fatalf("unexpected remove output: %q", removeRes.Output)
	}

	tool.Run(map[string]interface{}{"action": "add", "content": "task 2"})
	tool.Run(map[string]interface{}{"action": "add", "content": "task 3"})

	clearRes := tool.Run(map[string]interface{}{
		"action": "clear",
	})
	if clearRes == nil || !clearRes.Success {
		t.Fatalf("expected clear success, got %#v", clearRes)
	}
	if clearRes.Output != "Todo list cleared" {
		t.Fatalf("unexpected clear output: %q", clearRes.Output)
	}

	emptyRes := tool.Run(map[string]interface{}{
		"action": "list",
	})
	if emptyRes == nil || !emptyRes.Success {
		t.Fatalf("expected list success, got %#v", emptyRes)
	}
	if emptyRes.Output != "Todo list is empty" {
		t.Fatalf("unexpected empty output: %q", emptyRes.Output)
	}
}
