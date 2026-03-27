package core

import (
	"fmt"
	"strings"
	"time"

	"neo-code/internal/tui/components"
	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.ui.Width < minWindowWidth || m.ui.Height < minWindowHeight {
		return "Window too small"
	}

	m.syncLayout()
	statusBar := m.renderStatusBar()
	body := m.renderBody()
	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m Model) renderBody() string {
	mainPanel := lipgloss.NewStyle().
		Width(m.layout.mainWidth).
		Height(m.layout.bodyHeight).
		Render(m.renderMainPanel())

	if !m.layout.sideVisible {
		return mainPanel
	}

	sidePanel := lipgloss.NewStyle().
		Width(m.layout.sideWidth).
		Height(m.layout.bodyHeight).
		Render(m.sideViewport.View())

	separator := components.DimStyle.Render("|")
	return lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, separator, sidePanel)
}

func (m Model) renderMainPanel() string {
	viewportView := m.viewport
	viewportView.SetContent(m.renderChatContent())
	chatArea := lipgloss.NewStyle().
		Width(m.layout.mainWidth).
		Height(m.layout.mainContentHeight).
		Render(viewportView.View())

	inputArea := lipgloss.NewStyle().
		Width(m.layout.mainWidth).
		Render(m.renderInputArea())

	if m.ui.Mode == state.ModeHelp && m.layout.mainContentHeight >= 10 {
		chatArea = lipgloss.NewStyle().
			Width(m.layout.mainWidth).
			Height(m.layout.mainContentHeight).
			Render(components.RenderHelp(m.layout.mainWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, chatArea, inputArea)
}

func (m Model) renderStatusBar() string {
	leftStatus := "● idle"
	leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(components.ColorDim)).Bold(true)
	if m.chat.Generating {
		leftStatus = "● generating"
		leftStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(components.ColorWarning)).Bold(true)
	}
	left := fmt.Sprintf("%s %s", strings.TrimSpace(m.chat.ActiveModel), leftStyle.Render(leftStatus))

	center := fmt.Sprintf("%d msgs | %s", len(m.chat.Messages), m.mainScrollSummary())
	if status := strings.TrimSpace(m.ui.StatusMessage); status != "" {
		center += " | " + status
	} else if lastErr := strings.TrimSpace(m.ui.LastError); lastErr != "" {
		center += " | " + components.DimStyle.Render("last error: "+truncateInline(lastErr, 36))
	}

	rightParts := []string{"Enter: 发送", "Alt+Enter: 换行", "/help", "h:侧栏"}
	if !m.layout.sideVisible {
		rightParts = append(rightParts, components.DimStyle.Render("side: hidden"))
	}
	right := strings.Join(rightParts, "  ")

	return components.StatusBar{
		Left:   left,
		Center: center,
		Right:  right,
		Width:  m.ui.Width,
	}.Render()
}

func (m Model) mainScrollSummary() string {
	total := maxInt(1, m.chatLayout.ContentHeight)
	if total <= m.viewport.Height || m.viewport.Height <= 0 {
		return "100%"
	}
	visibleBottom := m.viewport.YOffset + m.viewport.Height
	if visibleBottom > total {
		visibleBottom = total
	}
	percent := int(float64(visibleBottom) * 100 / float64(total))
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	return fmt.Sprintf("%d%%", percent)
}

func (m Model) renderInputArea() string {
	return components.InputBox{
		Body:       m.textarea.View(),
		Focused:    m.ui.Focus == state.FocusInput,
		Generating: m.chat.Generating,
		Width:      m.layout.mainWidth - 2,
	}.Render()
}

func (m *Model) renderChatContent() string {
	layout := components.MessageList{Messages: m.toComponentMessages(), Width: m.viewport.Width}.RenderLayout()
	m.chatLayout = layout
	return layout.Content
}

