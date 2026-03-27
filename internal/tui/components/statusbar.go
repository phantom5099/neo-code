package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StatusBar struct {
	Left   string
	Center string
	Right  string
	Width  int
}

func (s StatusBar) Render() string {
	left := lipgloss.NewStyle().Bold(true).Render(strings.TrimSpace(s.Left))
	center := lipgloss.NewStyle().Bold(true).Render(strings.TrimSpace(s.Center))
	right := lipgloss.NewStyle().Bold(true).Render(strings.TrimSpace(s.Right))

	totalWidth := s.Width
	if totalWidth <= 0 {
		totalWidth = lipgloss.Width(left) + lipgloss.Width(center) + lipgloss.Width(right) + 4
	}

	remaining := totalWidth - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	if remaining < 2 {
		remaining = 2
	}
	leftGap := remaining / 2
	rightGap := remaining - leftGap

	return left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right
}
