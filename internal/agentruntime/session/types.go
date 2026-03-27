package session

import (
	"context"
	"time"
)

type Message struct {
	Role    string
	Content string
}

type WorkingMemoryTurn struct {
	User      string
	Assistant string
}

type WorkingMemoryState struct {
	CurrentTask         string
	TaskSummary         string
	LastCompletedAction string
	CurrentInProgress   string
	NextStep            string
	RecentTurns         []WorkingMemoryTurn
	OpenQuestions       []string
	RecentFiles         []string
	UpdatedAt           time.Time
}

type WorkingMemoryRepository interface {
	Get(ctx context.Context) (*WorkingMemoryState, error)
	Save(ctx context.Context, state *WorkingMemoryState) error
	Clear(ctx context.Context) error
}

type WorkingMemoryService interface {
	BuildContext(ctx context.Context, messages []Message) (string, error)
	Refresh(ctx context.Context, messages []Message) error
	Clear(ctx context.Context) error
	Get(ctx context.Context) (*WorkingMemoryState, error)
}
