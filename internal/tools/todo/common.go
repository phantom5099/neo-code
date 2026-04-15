package todo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
)

const (
	actionPlan      = "plan"
	actionAdd       = "add"
	actionUpdate    = "update"
	actionSetStatus = "set_status"
	actionRemove    = "remove"
	actionClaim     = "claim"
	actionComplete  = "complete"
	actionFail      = "fail"
)

const (
	reasonInvalidAction       = "invalid_action"
	reasonInvalidArguments    = "invalid_arguments"
	reasonTodoNotFound        = "todo_not_found"
	reasonInvalidTransition   = "invalid_transition"
	reasonDependencyViolation = "dependency_violation"
	reasonRevisionConflict    = "revision_conflict"
)

type writeInput struct {
	Action           string                  `json:"action"`
	Items            []agentsession.TodoItem `json:"items,omitempty"`
	Item             *agentsession.TodoItem  `json:"item,omitempty"`
	ID               string                  `json:"id,omitempty"`
	Patch            *todoPatchInput         `json:"patch,omitempty"`
	Status           agentsession.TodoStatus `json:"status,omitempty"`
	ExpectedRevision int64                   `json:"expected_revision,omitempty"`
	OwnerType        string                  `json:"owner_type,omitempty"`
	OwnerID          string                  `json:"owner_id,omitempty"`
	Artifacts        []string                `json:"artifacts,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
}

type todoPatchInput struct {
	Content       *string                  `json:"content,omitempty"`
	Status        *agentsession.TodoStatus `json:"status,omitempty"`
	Dependencies  *[]string                `json:"dependencies,omitempty"`
	Priority      *int                     `json:"priority,omitempty"`
	OwnerType     *string                  `json:"owner_type,omitempty"`
	OwnerID       *string                  `json:"owner_id,omitempty"`
	Acceptance    *[]string                `json:"acceptance,omitempty"`
	Artifacts     *[]string                `json:"artifacts,omitempty"`
	FailureReason *string                  `json:"failure_reason,omitempty"`
}

func (p *todoPatchInput) toSessionPatch() agentsession.TodoPatch {
	if p == nil {
		return agentsession.TodoPatch{}
	}
	return agentsession.TodoPatch{
		Content:       p.Content,
		Status:        p.Status,
		Dependencies:  p.Dependencies,
		Priority:      p.Priority,
		OwnerType:     p.OwnerType,
		OwnerID:       p.OwnerID,
		Acceptance:    p.Acceptance,
		Artifacts:     p.Artifacts,
		FailureReason: p.FailureReason,
	}
}

func parseInput(raw []byte) (writeInput, error) {
	var input writeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return writeInput{}, fmt.Errorf("todo_write: parse arguments: %w", err)
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	input.ID = strings.TrimSpace(input.ID)
	input.OwnerType = strings.TrimSpace(input.OwnerType)
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.Reason = strings.TrimSpace(input.Reason)
	return input, nil
}

func mapReason(err error) string {
	switch {
	case err == nil:
		return ""
	case strings.Contains(strings.ToLower(err.Error()), "unsupported action"):
		return reasonInvalidAction
	case strings.Contains(err.Error(), agentsession.ErrTodoNotFound.Error()):
		return reasonTodoNotFound
	case strings.Contains(err.Error(), agentsession.ErrInvalidTransition.Error()):
		return reasonInvalidTransition
	case strings.Contains(err.Error(), agentsession.ErrDependencyViolation.Error()):
		return reasonDependencyViolation
	case strings.Contains(err.Error(), agentsession.ErrRevisionConflict.Error()):
		return reasonRevisionConflict
	default:
		return tools.NormalizeErrorReason(tools.ToolNameTodoWrite, err)
	}
}

func errorResult(reason string, details string, extra map[string]any) tools.ToolResult {
	metadata := map[string]any{
		"reason_code": strings.TrimSpace(reason),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	result := tools.NewErrorResult(tools.ToolNameTodoWrite, strings.TrimSpace(reason), strings.TrimSpace(details), metadata)
	return tools.ApplyOutputLimit(result, tools.DefaultOutputLimitBytes)
}

func successResult(action string, items []agentsession.TodoItem) tools.ToolResult {
	content := renderTodos(action, items)
	result := tools.ToolResult{
		Name:    tools.ToolNameTodoWrite,
		Content: content,
		Metadata: map[string]any{
			"action":     strings.TrimSpace(action),
			"todo_count": len(items),
		},
	}
	return tools.ApplyOutputLimit(result, tools.DefaultOutputLimitBytes)
}

func renderTodos(action string, items []agentsession.TodoItem) string {
	lines := []string{
		"todo write result",
		"action: " + strings.TrimSpace(action),
		fmt.Sprintf("count: %d", len(items)),
	}
	if len(items) == 0 {
		return strings.Join(lines, "\n")
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		if items[i].Status != items[j].Status {
			return string(items[i].Status) < string(items[j].Status)
		}
		return items[i].ID < items[j].ID
	})

	lines = append(lines, "todos:")
	for _, item := range items {
		lines = append(lines,
			fmt.Sprintf("- [%s] %s (rev=%d, p=%d) %s", item.Status, item.ID, item.Revision, item.Priority, item.Content),
		)
	}
	return strings.Join(lines, "\n")
}
