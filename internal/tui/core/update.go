package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"neo-code/internal/tui/services"
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

	case BootstrapLoadedMsg:
		m.applyBootstrap(msg.Data)
		m.refreshViewport()
		return m, nil

	case InputHandledMsg:
		if msg.Err != nil {
			m.AddErrorMessage(fmt.Sprintf("操作失败：%v", msg.Err))
			m.setLastError(msg.Err)
			m.refreshViewport()
			return m, nil
		}
		return m.applyInputResult(msg.Result)

	case MemoryFeedbackMsg:
		if msg.Err != nil {
			m.AddErrorMessage(fmt.Sprintf("读取运行时状态失败：%v", msg.Err))
			m.setLastError(msg.Err)
			m.refreshViewport()
			return m, nil
		}
		if msg.Feedback != nil && msg.Feedback.Action == services.MemoryActionClearSession {
			m.ui.CommandDraft = ""
		}
		m.applyMemoryFeedback(msg.Feedback)
		return m, nil

	case TurnResolvedMsg:
		return m.applyTurnResolution(msg.Resolution)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case StreamChunkMsg:
		if m.chat.Generating {
			m.chat.AppendLastMessage(msg.Content)
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
		session := m.session
		return m, func() tea.Msg {
			if session == nil {
				return MemoryFeedbackMsg{Err: context.Canceled}
			}
			feedback, err := session.RefreshMemory(context.Background())
			return MemoryFeedbackMsg{Feedback: feedback, Err: err}
		}

	case ExitMsg:
		return m, tea.Quit
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

	m.ui.CmdHistIndex = -1
	var inputCmd tea.Cmd
	m.textarea, inputCmd = m.textarea.Update(msg)
	m.refreshViewport()
	return *m, inputCmd
}

func (m *Model) handleViewportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc && m.ui.Focus == state.FocusSide {
		cmd := m.setFocus(state.FocusMain)
		m.refreshViewport()
		return *m, cmd
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
	if len(m.ui.CommandHistory) == 0 {
		return nil
	}
	if m.ui.CmdHistIndex == -1 {
		m.ui.CommandDraft = m.textarea.Value()
	}
	if m.ui.CmdHistIndex < len(m.ui.CommandHistory)-1 {
		m.ui.CmdHistIndex++
	}
	if m.ui.CmdHistIndex >= 0 && m.ui.CmdHistIndex < len(m.ui.CommandHistory) {
		m.textarea.SetValue(m.ui.CommandHistory[len(m.ui.CommandHistory)-1-m.ui.CmdHistIndex])
		m.textarea.CursorEnd()
	}
	return nil
}

func (m *Model) browseHistoryDown() tea.Cmd {
	if m.ui.CmdHistIndex > 0 {
		m.ui.CmdHistIndex--
		m.textarea.SetValue(m.ui.CommandHistory[len(m.ui.CommandHistory)-1-m.ui.CmdHistIndex])
		m.textarea.CursorEnd()
		return nil
	}
	if m.ui.CmdHistIndex == 0 {
		m.ui.CmdHistIndex = -1
		m.textarea.SetValue(m.ui.CommandDraft)
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
	m.chat.HandleStreamDone()
	m.streamChan = nil
	m.setStatusMessage("生成完成")

	session := m.session
	snapshot := m.sessionSnapshot()
	return *m, func() tea.Msg {
		if session == nil {
			return StreamErrorMsg{Err: context.Canceled}
		}
		resolution, err := session.ContinueAfterStream(context.Background(), snapshot)
		if err != nil {
			return StreamErrorMsg{Err: err}
		}
		return TurnResolvedMsg{Resolution: resolution}
	}
}

func (m *Model) handleStreamError(err error) (tea.Model, tea.Cmd) {
	m.streamChan = nil
	m.ui.ApprovalRunning = false

	m.chat.ApplyStreamError(err, time.Now())
	replacedPlaceholder := true
	if false && len(m.chat.Messages) > 0 {
		lastMsg := &m.chat.Messages[len(m.chat.Messages)-1]
		if lastMsg.Role == "assistant" && strings.TrimSpace(lastMsg.Content) == "" {
			lastMsg.Content = fmt.Sprintf("错误：%v", err)
			lastMsg.Streaming = false
			lastMsg.Error = true
			lastMsg.Transient = true
			replacedPlaceholder = true
		}
	}

	if false && !replacedPlaceholder {
		m.AddErrorMessage(fmt.Sprintf("错误：%v", err))
	}
	m.setLastError(err)
	m.setStatusMessage("")
	m.refreshViewport()
	return *m, nil
}

func (m *Model) applyTurnResolution(resolution services.TurnResolution) (tea.Model, tea.Cmd) {
	m.ui.ApprovalRunning = false
	stream := m.chat.ApplyTurnResolution(resolution, time.Now())
	m.setStatusMessage(resolution.StatusMessage)

	if stream != nil {
		m.streamChan = stream
		m.refreshViewport()
		return *m, m.streamResponseFromChannel()
	}

	m.refreshViewport()
	return *m, nil
}
