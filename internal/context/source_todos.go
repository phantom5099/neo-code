package context

import (
	"context"
	"fmt"
	"sort"
	"strings"

	agentsession "neo-code/internal/session"
)

const (
	maxPromptTodos = 24
)

// todosSource 负责把会话 Todo 摘要渲染为 prompt section。
type todosSource struct{}

// Sections 渲染非终态 Todo，按状态与优先级排序后注入上下文。
func (todosSource) Sections(ctx context.Context, input BuildInput) ([]promptSection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(input.Todos) == 0 {
		return nil, nil
	}

	active := make([]agentsession.TodoItem, 0, len(input.Todos))
	for _, item := range input.Todos {
		if !item.Status.IsTerminal() {
			active = append(active, item.Clone())
		}
	}
	if len(active) == 0 {
		return nil, nil
	}

	sort.SliceStable(active, func(i, j int) bool {
		left := todoStatusRank(active[i].Status)
		right := todoStatusRank(active[j].Status)
		if left != right {
			return left < right
		}
		if active[i].Priority != active[j].Priority {
			return active[i].Priority > active[j].Priority
		}
		return active[i].CreatedAt.Before(active[j].CreatedAt)
	})
	if len(active) > maxPromptTodos {
		active = active[:maxPromptTodos]
	}

	lines := make([]string, 0, len(active)+1)
	for _, item := range active {
		line := fmt.Sprintf("- [%s] %s (p=%d, rev=%d): %s", item.Status, item.ID, item.Priority, item.Revision, item.Content)
		lines = append(lines, line)
		if len(item.Dependencies) > 0 {
			lines = append(lines, fmt.Sprintf("  deps: %s", strings.Join(item.Dependencies, ", ")))
		}
		if strings.TrimSpace(item.OwnerType) != "" || strings.TrimSpace(item.OwnerID) != "" {
			lines = append(lines, fmt.Sprintf("  owner: %s/%s", item.OwnerType, item.OwnerID))
		}
	}

	return []promptSection{
		{
			Title:   "Todo State",
			Content: strings.Join(lines, "\n"),
		},
	}, nil
}

// todoStatusRank 计算 Todo 状态排序优先级，值越小优先级越高。
func todoStatusRank(status agentsession.TodoStatus) int {
	switch status {
	case agentsession.TodoStatusInProgress:
		return 0
	case agentsession.TodoStatusBlocked:
		return 1
	case agentsession.TodoStatusPending:
		return 2
	default:
		return 3
	}
}
