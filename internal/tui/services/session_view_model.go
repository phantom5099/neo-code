package services

import "neo-code/internal/agentruntime/interaction"

type SessionViewMessage = interaction.SessionViewMessage
type SessionPendingApproval = interaction.SessionPendingApproval
type SessionViewModel = interaction.SessionViewModel

func NewSessionViewModel(workspaceRoot string) SessionViewModel {
	return interaction.NewSessionViewModel(workspaceRoot)
}
