package core

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/tui/components"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetWidth(msg.Width)
		m.SetHeight(msg.Height)
		m.syncLayout()
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if handled := m.handleMouseClick(msg); handled {
			m.refreshViewport()
			return m, nil
		}
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		m.ui.AutoScroll = m.viewport.AtBottom()
		return m, vpCmd

	case StreamChunkMsg:
		if m.chat.Generating {
			m.AppendLastMessage(msg.Content)
			m.refreshViewport()
		}
		return m, m.streamResponseFromChannel()

	case StreamDoneMsg:
		mu := m.mutex()
		mu.Lock()
		m.chat.Generating = false
		m.streamChan = nil

		var lastContent string
		shouldCheckToolCall := !m.chat.ToolExecuting && len(m.chat.Messages) > 0
		if len(m.chat.Messages) > 0 {
			lastMsg := &m.chat.Messages[len(m.chat.Messages)-1]
			lastMsg.Streaming = false
			if lastMsg.Role == "assistant" {
				lastContent = lastMsg.Content
			} else {
				shouldCheckToolCall = false
			}
		}
		mu.Unlock()

		if shouldCheckToolCall {
			if calls := parseAssistantTools(lastContent); len(calls) > 0 {
				call := calls[0]
				mu := m.mutex()
				mu.Lock()
				if m.chat.ToolExecuting {
					mu.Unlock()
					return m, nil
				}
				m.chat.ToolExecuting = true
				mu.Unlock()

				m.AddMessage("system", formatToolStatusMessage(call.Tool, call.Params))

				return m, func() tea.Msg {
					result := executeToolCall(call)
					if result == nil {
						mu := m.mutex()
						mu.Lock()
						m.chat.ToolExecuting = false
						mu.Unlock()
						return ToolErrorMsg{Err: fmt.Errorf("tool execution failed: empty result")}
					}
					return ToolResultMsg{Result: result, Call: call}
				}
			}
		}

		m.refreshViewport()
		return m, nil

	case StreamErrorMsg:
		mu := m.mutex()
		mu.Lock()
		m.chat.Generating = false
		m.streamChan = nil
		replacedPlaceholder := false
		if len(m.chat.Messages) > 0 {
			lastMsg := &m.chat.Messages[len(m.chat.Messages)-1]
			if lastMsg.Role == "assistant" && strings.TrimSpace(lastMsg.Content) == "" {
				lastMsg.Content = fmt.Sprintf("Error: %v", msg.Err)
				lastMsg.Streaming = false
				replacedPlaceholder = true
			}
		}
		mu.Unlock()

		if !replacedPlaceholder {
			m.AddMessage("assistant", fmt.Sprintf("Error: %v", msg.Err))
		}
		m.TrimHistory(m.chat.HistoryTurns)
		m.refreshViewport()
		return m, nil

	case ShowHelpMsg:
		m.ui.Mode = state.ModeHelp
		m.refreshViewport()
		return m, nil

	case HideHelpMsg:
		m.ui.Mode = state.ModeChat
		m.refreshViewport()
		return m, nil

	case RefreshMemoryMsg:
		stats, err := m.client.GetMemoryStats(context.Background())
		if err == nil && stats != nil {
			m.chat.MemoryStats = *stats
		}
		m.refreshViewport()
		return m, nil

	case ExitMsg:
		return m, tea.Quit

	case ToolResultMsg:
		mu := m.mutex()
		mu.Lock()
		m.chat.ToolExecuting = false
		mu.Unlock()

		if toolType, target, ok := isSecurityAskResult(msg.Result); ok {
			mu := m.mutex()
			mu.Lock()
			m.chat.PendingApproval = &state.PendingApproval{
				Call:     msg.Call,
				ToolType: toolType,
				Target:   target,
			}
			pending := m.chat.PendingApproval
			mu.Unlock()

			m.AddMessage("assistant", formatPendingApprovalMessage(pending))
			m.refreshViewport()
			return m, nil
		}

		m.AddMessage("system", formatToolContextMessage(msg.Result))
		m.AddMessage("assistant", "")
		m.chat.Generating = true
		m.refreshViewport()

		messages := m.buildMessages()
		return m, m.streamResponse(messages)

	case ToolErrorMsg:
		mu := m.mutex()
		mu.Lock()
		m.chat.ToolExecuting = false
		mu.Unlock()

		m.AddMessage("system", formatToolErrorContext(msg.Err))
		m.AddMessage("assistant", "")
		m.chat.Generating = true
		m.refreshViewport()

		messages := m.buildMessages()
		return m, m.streamResponse(messages)
	}

	return m, cmd
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ui.Mode == state.ModeHelp {
		if msg.Type == tea.KeyEsc || msg.String() == "q" {
			m.ui.Mode = state.ModeChat
			m.refreshViewport()
			return *m, nil
		}
	}

	switch msg.Type {
	case tea.KeyEnter:
		if !msg.Alt {
			return m.handleSubmit()
		}

	case tea.KeyF5, tea.KeyF8:
		return m.handleSubmit()

	case tea.KeyPgUp:
		m.ui.AutoScroll = false
		m.viewport.HalfViewUp()
		return *m, nil

	case tea.KeyPgDown:
		m.viewport.HalfViewDown()
		m.ui.AutoScroll = m.viewport.AtBottom()
		return *m, nil

	case tea.KeyUp:
		if strings.TrimSpace(m.textarea.Value()) == "" && len(m.chat.CommandHistory) > 0 {
			if m.chat.CmdHistIndex < len(m.chat.CommandHistory)-1 {
				m.chat.CmdHistIndex++
			}
			if m.chat.CmdHistIndex >= 0 && m.chat.CmdHistIndex < len(m.chat.CommandHistory) {
				m.textarea.SetValue(m.chat.CommandHistory[len(m.chat.CommandHistory)-1-m.chat.CmdHistIndex])
				m.textarea.CursorEnd()
				return *m, nil
			}
		}

	case tea.KeyDown:
		if m.chat.CmdHistIndex > 0 {
			m.chat.CmdHistIndex--
			m.textarea.SetValue(m.chat.CommandHistory[len(m.chat.CommandHistory)-1-m.chat.CmdHistIndex])
			m.textarea.CursorEnd()
			return *m, nil
		}
		if m.chat.CmdHistIndex == 0 {
			m.chat.CmdHistIndex = -1
			m.textarea.Reset()
			return *m, nil
		}
	}

	m.chat.CmdHistIndex = -1
	var inputCmd tea.Cmd
	m.textarea, inputCmd = m.textarea.Update(msg)
	m.refreshViewport()
	if m.viewport.AtBottom() {
		m.ui.AutoScroll = true
	}
	return *m, inputCmd
}

func (m *Model) handleMouseClick(msg tea.MouseMsg) bool {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return false
	}

	contentRow, contentCol, ok := m.chatContentPosition(msg)
	if !ok {
		return false
	}

	region, found := findClickableRegion(m.chatLayout.Regions, contentRow, contentCol)
	if !found || region.Kind != "copy" {
		return false
	}

	if err := m.copyCodeBlock(region.CodeBlock); err != nil {
		m.ui.CopyStatus = fmt.Sprintf("Copy failed: %v", err)
		return true
	}

	m.ui.CopyStatus = components.FormatCopyNotice(region.CodeBlock)
	return true
}
