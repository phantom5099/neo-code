package bootstrap

import (
	"neo-code/internal/tui/core"
	"neo-code/internal/tui/services"

	tea "github.com/charmbracelet/bubbletea"
)

func NewProgram(historyTurns int, configPath, workspaceRoot string) (*tea.Program, error) {
	client, err := services.NewLocalChatClient()
	if err != nil {
		return nil, err
	}

	model := core.NewModel(client, historyTurns, configPath, workspaceRoot)
	return tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	), nil
}
