package agentruntime

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/agentruntime/chat"
	"neo-code/internal/agentruntime/memory"
	"neo-code/internal/agentruntime/persona"
	"neo-code/internal/agentruntime/session"
	"neo-code/internal/agentruntime/todo"
	"neo-code/internal/config"
	"neo-code/internal/provider"
	"neo-code/internal/tool"
)

// ChatClient is the application-facing runtime interface used by the TUI.
type ChatClient interface {
	Chat(ctx context.Context, messages []chat.Message, model string) (<-chan string, error)
	GetMemoryStats(ctx context.Context) (*memory.MemoryStats, error)
	ClearMemory(ctx context.Context) error
	ClearSessionMemory(ctx context.Context) error
	DefaultModel() string
}

// WorkingSessionSummaryProvider exposes the persisted workspace summary.
type WorkingSessionSummaryProvider interface {
	GetWorkingSessionSummary(ctx context.Context) (string, error)
}

type localChatClient struct {
	promptSvc  persona.Service
	memorySvc  memory.MemoryService
	workingSvc session.WorkingMemoryService
	todoSvc    todo.TodoService
	config     *config.AppConfiguration
}

// NewLocalChatClient assembles the default local runtime stack.
func NewLocalChatClient() (ChatClient, error) {
	cfg := config.GlobalAppConfig
	if cfg == nil {
		return nil, fmt.Errorf("app config is not loaded")
	}

	storePath := strings.TrimSpace(cfg.Memory.StoragePath)
	if storePath == "" {
		storePath = config.DefaultMemoryStoragePath()
	}
	maxItems := cfg.Memory.MaxItems
	if maxItems <= 0 {
		maxItems = 1000
	}

	workspaceRoot := tool.GetWorkspaceRoot()
	persistentRepo := memory.NewFileMemoryStore(storePath, maxItems)
	sessionRepo := memory.NewSessionMemoryStore(maxItems)

	workingStatePath := ""
	if cfg.History.PersistSessionState {
		workingStatePath = session.BuildWorkspaceStatePath(cfg.History.WorkspaceStateDir, workspaceRoot)
	}
	workingRepo := session.NewWorkingMemoryStore(workingStatePath)

	memorySvc := memory.NewMemoryService(
		persistentRepo,
		sessionRepo,
		cfg.Memory.TopK,
		cfg.Memory.MinMatchScore,
		cfg.Memory.MaxPromptChars,
		storePath,
		cfg.Memory.PersistTypes,
	)
	workingSvc := session.NewWorkingMemoryService(workingRepo, cfg.History.ShortTermTurns, workspaceRoot)
	promptSvc := persona.NewFileService(strings.TrimSpace(cfg.Persona.FilePath))

	todoRepo := todo.NewInMemoryTodoRepository()
	todoSvc := todo.NewTodoService(todoRepo)
	tool.GlobalRegistry.Register(tool.NewTodoTool(todoSvc))

	return &localChatClient{
		promptSvc:  promptSvc,
		memorySvc:  memorySvc,
		workingSvc: workingSvc,
		todoSvc:    todoSvc,
		config:     cfg,
	}, nil
}

func (c *localChatClient) Chat(ctx context.Context, messages []chat.Message, model string) (<-chan string, error) {
	chatProvider, err := provider.NewChatProvider(model)
	if err != nil {
		return nil, err
	}
	chatSvc := chat.NewChatService(c.memorySvc, c.workingSvc, c.todoSvc, c.promptSvc, chatProvider)
	return chatSvc.Send(ctx, &chat.ChatRequest{Messages: messages, Model: model})
}

func (c *localChatClient) GetMemoryStats(ctx context.Context) (*memory.MemoryStats, error) {
	return c.memorySvc.GetStats(ctx)
}

func (c *localChatClient) ClearMemory(ctx context.Context) error {
	return c.memorySvc.Clear(ctx)
}

func (c *localChatClient) ClearSessionMemory(ctx context.Context) error {
	if err := c.memorySvc.ClearSession(ctx); err != nil {
		return err
	}
	if c.workingSvc != nil {
		return c.workingSvc.Clear(ctx)
	}
	return nil
}

func (c *localChatClient) DefaultModel() string {
	return provider.DefaultModelForConfig(c.config)
}

func (c *localChatClient) GetWorkingSessionSummary(ctx context.Context) (string, error) {
	if c.workingSvc == nil || c.config == nil || !c.config.History.ResumeLastSession {
		return "", nil
	}
	state, err := c.workingSvc.Get(ctx)
	if err != nil || state == nil {
		return "", err
	}
	return formatWorkingSessionSummary(state), nil
}

func formatWorkingSessionSummary(state *session.WorkingMemoryState) string {
	if state == nil {
		return ""
	}

	lines := make([]string, 0, 6)
	if strings.TrimSpace(state.CurrentTask) != "" {
		lines = append(lines, "Recovered previous workspace context:")
		lines = append(lines, "- Current goal: "+memory.SummarizeText(state.CurrentTask, 120))
	}
	if strings.TrimSpace(state.LastCompletedAction) != "" {
		lines = append(lines, "- Last completed: "+memory.SummarizeText(state.LastCompletedAction, 120))
	}
	if strings.TrimSpace(state.CurrentInProgress) != "" {
		lines = append(lines, "- In progress: "+memory.SummarizeText(state.CurrentInProgress, 120))
	}
	if strings.TrimSpace(state.NextStep) != "" {
		lines = append(lines, "- Next step: "+memory.SummarizeText(state.NextStep, 120))
	}
	if len(state.RecentFiles) > 0 {
		lines = append(lines, "- Recent files: "+strings.Join(state.RecentFiles, ", "))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
