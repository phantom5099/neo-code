package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"neo-code/internal/tui/components"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	minWindowWidth          = 20
	minWindowHeight         = 6
	sidebarBreakpoint       = 96
	minSidebarWidth         = 28
	sidebarRatioNumerator   = 30
	sidebarRatioDenominator = 100
)

type viewLayout struct {
	bodyHeight        int
	mainWidth         int
	sideWidth         int
	sideVisible       bool
	mainX             int
	sideX             int
	contentTop        int
	mainContentHeight int
	inputTop          int
	inputHeight       int
	statusHeight      int
}

func (m *Model) chatContentPosition(msg tea.MouseMsg) (int, int, bool) {
	if !m.layout.sideVisible && msg.X >= m.layout.mainWidth {
		return 0, 0, false
	}
	if msg.X < m.layout.mainX || msg.X >= m.layout.mainX+m.layout.mainWidth {
		return 0, 0, false
	}
	if msg.Y < m.layout.contentTop || msg.Y >= m.layout.contentTop+m.layout.mainContentHeight {
		return 0, 0, false
	}
	return m.viewport.YOffset + (msg.Y - m.layout.contentTop), msg.X - m.layout.mainX, true
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

	statusHeight := 1
	bodyHeight := m.ui.Height - statusHeight
	if bodyHeight < 4 {
		bodyHeight = 4
	}

	sideVisible := m.shouldShowSide()
	sideWidth := 0
	mainWidth := m.ui.Width
	if sideVisible {
		sideWidth = maxInt(minSidebarWidth, m.ui.Width*sidebarRatioNumerator/sidebarRatioDenominator)
		if sideWidth > m.ui.Width/2 {
			sideWidth = m.ui.Width / 2
		}
		if sideWidth >= m.ui.Width-20 {
			sideWidth = minSidebarWidth
		}
		mainWidth = m.ui.Width - sideWidth - 1
	}
	if mainWidth < 20 {
		mainWidth = 20
	}

	inputWidth := maxInt(18, mainWidth-2)
	m.textarea.SetWidth(inputWidth)
	m.textarea.SetHeight(m.calculateInputHeight())
	m.textarea.Prompt = "> "

	inputHeight := countLines(m.renderInputArea())
	if inputHeight < 4 {
		inputHeight = 4
	}
	mainContentHeight := bodyHeight - inputHeight
	if mainContentHeight < 3 {
		mainContentHeight = 3
	}

	m.layout = viewLayout{
		bodyHeight:        bodyHeight,
		mainWidth:         mainWidth,
		sideWidth:         sideWidth,
		sideVisible:       sideVisible,
		mainX:             0,
		sideX:             mainWidth + 1,
		contentTop:        0,
		mainContentHeight: mainContentHeight,
		inputTop:          mainContentHeight,
		inputHeight:       inputHeight,
		statusHeight:      statusHeight,
	}

	m.viewport.Width = mainWidth
	m.viewport.Height = mainContentHeight
	m.sideViewport.Width = maxInt(0, sideWidth)
	m.sideViewport.Height = bodyHeight
}

func (m *Model) refreshViewport() {
	m.syncLayout()
	content := m.renderChatContent()
	m.viewport.SetContent(content)
	if m.ui.AutoScroll || m.viewport.AtBottom() {
		m.viewport.GotoBottom()
		m.ui.AutoScroll = true
	}

	sideContent := m.renderSideContent()
	m.sideViewport.SetContent(sideContent)
}

func (m Model) shouldShowSide() bool {
	if m.ui.Width >= sidebarBreakpoint {
		return !m.ui.SideCollapsed
	}
	return m.ui.SideNarrowOpen && !m.ui.SideCollapsed
}

func (m *Model) setFocus(target state.FocusTarget) tea.Cmd {
	m.ui.Focus = target
	if target == state.FocusInput {
		return m.textarea.Focus()
	}
	m.textarea.Blur()
	return nil
}

func (m *Model) focusNext() tea.Cmd {
	order := []state.FocusTarget{state.FocusInput, state.FocusMain}
	if m.shouldShowSide() {
		order = append(order, state.FocusSide)
	}
	current := 0
	for i, target := range order {
		if target == m.ui.Focus {
			current = i
			break
		}
	}
	return m.setFocus(order[(current+1)%len(order)])
}

func (m *Model) focusPrev() tea.Cmd {
	order := []state.FocusTarget{state.FocusInput, state.FocusMain}
	if m.shouldShowSide() {
		order = append(order, state.FocusSide)
	}
	current := 0
	for i, target := range order {
		if target == m.ui.Focus {
			current = i
			break
		}
	}
	return m.setFocus(order[(current-1+len(order))%len(order)])
}

func (m Model) compactWorkspacePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "unknown"
	}

	root := strings.TrimSpace(m.chat.WorkspaceRoot)
	if root == "" {
		root = strings.TrimSpace(getWorkspaceRoot())
	}

	if root != "" {
		if rel, err := filepath.Rel(root, trimmed); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "./" + filepath.ToSlash(rel)
		}
		if samePath(root, trimmed) {
			return "."
		}
	}

	if home, err := filepath.Abs(homeDir()); err == nil {
		if abs, absErr := filepath.Abs(trimmed); absErr == nil {
			if rel, relErr := filepath.Rel(home, abs); relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return "~/" + filepath.ToSlash(rel)
			}
			if samePath(home, abs) {
				return "~"
			}
		}
	}

	return filepath.ToSlash(trimmed)
}

func samePath(a, b string) bool {
	left := filepath.Clean(strings.TrimSpace(a))
	right := filepath.Clean(strings.TrimSpace(b))
	return strings.EqualFold(left, right)
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
