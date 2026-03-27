package services

import "neo-code/internal/agentruntime/interaction"

type SessionService = interaction.SessionService
type SessionSnapshot = interaction.SessionSnapshot
type InputResult = interaction.InputResult

func NewSessionService(controller Controller) SessionService {
	return interaction.NewSessionService(controller)
}
