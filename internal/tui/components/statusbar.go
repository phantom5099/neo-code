package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type StatusBar struct {
	Model      string
	MemoryCnt  int
	Generating bool
	Width      int
}

func (s StatusBar) Render() string {
	modelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98C379")).
		Background(lipgloss.Color("#282C34")).
		Padding(0, 1)

	memStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C678DD")).
		Background(lipgloss.Color("#282C34")).
		Padding(0, 1)

	statusText := "Idle"
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Background(lipgloss.Color("#282C34")).
		Padding(0, 1)
	if s.Generating {
		statusText = "Generating"
		statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Background(lipgloss.Color("#282C34")).
			Padding(0, 1)
	}

	timeStr := time.Now().Format("15:04")
	timestampStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Background(lipgloss.Color("#282C34")).
		Padding(0, 1)

	modelText := modelStyle.Render(s.Model)
	memText := memStyle.Render(fmt.Sprintf("Memory: %d", s.MemoryCnt))
	status := statusStyle.Render(statusText)
	timestamp := timestampStyle.Render(timeStr)

	left := strings.Join([]string{modelText, memText, status}, "  ")
	padding := s.Width - lipgloss.Width(left) - lipgloss.Width(timestamp)
	if padding < 1 {
		padding = 1
	}

	return left + strings.Repeat(" ", padding) + timestamp
}
