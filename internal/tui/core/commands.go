package core

import (
	"context"
	"strings"
	"time"

	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.textarea.SetHeight(m.calculateInputHeight())
	m.syncLayout()

	if input == "" {
		return *m, nil
	}

	if m.ui.Mode == state.ModeHelp {
		m.ui.Mode = state.ModeChat
		return *m, nil
	}

	return m.handleInput(input)
}

func (m *Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	return m.handleInput(input)
}

func (m *Model) handleInput(input string) (tea.Model, tea.Cmd) {
	session := m.session
	snapshot := m.sessionSnapshot()
	return *m, func() tea.Msg {
		if session == nil {
			return InputHandledMsg{Err: context.Canceled}
		}
		result, err := session.HandleInput(context.Background(), snapshot, input)
		return InputHandledMsg{Result: result, Err: err}
	}
}

func (m *Model) applyInputResult(result services.InputResult) (tea.Model, tea.Cmd) {
	if result.Quit {
		return *m, tea.Quit
	}
	if result.OpenHelp {
		m.ui.Mode = state.ModeHelp
		m.refreshViewport()
		return *m, nil
	}

	if result.ReportError != nil {
		m.setLastError(result.ReportError)
	}
	if strings.TrimSpace(result.StatusMessage) != "" {
		m.setStatusMessage(result.StatusMessage)
	}
	if result.MutationFeedback != nil {
		m.applyMutationFeedback(result.MutationFeedback)
	}
	if result.MemoryFeedback != nil {
		if result.MemoryFeedback.Action == services.MemoryActionClearSession {
			m.ui.CommandDraft = ""
		}
		m.applyMemoryFeedback(result.MemoryFeedback)
	}
	if result.TurnResolution != nil {
		return m.applyTurnResolution(*result.TurnResolution)
	}
	if len(result.Messages) > 0 {
		m.applyServiceMessages(result.Messages)
	}
	if strings.TrimSpace(result.HistoryEntry) != "" {
		m.ui.CommandHistory = append(m.ui.CommandHistory, result.HistoryEntry)
		m.ui.CmdHistIndex = -1
		m.ui.CommandDraft = ""
	}
	if result.Stream != nil {
		m.chat.StartStreaming()
		m.ui.AutoScroll = true
		m.streamChan = result.Stream
		m.refreshViewport()
		return *m, m.streamResponseFromChannel()
	}

	m.refreshViewport()
	return *m, nil
}

func (m *Model) applyMutationFeedback(feedback *services.MutationFeedback) {
	if feedback == nil {
		return
	}
	m.chat.ApplyMutationFeedback(feedback, time.Now())
	m.setStatusMessage(feedback.StatusMessage)
	if feedback.ValidationErr != nil {
		m.setLastError(feedback.ValidationErr)
	}
	m.refreshViewport()
}

func (m *Model) applyMemoryFeedback(feedback *services.MemoryFeedback) {
	if feedback == nil {
		return
	}
	m.chat.ApplyMemoryFeedback(feedback, time.Now())
	m.setStatusMessage(feedback.StatusMessage)
	m.refreshViewport()
}

func (m *Model) applySnapshot(snapshot services.UISnapshot) {
	m.chat.ApplySnapshot(snapshot)
}
