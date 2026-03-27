package state

import (
	"time"

	"neo-code/internal/tui/services"
)

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Streaming bool
	Error     bool
}

type PendingApproval struct {
	Call     services.ToolCall
	ToolType string
	Target   string
}

type ChatState struct {
	Messages         []Message
	HistoryTurns     int
	Generating       bool
	SessionStartedAt time.Time
	ProviderName     string
	ActiveModel      string
	DefaultModel     string
	WorkspaceSummary string
	MemoryStats      services.MemoryStats
	CommandHistory   []string
	CmdHistIndex     int
	CommandDraft     string
	WorkspaceRoot    string
	TouchedFiles     []string
	ToolExecuting    bool
	PendingApproval  *PendingApproval
	APIKeyReady      bool
	ConfigPath       string
}