func (m *Model) renderSideContent() string {
	var builder strings.Builder

	headerTitle := fmt.Sprintf("New session - %s", m.chat.SessionStartedAt.Format(time.RFC3339))
	builder.WriteString(components.TitleStyle.Render(truncateInline(headerTitle, m.layout.sideWidth)))
	builder.WriteString("\n")
	builder.WriteString(components.DimStyle.Render("Workspace: " + truncateInline(m.compactWorkspacePath(m.chat.WorkspaceRoot), m.layout.sideWidth)))
	builder.WriteString("\n\n")

	builder.WriteString(components.TitleStyle.Render("Context"))
	builder.WriteString("\n")
	files := m.chat.TouchedFiles
	if len(files) == 0 {
		builder.WriteString(components.DimStyle.Render("尚未涉及修改文件"))
		builder.WriteString("\n")
	} else {
		for _, file := range files {
			builder.WriteString("- ")
			builder.WriteString(truncateInline(file, m.layout.sideWidth-2))
			builder.WriteString("\n")
		}
	}
	builder.WriteString(fmt.Sprintf("Tokens ~ %d\n", services.EstimateMessageTokens(m.sessionMessages())))
	builder.WriteString(fmt.Sprintf("Model: %s\n", truncateInline(m.chat.ActiveModel, m.layout.sideWidth)))
	builder.WriteString("\n")

	builder.WriteString(components.TitleStyle.Render("LSP / 状态"))
	builder.WriteString("\n")
	summary := strings.TrimSpace(m.chat.WorkspaceSummary)
	if summary == "" {
		summary = "LSPs will activate as files are read"
	}
	builder.WriteString(renderSideBlock(summary, m.layout.sideWidth))
	builder.WriteString("\n\n")

	if m.layout.bodyHeight >= 16 {
		builder.WriteString(components.TitleStyle.Render("快捷帮助"))
		builder.WriteString("\n")
		for _, line := range m.quickHelpLines() {
			builder.WriteString(renderSideHelpLine(line, m.layout.sideWidth))
			builder.WriteString("\n")
		}
	}

	return strings.TrimRight(builder.String(), "\n")
}

func (m Model) quickHelpLines() []string {
	providerName := strings.TrimSpace(m.chat.ProviderName)
	defaultModel := strings.TrimSpace(m.chat.DefaultModel)
	if defaultModel == "" {
		defaultModel = strings.TrimSpace(services.DefaultModelForProvider(providerName))
	}

	return []string{
		"/provider <name>",
		"/switch <model>",
		"/memory",
		"/clear-context",
		fmt.Sprintf("provider: %s", firstNonEmpty(providerName, "unknown")),
		fmt.Sprintf("default model: %s", firstNonEmpty(defaultModel, m.chat.ActiveModel)),
	}
}

func renderSideHelpLine(line string, width int) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(components.ColorMutedText)).
		MaxWidth(maxInt(10, width)).
		Render(line)
}

func renderSideBlock(text string, width int) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(components.ColorMutedText)).
		MaxWidth(maxInt(10, width)).
		Render(truncateInline(text, width*3))
}

func (m Model) toComponentMessages() []components.Message {
	messages := make([]components.Message, len(m.chat.Messages))
	for i, msg := range m.chat.Messages {
		messages[i] = components.Message{
			Role:      msg.Role,
			Content:   displayMessageContent(msg, m.ui.SystemExpanded),
			Timestamp: msg.Timestamp,
			Streaming: msg.Streaming,
			Error:     msg.Error,
		}
	}
	return messages
}

func displayMessageContent(msg state.Message, showSystem bool) string {
	if msg.Role == "system" {
		if !showSystem {
			switch msg.Kind {
			case services.MessageKindResumeSummary:
				return "恢复摘要已折叠，按 ] 展开"
			default:
				return "系统上下文已折叠，按 ] 展开"
			}
		}
		return strings.TrimSpace(msg.Content)
	}
	return msg.Content
}

func truncateInline(text string, maxLen int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxLen <= 0 || len([]rune(trimmed)) <= maxLen {
		return trimmed
	}
	runes := []rune(trimmed)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
