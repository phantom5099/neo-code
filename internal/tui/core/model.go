package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"neo-code/internal/tui/components"
	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	ui   state.UIState
	chat state.ChatState

	client services.ChatClient

	streamChan      <-chan string
	textarea        textarea.Model
	viewport        viewport.Model
	sideViewport    viewport.Model
	chatLayout      components.RenderedChatLayout
	layout          viewLayout
	copyToClipboard func(string) error

	mu *sync.Mutex
}

const resumeSummaryPrefix = "[RESUME_SUMMARY]"

func NewModel(client services.ChatClient, historyTurns int, configPath, workspaceRoot string) Model {
	stats, _ := client.GetMemoryStats(context.Background())
	if stats == nil {
		stats = &services.MemoryStats{}
	}
	if historyTurns <= 0 {
		historyTurns = 6
	}

	input := textarea.New()
	focusedStyle, blurredStyle := textarea.DefaultStyles()
	focusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	blurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	focusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EAF2"))
	blurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAB2C0"))
	input.FocusedStyle = focusedStyle
	input.BlurredStyle = blurredStyle
	input.Placeholder = "Type a message..."
	input.Focus()
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.Prompt = "> "
	input.CharLimit = 0
	input.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EAF2"))
	_ = input.Cursor.SetMode(cursor.CursorBlink)

	vp := viewport.New(0, 0)
	vp.SetContent("")
	sideVP := viewport.New(0, 0)
	sideVP.SetContent("")
	snapshot := services.ReadUISnapshot(context.Background(), client)
	startedAt := time.Now()

	model := Model{
		ui: state.UIState{
			Mode:            state.ModeChat,
			Focus:           state.FocusInput,
			AutoScroll:      true,
			StatusMessage:   "Tab切换焦点，h切换侧栏，]展开系统消息",
			HelpCollapsed:   false,
			FirstGuideShown: true,
		},
		chat: state.ChatState{
			Messages:         make([]state.Message, 0),
			HistoryTurns:     historyTurns,
			SessionStartedAt: startedAt,
			ProviderName:     snapshot.ProviderName,
			ActiveModel:      firstNonEmpty(snapshot.CurrentModel, client.DefaultModel()),
			DefaultModel:     snapshot.DefaultModel,
			WorkspaceSummary: snapshot.WorkspaceSummary,
			MemoryStats:      *stats,
			CommandHistory:   make([]string, 0),
			CmdHistIndex:     -1,
			WorkspaceRoot:    workspaceRoot,
			APIKeyReady:      services.RuntimeAPIKeyReady(),
			ConfigPath:       configPath,
		},
		client:          client,
		textarea:        input,
		viewport:        vp,
		sideViewport:    sideVP,
		copyToClipboard: clipboard.WriteAll,
		mu:              &sync.Mutex{},
	}

	if provider, ok := client.(services.WorkingSessionSummaryProvider); ok {
		if summary, err := provider.GetWorkingSessionSummary(context.Background()); err == nil && strings.TrimSpace(summary) != "" {
			model.chat.Messages = append(model.chat.Messages, state.Message{
				Role:      "system",
				Content:   resumeSummaryPrefix + "\n" + summary,
				Timestamp: time.Now(),
			})
		}
	}

	return model
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (m *Model) mutex() *sync.Mutex {
	if m.mu == nil {
		m.mu = &sync.Mutex{}
	}
	return m.mu
}

func (m Model) Init() tea.Cmd {
	return m.textarea.Focus()
}

func (m *Model) SetWidth(w int) {
	m.ui.Width = w
}

func (m *Model) SetHeight(h int) {
	m.ui.Height = h
}

func (m *Model) AddMessage(role, content string) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	m.chat.Messages = append(m.chat.Messages, state.Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) AddErrorMessage(content string) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	m.chat.Messages = append(m.chat.Messages, state.Message{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
		Error:     true,
	})
}

func (m *Model) AppendLastMessage(content string) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	if len(m.chat.Messages) > 0 {
		m.chat.Messages[len(m.chat.Messages)-1].Content += content
	}
}

func (m *Model) FinishLastMessage() {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	if len(m.chat.Messages) > 0 {
		m.chat.Messages[len(m.chat.Messages)-1].Streaming = false
	}
}

func (m *Model) TrimHistory(maxTurns int) {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()

	if len(m.chat.Messages) <= maxTurns*2 {
		return
	}

	var system []state.Message
	var others []state.Message
	for _, msg := range m.chat.Messages {
		if msg.Role == "system" {
			system = append(system, msg)
			continue
		}
		others = append(others, msg)
	}

	if len(others) > maxTurns*2 {
		others = others[len(others)-maxTurns*2:]
	}

	m.chat.Messages = append(system, others...)
}

func isResumeSummaryMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), resumeSummaryPrefix)
}

func (m *Model) setStatusMessage(message string) {
	m.ui.StatusMessage = strings.TrimSpace(message)
}

func (m *Model) setLastError(err error) {
	if err == nil {
		return
	}
	m.ui.LastError = err.Error()
}

func (m *Model) rememberTouchedFiles(paths ...string) {
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		exists := false
		for _, existing := range m.chat.TouchedFiles {
			if strings.EqualFold(existing, trimmed) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		m.chat.TouchedFiles = append(m.chat.TouchedFiles, trimmed)
	}
}
