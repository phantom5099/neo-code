package core

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m.handleMouse(msg)

	case StreamChunkMsg:
		if m.chat.Generating {
			m.AppendLastMessage(msg.Content)
			m.refreshViewport()
		}
		return m, m.streamResponseFromChannel()

	case StreamDoneMsg:
		return m.handleStreamDone()

	case StreamErrorMsg:
		return m.handleStreamError(msg.Err)

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
			m.setStatusMessage("Memory 已刷新")
		}
		m.refreshViewport()
		return m, nil

	case ExitMsg:
		return m, tea.Quit

	case ToolResultMsg:
		return m.handleToolResult(msg)

	case ToolErrorMsg:
		return m.handleToolError(msg.Err)
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "tab":
		cmd := m.focusNext()
		m.refreshViewport()
		return *m, cmd
	case "shift+tab":
		cmd := m.focusPrev()
		m.refreshViewport()
		return *m, cmd
	}

	if m.ui.Mode == state.ModeHelp && msg.Type == tea.KeyEsc {
		m.ui.Mode = state.ModeChat
		m.refreshViewport()
		return *m, nil
	}

	if m.ui.Focus == state.FocusInput {
		return m.handleInputKey(msg)
	}
	return m.handleViewportKey(msg)
}

func (m *Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		cmd := m.setFocus(state.FocusMain)
		m.refreshViewport()
		return *m, cmd
	case tea.KeyEnter:
		if !msg.Alt {
			return m.handleSubmit()
		}
	}

	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "up":
		if cmd := m.browseHistoryUp(); cmd != nil {
			m.refreshViewport()
			return *m, cmd
		}
	case "down":
		if cmd := m.browseHistoryDown(); cmd != nil {
			m.refreshViewport()
			return *m, cmd
		}
	}

	m.chat.CmdHistIndex = -1
	var inputCmd tea.Cmd
	m.textarea, inputCmd = m.textarea.Update(msg)
	m.refreshViewport()
	return *m, inputCmd
}

func (m *Model) handleViewportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.ui.Focus == state.FocusSide {
			cmd := m.setFocus(state.FocusMain)
			m.refreshViewport()
			return *m, cmd
		}
	}

	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "/":
		m.textarea.SetValue("/")
		cmd := m.setFocus(state.FocusInput)
		m.refreshViewport()
		return *m, cmd
	case "enter":
		cmd := m.setFocus(state.FocusInput)
		m.refreshViewport()
		return *m, cmd
	case "pgup":
		m.scrollFocusedViewportPageUp()
	case "pgdown":
		m.scrollFocusedViewportPageDown()
	case "up":
		m.scrollFocusedViewportLineUp()
	case "down":
		m.scrollFocusedViewportLineDown()
	case "g":
		m.scrollFocusedViewportTop()
	case "h":
		m.toggleSidebar()
	case "]":
		m.ui.SystemExpanded = !m.ui.SystemExpanded
		if m.ui.SystemExpanded {
			m.setStatusMessage("系统消息已展开")
		} else {
			m.setStatusMessage("系统消息已折叠")
		}
	case "ctrl+l":
		m.refreshViewport()
		return *m, tea.ClearScreen
	}

	if msg.String() == "G" {
		m.scrollFocusedViewportBottom()
	}

	m.refreshViewport()
	return *m, nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if handled := m.handleMouseClick(msg); handled {
		m.refreshViewport()
		return *m, nil
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		switch {
		case m.layout.sideVisible && msg.X >= m.layout.sideX:
			m.setFocus(state.FocusSide)
		case msg.Y >= m.layout.inputTop:
			_ = m.setFocus(state.FocusInput)
		default:
			_ = m.setFocus(state.FocusMain)
		}
	}

	switch {
	case m.layout.sideVisible && msg.X >= m.layout.sideX:
		var cmd tea.Cmd
		m.sideViewport, cmd = m.sideViewport.Update(msg)
		return *m, cmd
	case msg.Y < m.layout.mainContentHeight:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.ui.AutoScroll = m.viewport.AtBottom()
		return *m, cmd
	default:
		return *m, nil
	}
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
		m.setLastError(err)
		m.setStatusMessage("copy failed")
		return true
	}

	lang := strings.TrimSpace(region.CodeBlock.Lang)
	if lang == "" {
		lang = "text"
	}
	m.setStatusMessage("copied: " + lang)
	return true
}

func (m *Model) browseHistoryUp() tea.Cmd {
	if len(m.chat.CommandHistory) == 0 {
		return nil
	}
	if m.chat.CmdHistIndex == -1 {
		m.chat.CommandDraft = m.textarea.Value()
	}
	if m.chat.CmdHistIndex < len(m.chat.CommandHistory)-1 {
		m.chat.CmdHistIndex++
	}
	if m.chat.CmdHistIndex >= 0 && m.chat.CmdHistIndex < len(m.chat.CommandHistory) {
		m.textarea.SetValue(m.chat.CommandHistory[len(m.chat.CommandHistory)-1-m.chat.CmdHistIndex])
		m.textarea.CursorEnd()
	}
	return nil
}

