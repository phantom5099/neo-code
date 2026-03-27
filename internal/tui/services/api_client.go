package services

import "neo-code/internal/agentruntime/interaction"

type Message = interaction.Message
type ChatClient = interaction.ChatClient
type WorkingSessionSummaryProvider = interaction.WorkingSessionSummaryProvider
type MemoryStats = interaction.MemoryStats

func NewLocalChatClient() (ChatClient, error) {
	return interaction.NewLocalChatClient()
}
