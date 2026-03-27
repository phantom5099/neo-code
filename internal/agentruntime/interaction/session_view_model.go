package interaction

import (
	"fmt"
	"strings"
	"time"
)

type SessionViewMessage struct {
	Role      string
	Content   string
	Kind      MessageKind
	Timestamp time.Time
	Streaming bool
	Error     bool
	Transient bool
}

type SessionPendingApproval struct {
	Call     ToolCall
	ToolType string
	Target   string
}

type SessionViewModel struct {
	Messages         []SessionViewMessage
	Generating       bool
	SessionStartedAt time.Time
	ProviderName     string
	ActiveModel      string
	DefaultModel     string
	WorkspaceSummary string
	MemoryStats      MemoryStats
	WorkspaceRoot    string
	TouchedFiles     []string
	PendingApproval  *SessionPendingApproval
	APIKeyReady      bool
}

func NewSessionViewModel(workspaceRoot string) SessionViewModel {
	return SessionViewModel{
		Messages:         make([]SessionViewMessage, 0),
		SessionStartedAt: time.Now(),
		WorkspaceRoot:    strings.TrimSpace(workspaceRoot),
	}
}

func (m SessionViewModel) SessionMessages() []SessionMessage {
	result := make([]SessionMessage, 0, len(m.Messages))
	for _, msg := range m.Messages {
		result = append(result, SessionMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Kind:      msg.Kind,
			Transient: msg.Transient,
		})
	}
	return result
}

func (m SessionViewModel) Snapshot(approvalRunning bool) SessionSnapshot {
	snapshot := SessionSnapshot{
		Messages:        m.SessionMessages(),
		ActiveModel:     m.ActiveModel,
		APIKeyReady:     m.APIKeyReady,
		Generating:      m.Generating,
		ApprovalRunning: approvalRunning,
		WorkspaceRoot:   m.WorkspaceRoot,
	}
	if m.PendingApproval != nil {
		snapshot.PendingApproval = &ToolApprovalRequest{
			Call:     m.PendingApproval.Call,
			ToolType: m.PendingApproval.ToolType,
			Target:   m.PendingApproval.Target,
		}
	}
	return snapshot
}

func (m *SessionViewModel) ApplyBootstrap(data BootstrapData, now time.Time) {
	if m == nil {
		return
	}
	m.MemoryStats = data.MemoryStats
	m.APIKeyReady = data.APIKeyReady
	m.ApplySnapshot(data.Snapshot)
	if strings.TrimSpace(data.ResumeSummary.Content) != "" {
		m.Messages = append(m.Messages, SessionViewMessage{
			Role:      data.ResumeSummary.Role,
			Content:   data.ResumeSummary.Content,
			Kind:      data.ResumeSummary.Kind,
			Timestamp: now,
			Transient: true,
		})
	}
}

func (m *SessionViewModel) ApplyMessages(messages []SessionMessage, now time.Time) {
	if m == nil {
		return
	}
	for _, msg := range messages {
		m.Messages = append(m.Messages, SessionViewMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Kind:      msg.Kind,
			Timestamp: now,
			Transient: msg.Transient,
		})
	}
}

func (m *SessionViewModel) AddPlainMessage(role, content string, now time.Time) {
	if m == nil {
		return
	}
	m.Messages = append(m.Messages, SessionViewMessage{
		Role:      role,
		Content:   content,
		Kind:      MessageKindPlain,
		Timestamp: now,
	})
}

func (m *SessionViewModel) AddTransientPlainMessage(role, content string, now time.Time) {
	if m == nil {
		return
	}
	m.Messages = append(m.Messages, SessionViewMessage{
		Role:      role,
		Content:   content,
		Kind:      MessageKindPlain,
		Timestamp: now,
		Transient: true,
	})
}

func (m *SessionViewModel) AddErrorMessage(content string, now time.Time) {
	if m == nil {
		return
	}
	m.Messages = append(m.Messages, SessionViewMessage{
		Role:      "assistant",
		Content:   content,
		Kind:      MessageKindPlain,
		Timestamp: now,
		Error:     true,
		Transient: true,
	})
}

func (m *SessionViewModel) AppendLastMessage(content string) {
	if m == nil || len(m.Messages) == 0 {
		return
	}
	m.Messages[len(m.Messages)-1].Content += content
}

