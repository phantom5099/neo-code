package context

import (
	stdcontext "context"
	"fmt"
	"strings"
	"testing"
	"time"

	agentsession "neo-code/internal/session"
)

func TestTodosSourceSections(t *testing.T) {
	t.Parallel()

	now := time.Now()
	input := BuildInput{
		Todos: []agentsession.TodoItem{
			{
				ID:        "done",
				Content:   "done",
				Status:    agentsession.TodoStatusCompleted,
				Priority:  1,
				Revision:  2,
				CreatedAt: now.Add(-2 * time.Hour),
			},
			{
				ID:           "in-progress",
				Content:      "working",
				Status:       agentsession.TodoStatusInProgress,
				Priority:     5,
				Revision:     3,
				Dependencies: []string{"base"},
				CreatedAt:    now.Add(-time.Hour),
			},
			{
				ID:        "pending",
				Content:   "pending",
				Status:    agentsession.TodoStatusPending,
				Priority:  2,
				Revision:  1,
				CreatedAt: now,
			},
		},
	}

	sections, err := (todosSource{}).Sections(stdcontext.Background(), input)
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("Sections() len = %d, want 1", len(sections))
	}
	if sections[0].Title != "Todo State" {
		t.Fatalf("title = %q, want %q", sections[0].Title, "Todo State")
	}
	if strings.Contains(sections[0].Content, "done") {
		t.Fatalf("expected terminal todo filtered, got %q", sections[0].Content)
	}
	lines := strings.Split(sections[0].Content, "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "in-progress") {
		t.Fatalf("expected in_progress todo first, got %q", sections[0].Content)
	}
}

func TestTodosSourceSectionsBoundaries(t *testing.T) {
	t.Parallel()

	source := todosSource{}
	sections, err := source.Sections(stdcontext.Background(), BuildInput{})
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if sections != nil {
		t.Fatalf("Sections() = %+v, want nil", sections)
	}

	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	_, err = source.Sections(ctx, BuildInput{})
	if err != stdcontext.Canceled {
		t.Fatalf("Sections() err = %v, want context.Canceled", err)
	}
}

func TestTodosSourceSectionsAllTerminal(t *testing.T) {
	t.Parallel()

	input := BuildInput{
		Todos: []agentsession.TodoItem{
			{ID: "done", Content: "done", Status: agentsession.TodoStatusCompleted},
			{ID: "fail", Content: "fail", Status: agentsession.TodoStatusFailed},
			{ID: "cancel", Content: "cancel", Status: agentsession.TodoStatusCanceled},
		},
	}
	sections, err := (todosSource{}).Sections(stdcontext.Background(), input)
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if sections != nil {
		t.Fatalf("Sections() = %+v, want nil for all terminal todos", sections)
	}
}

func TestTodosSourceSectionsIncludesOwnerDepsAndLimit(t *testing.T) {
	t.Parallel()

	now := time.Now()
	todos := make([]agentsession.TodoItem, 0, maxPromptTodos+5)
	for i := 0; i < maxPromptTodos+5; i++ {
		todos = append(todos, agentsession.TodoItem{
			ID:        fmt.Sprintf("todo-%03d", i),
			Content:   "task",
			Status:    agentsession.TodoStatusPending,
			Priority:  i % 3,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
			Revision:  int64(i + 1),
		})
	}
	// 插入一个更高优先级的执行中任务，并带 deps/owner 分支。
	todos = append(todos, agentsession.TodoItem{
		ID:           "hot",
		Content:      "hot task",
		Status:       agentsession.TodoStatusInProgress,
		Priority:     99,
		CreatedAt:    now.Add(-time.Minute),
		Revision:     7,
		Dependencies: []string{"base-1", "base-2"},
		OwnerType:    "agent",
		OwnerID:      "worker-1",
	})

	sections, err := (todosSource{}).Sections(stdcontext.Background(), BuildInput{Todos: todos})
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("Sections() len = %d, want 1", len(sections))
	}

	lines := strings.Split(sections[0].Content, "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected rendered content: %q", sections[0].Content)
	}
	if !strings.Contains(lines[0], "hot") {
		t.Fatalf("expected highest rank todo first, got first line: %q", lines[0])
	}
	if !strings.Contains(sections[0].Content, "deps: base-1, base-2") {
		t.Fatalf("expected deps line in content: %q", sections[0].Content)
	}
	if !strings.Contains(sections[0].Content, "owner: agent/worker-1") {
		t.Fatalf("expected owner line in content: %q", sections[0].Content)
	}

	mainTodoLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			mainTodoLines++
		}
	}
	if mainTodoLines != maxPromptTodos {
		t.Fatalf("main todo lines = %d, want %d", mainTodoLines, maxPromptTodos)
	}
}

func TestTodoStatusRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status agentsession.TodoStatus
		want   int
	}{
		{status: agentsession.TodoStatusInProgress, want: 0},
		{status: agentsession.TodoStatusBlocked, want: 1},
		{status: agentsession.TodoStatusPending, want: 2},
		{status: agentsession.TodoStatusCompleted, want: 3},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			if got := todoStatusRank(tt.status); got != tt.want {
				t.Fatalf("todoStatusRank(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}