func (m *Model) browseHistoryDown() tea.Cmd {
	if m.chat.CmdHistIndex > 0 {
		m.chat.CmdHistIndex--
		m.textarea.SetValue(m.chat.CommandHistory[len(m.chat.CommandHistory)-1-m.chat.CmdHistIndex])
		m.textarea.CursorEnd()
		return nil
	}
	if m.chat.CmdHistIndex == 0 {
		m.chat.CmdHistIndex = -1
		m.textarea.SetValue(m.chat.CommandDraft)
		m.textarea.CursorEnd()
		return nil
	}
	return nil
}

func (m *Model) toggleSidebar() {
	if m.ui.Width < sidebarBreakpoint {
		m.ui.SideNarrowOpen = !m.layout.sideVisible
	} else {
		m.ui.SideCollapsed = !m.ui.SideCollapsed
	}
	if !m.shouldShowSide() && m.ui.Focus == state.FocusSide {
		_ = m.setFocus(state.FocusMain)
	}
	if m.shouldShowSide() {
		m.setStatusMessage("side: visible")
	} else {
		m.setStatusMessage("side: hidden")
	}
}

func (m *Model) scrollFocusedViewportPageUp() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.HalfViewUp()
	default:
		m.viewport.HalfViewUp()
		m.ui.AutoScroll = false
	}
}

func (m *Model) scrollFocusedViewportPageDown() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.HalfViewDown()
	default:
		m.viewport.HalfViewDown()
		m.ui.AutoScroll = m.viewport.AtBottom()
	}
}

func (m *Model) scrollFocusedViewportLineUp() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.LineUp(1)
	default:
		m.viewport.LineUp(1)
		m.ui.AutoScroll = false
	}
}

func (m *Model) scrollFocusedViewportLineDown() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.LineDown(1)
	default:
		m.viewport.LineDown(1)
		m.ui.AutoScroll = m.viewport.AtBottom()
	}
}

func (m *Model) scrollFocusedViewportTop() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.GotoTop()
	default:
		m.viewport.GotoTop()
		m.ui.AutoScroll = false
	}
}

func (m *Model) scrollFocusedViewportBottom() {
	switch m.ui.Focus {
	case state.FocusSide:
		m.sideViewport.GotoBottom()
	default:
		m.viewport.GotoBottom()
		m.ui.AutoScroll = true
	}
}

func (m *Model) handleStreamDone() (tea.Model, tea.Cmd) {
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
	m.setStatusMessage("生成完成")

	if shouldCheckToolCall {
		if calls := parseAssistantTools(lastContent); len(calls) > 0 {
			call := calls[0]
			mu := m.mutex()
			mu.Lock()
			if m.chat.ToolExecuting {
				mu.Unlock()
				return *m, nil
			}
			m.chat.ToolExecuting = true
			mu.Unlock()

			m.rememberTouchedFiles(toolPathsFromCall(call)...)
			m.AddMessage("system", formatToolStatusMessage(call.Tool, call.Params))
			m.refreshViewport()

			return *m, func() tea.Msg {
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
	return *m, nil
}

func (m *Model) handleStreamError(err error) (tea.Model, tea.Cmd) {
	mu := m.mutex()
	mu.Lock()
	m.chat.Generating = false
	m.streamChan = nil
	replacedPlaceholder := false
	if len(m.chat.Messages) > 0 {
		lastMsg := &m.chat.Messages[len(m.chat.Messages)-1]
		if lastMsg.Role == "assistant" && strings.TrimSpace(lastMsg.Content) == "" {
			lastMsg.Content = fmt.Sprintf("错误：%v", err)
			lastMsg.Streaming = false
			lastMsg.Error = true
			replacedPlaceholder = true
		}
	}
	mu.Unlock()

	if !replacedPlaceholder {
		m.AddErrorMessage(fmt.Sprintf("错误：%v", err))
	}
	m.setLastError(err)
	m.setStatusMessage("")
	m.TrimHistory(m.chat.HistoryTurns)
	m.refreshViewport()
	return *m, nil
}

func (m *Model) handleToolResult(msg ToolResultMsg) (tea.Model, tea.Cmd) {
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
		m.setStatusMessage("等待审批")
		m.refreshViewport()
		return *m, nil
	}

	m.rememberTouchedFiles(toolPathsFromCall(msg.Call)...)
	m.rememberTouchedFiles(toolPathsFromResult(msg.Result)...)
	m.AddMessage("system", formatToolContextMessage(msg.Result))
	m.AddMessage("assistant", "")
	m.chat.Generating = true
	m.setStatusMessage("Generating...")
	m.refreshViewport()

	messages := m.buildMessages()
	return *m, m.streamResponse(messages)
}

func (m *Model) handleToolError(err error) (tea.Model, tea.Cmd) {
	mu := m.mutex()
	mu.Lock()
	m.chat.ToolExecuting = false
	mu.Unlock()

	m.AddMessage("system", formatToolErrorContext(err))
	m.AddMessage("assistant", "")
	m.chat.Generating = true
	m.setLastError(err)
	m.setStatusMessage("Generating...")
	m.refreshViewport()

	messages := m.buildMessages()
	return *m, m.streamResponse(messages)
}
