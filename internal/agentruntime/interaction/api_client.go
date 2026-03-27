package interaction

import (
	"neo-code/internal/agentruntime"
	"neo-code/internal/agentruntime/chat"
	"neo-code/internal/agentruntime/memory"
)

type Message = chat.Message
type ChatClient = agentruntime.ChatClient
type WorkingSessionSummaryProvider = agentruntime.WorkingSessionSummaryProvider
type MemoryStats = memory.MemoryStats

func NewLocalChatClient() (ChatClient, error) {
	return agentruntime.NewLocalChatClient()
}
