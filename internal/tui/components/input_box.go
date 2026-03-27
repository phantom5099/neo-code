package components

import "github.com/charmbracelet/lipgloss"

type InputBox struct {
	Body       string
	Generating bool
	Status     string
}

func (i InputBox) Render() string {
	helpText := "[Enter: send | Alt+Enter: newline | PgUp/PgDn: scroll]"
	if !i.Generating {
		helpText = "[Enter: send | Alt+Enter: newline | Ctrl+V: paste | click [Copy]: copy | PgUp/PgDn: scroll]"
	}

	statusText := i.Status
	if statusText == "" {
		statusText = "Ready"
	}

	status := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF")).
		Render(statusText)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Render(helpText)

	return i.Body + "\n" + status + "\n" + footer
}
