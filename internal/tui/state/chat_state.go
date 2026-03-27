package state

import "neo-code/internal/tui/services"

type Message = services.SessionViewMessage
type PendingApproval = services.SessionPendingApproval
type ChatState = services.SessionViewModel

func NewChatState(workspaceRoot string) ChatState {
	return services.NewSessionViewModel(workspaceRoot)
}
