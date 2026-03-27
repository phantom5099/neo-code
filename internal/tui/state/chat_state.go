package state

import (
	"time"

	"neo-code/internal/tui/services"
)

type Message struct {
	Role      string
	Content   string
	Kind      services.MessageKind
	Timestamp time.Time
	Streaming bool
	Error     bool
	Transient bool
}

type PendingApproval struct {
	Call     services.ToolCall
	ToolType string
	Target   string
}

type ChatState struct {
	Messages         []Message
	Generating       bool
	SessionStartedAt time.Time
	ProviderName     string
	ActiveModel      string
	DefaultModel     string
	WorkspaceSummary string
	MemoryStats      services.MemoryStats
	WorkspaceRoot    string
	TouchedFiles     []string
	PendingApproval  *PendingApproval
	APIKeyReady      bool
}
