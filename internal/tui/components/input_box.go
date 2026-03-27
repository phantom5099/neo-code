package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type InputBox struct {
	Body       string
	Focused    bool
	Generating bool
	Width      int
}

func (i InputBox) Render() string {
	title := "输入"
	if i.Generating {
		title = "输入 · Generating..."
	}
	titleStyle := TitleStyle
	if !i.Focused {
		titleStyle = DimStyle
	}

	width := i.Width
	if width < 12 {
		width = 12
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDim)).
		Padding(0, 1).
		Width(width).
		Render(strings.TrimRight(i.Body, "\n"))

	return titleStyle.Render(title) + "\n" + box
}
