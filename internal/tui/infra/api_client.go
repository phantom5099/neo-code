package infra

import (
	"context"
	"strings"

	"go-llm-demo/configs"
	"go-llm-demo/internal/server/domain"
	"go-llm-demo/internal/server/infra/provider"
	"go-llm-demo/internal/server/infra/repository"
	"go-llm-demo/internal/server/infra/tools"
	"go-llm-demo/internal/server/service"
)

type Message = domain.Message

// ChatClient 定义 TUI 侧依赖的最小聊天与记忆接口。
type ChatClient interface {
	Chat(ctx context.Context, messages []Message, model string) (<-chan string, error)
	GetMemoryStats(ctx context.Context) (*MemoryStats, error)
	ClearMemory(ctx context.Context) error
	ClearSessionMemory(ctx context.Context) error
	GetTodoList(ctx context.Context) ([]domain.Todo, error)
	DefaultModel() string
}

// MemoryStats 是 TUI 展示 memory 面板所需的聚合统计信息。
type MemoryStats struct {
	PersistentItems int
	SessionItems    int
	TotalItems      int
	TopK            int
	MinScore        float64
	Path            string
	ByType          map[string]int
}

type localChatClient struct {
	roleSvc    domain.RoleService
	memorySvc  domain.MemoryService
	workingSvc domain.WorkingMemoryService
	todoSvc    domain.TodoService
	config     *configs.AppConfiguration
}

// NewLocalChatClient 将本地服务组装为 TUI 使用的聊天客户端。
func NewLocalChatClient() (ChatClient, error) {
	cfg := configs.GlobalAppConfig
	if cfg == nil {
		return nil, context.Canceled
	}

	storePath := strings.TrimSpace(cfg.Memory.StoragePath)
	if storePath == "" {
		storePath = "./data/memory_rules.json"
	}
	maxItems := cfg.Memory.MaxItems
	if maxItems <= 0 {
		maxItems = 1000
	}
	persistentRepo := repository.NewFileMemoryStore(storePath, maxItems)
	sessionRepo := repository.NewSessionMemoryStore(maxItems)
	workingRepo := repository.NewWorkingMemoryStore()
	memorySvc := service.NewMemoryService(
		persistentRepo,
		sessionRepo,
		cfg.Memory.TopK,
		cfg.Memory.MinMatchScore,
		cfg.Memory.MaxPromptChars,
		storePath,
		cfg.Memory.PersistTypes,
	)
	workingSvc := service.NewWorkingMemoryService(workingRepo, cfg.History.ShortTermTurns, tools.GetWorkspaceRoot())

	roleRepo := repository.NewFileRoleStore("./data/roles.json")
	roleSvc := service.NewRoleService(roleRepo, strings.TrimSpace(cfg.Persona.FilePath))

	todoRepo := repository.NewInMemoryTodoRepository()
	todoSvc := service.NewTodoService(todoRepo)
	tools.GlobalRegistry.Register(tools.NewTodoTool(todoSvc))

	// 当前仍以内进程方式组装服务，后续替换为真实 transport 时可继续复用该接口。
	return &localChatClient{
		roleSvc:    roleSvc,
		memorySvc:  memorySvc,
		workingSvc: workingSvc,
		todoSvc:    todoSvc,
		config:     cfg,
	}, nil
}

// Chat 通过本地聊天服务发送消息。
func (c *localChatClient) Chat(ctx context.Context, messages []Message, model string) (<-chan string, error) {
	chatProvider, err := provider.NewChatProvider(model)
	if err != nil {
		return nil, err
	}
	chatSvc := service.NewChatService(c.memorySvc, c.workingSvc, c.todoSvc, c.roleSvc, chatProvider)
	return chatSvc.Send(ctx, &domain.ChatRequest{Messages: messages, Model: model})
}

// GetMemoryStats 返回 TUI 所需的当前记忆统计信息。
func (c *localChatClient) GetMemoryStats(ctx context.Context) (*MemoryStats, error) {
	stats, err := c.memorySvc.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	return &MemoryStats{
		PersistentItems: stats.PersistentItems,
		SessionItems:    stats.SessionItems,
		TotalItems:      stats.TotalItems,
		TopK:            stats.TopK,
		MinScore:        stats.MinScore,
		Path:            stats.Path,
		ByType:          stats.ByType,
	}, nil
}

// ClearMemory 通过本地记忆服务清空长期记忆。
func (c *localChatClient) ClearMemory(ctx context.Context) error {
	return c.memorySvc.Clear(ctx)
}

// ClearSessionMemory 清空会话记忆和工作记忆状态。
func (c *localChatClient) ClearSessionMemory(ctx context.Context) error {
	if err := c.memorySvc.ClearSession(ctx); err != nil {
		return err
	}
	if c.workingSvc != nil {
		return c.workingSvc.Clear(ctx)
	}
	return nil
}

// GetTodoList 返回当前任务清单。
func (c *localChatClient) GetTodoList(ctx context.Context) ([]domain.Todo, error) {
	if c.todoSvc == nil {
		return nil, nil
	}
	return c.todoSvc.ListTodos(ctx)
}

// DefaultModel 返回 TUI 使用的默认模型。
func (c *localChatClient) DefaultModel() string {
	return provider.DefaultModelForConfig(c.config)
}
