package todo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/tools"
)

// Tool 是会话级 Todo 读写工具实现。
type Tool struct{}

// New 返回 todo_write 工具实例。
func New() *Tool {
	return &Tool{}
}

// Name 返回工具唯一名称。
func (t *Tool) Name() string {
	return tools.ToolNameTodoWrite
}

// Description 返回工具描述。
func (t *Tool) Description() string {
	return "Write and manage session todos with status transitions, revisions, and dependencies."
}

// Schema 返回 todo_write 工具参数 schema。
func (t *Tool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					actionPlan,
					actionAdd,
					actionUpdate,
					actionSetStatus,
					actionRemove,
					actionClaim,
					actionComplete,
					actionFail,
				},
			},
			"items": map[string]any{
				"type": "array",
			},
			"item": map[string]any{
				"type": "object",
			},
			"id": map[string]any{
				"type": "string",
			},
			"patch": map[string]any{
				"type": "object",
			},
			"status": map[string]any{
				"type": "string",
			},
			"expected_revision": map[string]any{
				"type": "integer",
			},
			"owner_type": map[string]any{
				"type": "string",
			},
			"owner_id": map[string]any{
				"type": "string",
			},
			"artifacts": map[string]any{
				"type": "array",
			},
			"reason": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"action"},
	}
}

// MicroCompactPolicy 返回工具微压缩策略。
func (t *Tool) MicroCompactPolicy() tools.MicroCompactPolicy {
	return tools.MicroCompactPolicyCompact
}

// Execute 执行 todo_write 的 action 分发，并把变更写回会话。
func (t *Tool) Execute(ctx context.Context, call tools.ToolCallInput) (tools.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return errorResult(reasonInvalidArguments, err.Error(), nil), err
	}
	if call.SessionMutator == nil {
		err := errors.New("todo_write: session mutator is unavailable")
		return errorResult(reasonInvalidArguments, err.Error(), nil), err
	}

	input, err := parseInput(call.Arguments)
	if err != nil {
		return errorResult(reasonInvalidArguments, err.Error(), nil), err
	}

	resultErr := t.dispatch(call, input)
	if resultErr != nil {
		reason := mapReason(resultErr)
		return errorResult(reason, resultErr.Error(), map[string]any{"action": input.Action}), resultErr
	}

	return successResult(input.Action, call.SessionMutator.ListTodos()), nil
}

// dispatch 按 action 执行对应 Todo 变更。
func (t *Tool) dispatch(call tools.ToolCallInput, input writeInput) error {
	switch input.Action {
	case actionPlan:
		return call.SessionMutator.ReplaceTodos(input.Items)
	case actionAdd:
		if input.Item == nil {
			return fmt.Errorf("todo_write: action %q requires item", actionAdd)
		}
		return call.SessionMutator.AddTodo(*input.Item)
	case actionUpdate:
		if input.ID == "" || input.Patch == nil {
			return fmt.Errorf("todo_write: action %q requires id and patch", actionUpdate)
		}
		return call.SessionMutator.UpdateTodo(input.ID, input.Patch.toSessionPatch(), input.ExpectedRevision)
	case actionSetStatus:
		if input.ID == "" {
			return fmt.Errorf("todo_write: action %q requires id", actionSetStatus)
		}
		if !input.Status.Valid() {
			return fmt.Errorf("todo_write: action %q requires valid status", actionSetStatus)
		}
		return call.SessionMutator.SetTodoStatus(input.ID, input.Status, input.ExpectedRevision)
	case actionRemove:
		if input.ID == "" {
			return fmt.Errorf("todo_write: action %q requires id", actionRemove)
		}
		return call.SessionMutator.DeleteTodo(input.ID)
	case actionClaim:
		if input.ID == "" {
			return fmt.Errorf("todo_write: action %q requires id", actionClaim)
		}
		if strings.TrimSpace(input.OwnerType) == "" || strings.TrimSpace(input.OwnerID) == "" {
			return fmt.Errorf("todo_write: action %q requires owner_type and owner_id", actionClaim)
		}
		return call.SessionMutator.ClaimTodo(input.ID, input.OwnerType, input.OwnerID, input.ExpectedRevision)
	case actionComplete:
		if input.ID == "" {
			return fmt.Errorf("todo_write: action %q requires id", actionComplete)
		}
		return call.SessionMutator.CompleteTodo(input.ID, input.Artifacts, input.ExpectedRevision)
	case actionFail:
		if input.ID == "" {
			return fmt.Errorf("todo_write: action %q requires id", actionFail)
		}
		return call.SessionMutator.FailTodo(input.ID, input.Reason, input.ExpectedRevision)
	default:
		return fmt.Errorf("todo_write: unsupported action %q", input.Action)
	}
}
