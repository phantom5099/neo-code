package core

import (
	"context"
	"strings"
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

	controller services.Controller

	streamChan      <-chan string
	textarea        textarea.Model
	viewport        viewport.Model
	sideViewport    viewport.Model
	chatLayout      components.RenderedChatLayout
	layout          viewLayout
	copyToClipboard func(string) error
}

func NewModel(controller services.Controller, workspaceRoot string) Model {
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
	input.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "换行"),
	)
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EAF2"))
	_ = input.Cursor.SetMode(cursor.CursorBlink)

	vp := viewport.New(0, 0)
	vp.SetContent("")
	sideVP := viewport.New(0, 0)
	sideVP.SetContent("")

	return Model{
		ui: state.UIState{
			Mode:           state.ModeChat,
			Focus:          state.FocusInput,
			AutoScroll:     true,
			StatusMessage:  "Tab 切换焦点  h 切换侧栏  ] 展开系统消息",
			CommandHistory: make([]string, 0),
			CmdHistIndex:   -1,
		},
		chat: state.ChatState{
			Messages:         make([]state.Message, 0),
			SessionStartedAt: time.Now(),
			WorkspaceRoot:    workspaceRoot,
		},
		controller:      controller,
		textarea:        input,
		viewport:        vp,
		sideViewport:    sideVP,
		copyToClipboard: clipboard.WriteAll,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.textarea.Focus(),
		m.loadBootstrapCmd(),
	)
}

func (m *Model) loadBootstrapCmd() tea.Cmd {
	controller := m.controller
	return func() tea.Msg {
		if controller == nil {
			return BootstrapLoadedMsg{}
		}
		return BootstrapLoadedMsg{Data: controller.Bootstrap(context.Background())}
	}
}

func (m *Model) conversationRequest() services.ConversationRequest {
	return services.ConversationRequest{
		Messages:    m.sessionMessages(),
		ActiveModel: m.chat.ActiveModel,
	}
}

func (m *Model) sessionMessages() []services.SessionMessage {
	result := make([]services.SessionMessage, 0, len(m.chat.Messages))
	for _, msg := range m.chat.Messages {
		result = append(result, services.SessionMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Kind:      msg.Kind,
			Transient: msg.Transient,
		})
	}
	return result
}

func (m *Model) applyBootstrap(data services.BootstrapData) {
	m.chat.MemoryStats = data.MemoryStats
	m.chat.APIKeyReady = data.APIKeyReady
	m.applySnapshot(data.Snapshot)
	if strings.TrimSpace(data.ResumeSummary.Content) != "" {
		m.chat.Messages = append(m.chat.Messages, state.Message{
			Role:      data.ResumeSummary.Role,
			Content:   data.ResumeSummary.Content,
			Kind:      data.ResumeSummary.Kind,
			Timestamp: time.Now(),
			Transient: true,
		})
	}
}

func (m *Model) applyServiceMessages(messages []services.SessionMessage) {
	for _, msg := range messages {
		m.chat.Messages = append(m.chat.Messages, state.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Kind:      msg.Kind,
			Timestamp: time.Now(),
			Transient: msg.Transient,
		})
	}
}

func (m *Model) SetWidth(w int) {
	m.ui.Width = w
}

func (m *Model) SetHeight(h int) {
	m.ui.Height = h
}

func (m *Model) AddMessage(role, content string) {
	m.chat.Messages = append(m.chat.Messages, state.Message{
		Role:      role,
		Content:   content,
		Kind:      services.MessageKindPlain,
		Timestamp: time.Now(),
	})
}

func (m *Model) AddTransientMessage(role, content string) {
	m.chat.Messages = append(m.chat.Messages, state.Message{
		Role:      role,
		Content:   content,
		Kind:      services.MessageKindPlain,
		Timestamp: time.Now(),
		Transient: true,
	})
}

func (m *Model) AddErrorMessage(content string) {
	m.chat.Messages = append(m.chat.Messages, state.Message{
		Role:      "assistant",
		Content:   content,
		Kind:      services.MessageKindPlain,
		Timestamp: time.Now(),
		Error:     true,
		Transient: true,
	})
}

func (m *Model) AppendLastMessage(content string) {
	if len(m.chat.Messages) > 0 {
		m.chat.Messages[len(m.chat.Messages)-1].Content += content
	}
}

func (m *Model) FinishLastMessage() {
	if len(m.chat.Messages) > 0 {
		m.chat.Messages[len(m.chat.Messages)-1].Streaming = false
	}
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