func (m *SessionViewModel) FinishLastMessage() {
	if m == nil || len(m.Messages) == 0 {
		return
	}
	m.Messages[len(m.Messages)-1].Streaming = false
}

func (m *SessionViewModel) StartStreaming() {
	if m == nil {
		return
	}
	m.Generating = true
	if len(m.Messages) == 0 {
		return
	}
	m.Messages[len(m.Messages)-1].Streaming = true
}

func (m *SessionViewModel) HandleStreamDone() {
	if m == nil {
		return
	}
	m.Generating = false
	m.FinishLastMessage()
}

func (m *SessionViewModel) ApplyStreamError(err error, now time.Time) {
	if m == nil {
		return
	}
	m.Generating = false

	if len(m.Messages) > 0 {
		lastMsg := &m.Messages[len(m.Messages)-1]
		if lastMsg.Role == "assistant" && strings.TrimSpace(lastMsg.Content) == "" {
			lastMsg.Content = fmt.Sprintf("错误：%v", err)
			lastMsg.Streaming = false
			lastMsg.Error = true
			lastMsg.Transient = true
			return
		}
	}

	m.AddErrorMessage(fmt.Sprintf("错误：%v", err), now)
}

func (m *SessionViewModel) ApplySnapshot(snapshot UISnapshot) {
	if m == nil {
		return
	}
	if strings.TrimSpace(snapshot.ProviderName) != "" {
		m.ProviderName = snapshot.ProviderName
	}
	if strings.TrimSpace(snapshot.CurrentModel) != "" {
		m.ActiveModel = snapshot.CurrentModel
	}
	if strings.TrimSpace(snapshot.DefaultModel) != "" {
		m.DefaultModel = snapshot.DefaultModel
	}
	m.WorkspaceSummary = snapshot.WorkspaceSummary
}

func (m *SessionViewModel) ApplyMutationFeedback(feedback *MutationFeedback, now time.Time) {
	if m == nil || feedback == nil {
		return
	}
	m.APIKeyReady = feedback.APIKeyReady
	m.ApplySnapshot(feedback.Snapshot)
	if strings.TrimSpace(feedback.AssistantMessage) == "" {
		return
	}
	if feedback.ErrorKind == MutationErrorUnsupportedProvider && len(feedback.SupportedProviders) > 0 {
		m.AddTransientPlainMessage("assistant", feedback.AssistantMessage+"\n支持的 provider：\n  - "+strings.Join(feedback.SupportedProviders, "\n  - "), now)
		return
	}
	m.AddTransientPlainMessage("assistant", feedback.AssistantMessage, now)
}

func (m *SessionViewModel) ResetSessionContext() {
	if m == nil {
		return
	}
	m.Messages = nil
	m.Generating = false
	m.WorkspaceSummary = ""
	m.TouchedFiles = nil
	m.PendingApproval = nil
}

func (m *SessionViewModel) ApplyMemoryFeedback(feedback *MemoryFeedback, now time.Time) {
	if m == nil || feedback == nil {
		return
	}
	if feedback.Action == MemoryActionClearSession {
		m.ResetSessionContext()
	}
	if feedback.Stats != nil {
		m.MemoryStats = *feedback.Stats
	}
	if strings.TrimSpace(feedback.AssistantMessage) != "" {
		m.AddTransientPlainMessage("assistant", feedback.AssistantMessage, now)
	}
}

func (m *SessionViewModel) ApplyTurnResolution(resolution TurnResolution, now time.Time) <-chan string {
	if m == nil {
		return nil
	}

	m.PendingApproval = nil
	if resolution.PendingApproval != nil {
		m.PendingApproval = &SessionPendingApproval{
			Call:     resolution.PendingApproval.Call,
			ToolType: resolution.PendingApproval.ToolType,
			Target:   resolution.PendingApproval.Target,
		}
	}
	m.TouchedFiles = MergeTouchedPaths(m.TouchedFiles, resolution.TouchedPaths...)
	m.ApplyMessages(resolution.Messages, now)

	if resolution.Stream != nil {
		m.AddPlainMessage("assistant", "", now)
		m.StartStreaming()
		return resolution.Stream
	}

	return nil
}
