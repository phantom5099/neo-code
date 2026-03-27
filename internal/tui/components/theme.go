package components

import "github.com/charmbracelet/lipgloss"

const (
	ColorTitle     = "#61AFEF"
	ColorDim       = "#5C6370"
	ColorSelection = "#C678DD"
	ColorSuccess   = "#98C379"
	ColorWarning   = "#E5C07B"
	ColorError     = "#E06C75"
	ColorText      = "#E6EAF2"
	ColorMutedText = "#AAB2C0"
	ColorPanel     = "#282C34"
	ColorPanelAlt  = "#21252B"
)

var (
	TitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTitle)).Bold(true)
	DimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
)
