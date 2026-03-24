package core

import (
	"context"
	"sync"
	"time"

	"go-llm-demo/configs"
	"go-llm-demo/internal/server/domain"
	"go-llm-demo/internal/tui/infra"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mode 表示 TUI 当前所处的主交互模式。
type Mode int

const (
	ModeChat Mode = iota
	ModeCodeInput
	ModeHelp
	ModeMemory
)

// Model 保存 Bubble Tea 运行所需的全部界面状态。
type Model struct {
	width   int
	height  int
	mode    Mode
	focused string

	messages                 []Message
	historyTurns             int
	maxToolContextMessages   int
	maxToolContextOutputSize int

	generating  bool
	activeModel string

	memoryStats infra.MemoryStats
	todos       []domain.Todo

	commandHistory []string
	cmdHistIndex   int

	client  infra.ChatClient
	persona string

	workspaceRoot string

	toolExecuting bool
	apiKeyReady   bool
	configPath    string

	streamChan <-chan string
	textarea   textarea.Model
	viewport   viewport.Model
	autoScroll bool

	mu *sync.Mutex
}

// Message 是 TUI 内部使用的聊天消息表示。
type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Streaming bool
}

// NewModel 创建 TUI 状态模型。
// historyTurns 用于限制发送给后端的短期对话轮数，避免原始消息无限增长。
func NewModel(client infra.ChatClient, persona string, historyTurns int, configPath, workspaceRoot string) Model {
	stats, _ := client.GetMemoryStats(context.Background())
	if stats == nil {
		stats = &infra.MemoryStats{}
	}

	maxToolContextMessages := 3
	maxToolContextOutputSize := 4000
	if configs.GlobalAppConfig != nil {
		if historyTurns <= 0 {
			historyTurns = configs.GlobalAppConfig.History.ShortTermTurns
		}
		maxToolContextMessages = configs.GlobalAppConfig.History.MaxToolContextMessages
		maxToolContextOutputSize = configs.GlobalAppConfig.History.MaxToolContextOutputSize
	}

	if historyTurns <= 0 {
		historyTurns = 6
	}

	// 输入框和视口在初始化阶段统一配置，后续 Update/View 只处理状态变化。
	input := textarea.New()
	focusedStyle, blurredStyle := textarea.DefaultStyles()
	focusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	blurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	focusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EAF2"))
	blurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAB2C0"))
	input.FocusedStyle = focusedStyle
	input.BlurredStyle = blurredStyle
	input.Placeholder = "输入消息..."
	input.Focus()
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.Prompt = "> "
	input.CharLimit = 0
	input.KeyMap.InsertNewline.SetEnabled(true)
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EAF2"))
	_ = input.Cursor.SetMode(cursor.CursorBlink)

	vp := viewport.New(0, 0)
	vp.SetContent("")

	return Model{
		mode:                     ModeChat,
		focused:                  "input",
		messages:                 make([]Message, 0),
		historyTurns:             historyTurns,
		maxToolContextMessages:   maxToolContextMessages,
		maxToolContextOutputSize: maxToolContextOutputSize,
		activeModel:              client.DefaultModel(),
		memoryStats:              *stats,
		commandHistory:           make([]string, 0),
		cmdHistIndex:             -1,
		client:                   client,
		persona:                  persona,
		workspaceRoot:            workspaceRoot,
		apiKeyReady:              configs.RuntimeAPIKey() != "",
		configPath:               configPath,
		textarea:                 input,
		viewport:                 vp,
		autoScroll:               true,
		mu:                       &sync.Mutex{},
	}
}

func (m *Model) mutex() *sync.Mutex {
	if m.mu == nil {
		m.mu = &sync.Mutex{}
	}
	return m.mu
}

// Init 返回 Bubble Tea 的初始命令。
func (m Model) Init() tea.Cmd {
	return m.textarea.Focus()
}

// SetWidth 更新当前视口宽度。
func (m *Model) SetWidth(w int) {
	m.width = w
}

// SetHeight 更新当前视口高度。
func (m *Model) SetHeight(h int) {
	m.height = h
}

// AddMessage 向聊天历史追加一条带时间戳的消息。
func (m *Model) AddMessage(role, content string) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()
	m.messages = append(m.messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AppendLastMessage 将流式内容追加到最后一条消息中。
func (m *Model) AppendLastMessage(content string) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()
	if len(m.messages) > 0 {
		m.messages[len(m.messages)-1].Content += content
	}
}

// FinishLastMessage 将最后一条消息标记为结束流式输出。
func (m *Model) FinishLastMessage() {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()
	if len(m.messages) > 0 {
		m.messages[len(m.messages)-1].Streaming = false
	}
}

// TrimHistory 在保留系统消息的同时裁剪最近的非系统对话轮次。
func (m *Model) TrimHistory(maxTurns int) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()
	if len(m.messages) <= maxTurns*2 {
		return
	}

	// 系统消息承载 persona、工具上下文等结构化信息，不参与普通轮次裁剪。
	var system []Message
	var others []Message

	for _, msg := range m.messages {
		if msg.Role == "system" {
			system = append(system, msg)
		} else {
			others = append(others, msg)
		}
	}

	if len(others) > maxTurns*2 {
		others = others[len(others)-maxTurns*2:]
	}

	m.messages = append(system, others...)
}
