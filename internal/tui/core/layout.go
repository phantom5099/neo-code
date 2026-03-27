package core

import (
	"fmt"
	"strings"

	"neo-code/internal/tui/components"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) chatContentPosition(msg tea.MouseMsg) (int, int, bool) {
	statusHeight := 1
	chatTop := statusHeight
	chatBottom := chatTop + m.viewport.Height
	if msg.Y < chatTop || msg.Y >= chatBottom {
		return 0, 0, false
	}
	return m.viewport.YOffset + (msg.Y - chatTop), msg.X, true
}

func findClickableRegion(regions []components.ClickableRegion, row, col int) (components.ClickableRegion, bool) {
	for _, region := range regions {
		if row < region.StartRow || row > region.EndRow {
			continue
		}
		if col < region.StartCol || col > region.EndCol {
			continue
		}
		return region, true
	}
	return components.ClickableRegion{}, false
}

func (m *Model) copyCodeBlock(ref components.CodeBlockRef) error {
	if m.copyToClipboard == nil {
		return fmt.Errorf("clipboard unavailable")
	}
	return m.copyToClipboard(ref.Code)
}

func (m *Model) calculateInputHeight() int {
	lines := strings.Count(m.textarea.Value(), "\n") + 1
	if lines < 3 {
		return 3
	}
	if lines > 8 {
		return 8
	}
	return lines
}

func (m *Model) syncLayout() {
	if m.ui.Width <= 0 || m.ui.Height <= 0 {
		return
	}

	inputWidth := m.ui.Width
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.textarea.SetWidth(inputWidth)
	m.textarea.SetHeight(m.calculateInputHeight())
	m.textarea.Prompt = "> "

	statusHeight := 1
	inputHeight := m.textarea.Height() + 2
	helpHeight := 0
	if m.ui.Mode == state.ModeHelp {
		helpHeight = minInt(20, m.ui.Height-statusHeight-3)
	}
	contentHeight := m.ui.Height - statusHeight - inputHeight - helpHeight
	if contentHeight < 3 {
		contentHeight = 3
	}

	m.viewport.Width = m.ui.Width
	m.viewport.Height = contentHeight
}

func (m *Model) refreshViewport() {
	m.syncLayout()
	content := m.renderChatContent()
	m.viewport.SetContent(content)
	if m.ui.AutoScroll || m.viewport.AtBottom() {
		m.viewport.GotoBottom()
		m.ui.AutoScroll = true
	}
}
