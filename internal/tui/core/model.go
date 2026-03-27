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

	session services.SessionService

	streamChan      <-chan string
	textarea        textarea.Model
	viewport        viewport.Model
	sideViewport    viewport.Model
	chatLayout      components.RenderedChatLayout
	layout          viewLayout
	copyToClipboard func(string) error
}

func NewModel(session services.SessionService, workspaceRoot string) Model {
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
		chat:            state.NewChatState(workspaceRoot),
		session:         session,
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
	session := m.session
	return func() tea.Msg {
		if session == nil {
			return BootstrapLoadedMsg{}
		}
		return BootstrapLoadedMsg{Data: session.Bootstrap(context.Background())}
	}
}

func (m *Model) sessionMessages() []services.SessionMessage {
	return m.chat.SessionMessages()
}

func (m *Model) sessionSnapshot() services.SessionSnapshot {
	return m.chat.Snapshot(m.ui.ApprovalRunning)
}

func (m *Model) applyBootstrap(data services.BootstrapData) {
	m.chat.ApplyBootstrap(data, time.Now())
}

func (m *Model) applyServiceMessages(messages []services.SessionMessage) {
	m.chat.ApplyMessages(messages, time.Now())
}

func (m *Model) SetWidth(w int) {
	m.ui.Width = w
}

func (m *Model) SetHeight(h int) {
	m.ui.Height = h
}

func (m *Model) AddMessage(role, content string) {
	m.chat.AddPlainMessage(role, content, time.Now())
}

func (m *Model) AddTransientMessage(role, content string) {
	m.chat.AddTransientPlainMessage(role, content, time.Now())
}

func (m *Model) AddErrorMessage(content string) {
	m.chat.AddErrorMessage(content, time.Now())
}

func (m *Model) AppendLastMessage(content string) {
	m.chat.AppendLastMessage(content)
}

func (m *Model) FinishLastMessage() {
	m.chat.FinishLastMessage()
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
