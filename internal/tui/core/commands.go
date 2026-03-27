package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/tui/services"
	"neo-code/internal/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	updateAPIKey = func(ctx context.Context, controller services.Controller, envName string) (*services.MutationFeedback, error) {
		return controller.UpdateAPIKey(ctx, envName)
	}
	switchProvider = func(ctx context.Context, controller services.Controller, providerName string) (*services.MutationFeedback, error) {
		return controller.SwitchProvider(ctx, providerName)
	}
	switchModel = func(ctx context.Context, controller services.Controller, model string) (*services.MutationFeedback, error) {
		return controller.SwitchModel(ctx, model)
	}
	loadMemoryStats = func(ctx context.Context, controller services.Controller) (*services.MemoryFeedback, error) {
		return controller.LoadMemoryStats(ctx)
	}
	clearPersistentMemory = func(ctx context.Context, controller services.Controller) (*services.MemoryFeedback, error) {
		return controller.ClearPersistentMemory(ctx)
	}
	clearSessionContext = func(ctx context.Context, controller services.Controller) (*services.MemoryFeedback, error) {
		return controller.ClearSessionContext(ctx)
	}
	resolveApproval = func(ctx context.Context, controller services.Controller, req services.ConversationRequest, pending services.ToolApprovalRequest, decision services.ApprovalDecision) (services.TurnResolution, error) {
		return controller.ResolveApproval(ctx, req, pending, decision)
	}
	resolveAssistantTurn = func(ctx context.Context, controller services.Controller, req services.ConversationRequest, assistantContent string) (services.TurnResolution, error) {
		return controller.ResolveAssistantTurn(ctx, req, assistantContent)
	}
)

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.textarea.SetHeight(m.calculateInputHeight())
	m.syncLayout()

	if input == "" {
		return *m, nil
	}

	if m.ui.Mode == state.ModeHelp {
		m.ui.Mode = state.ModeChat
		return *m, nil
	}

	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}
	if m.chat.Generating {
		m.setStatusMessage("Generating...")
		return *m, nil
	}
	if !m.chat.APIKeyReady {
		m.AddErrorMessage("当前 API Key 不可用，请使用 /apikey、/provider 或 /switch 调整配置。")
		m.setLastError(errors.New("api key unavailable"))
		return *m, nil
	}
	if m.chat.PendingApproval != nil {
		m.AddErrorMessage("当前有待确认的安全审批，请先使用 /y 或 /n 处理。")
		return *m, nil
	}

	m.AddMessage("user", input)
	m.AddMessage("assistant", "")
	m.chat.Generating = true
	m.ui.AutoScroll = true
	m.refreshViewport()

	m.ui.CommandHistory = append(m.ui.CommandHistory, input)
	m.ui.CmdHistIndex = -1
	m.ui.CommandDraft = ""
	m.setStatusMessage("")

	return *m, m.streamResponse()
}

func (m *Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return *m, nil
	}

	cmd := fields[0]
	args := fields[1:]
	if !m.chat.APIKeyReady && !isAPIKeyRecoveryCommand(cmd) {
		m.AddTransientMessage("assistant", "当前 API Key 不可用。现在仅可使用 /apikey、/provider、/switch、/help、/pwd(/workspace) 和 /exit。")
		m.refreshViewport()
		return *m, nil
	}

	switch cmd {
	case "/help":
		m.ui.Mode = state.ModeHelp
		m.refreshViewport()
		return *m, nil

	case "/y":
		if len(args) > 0 {
			m.AddTransientMessage("assistant", "用法：/y")
			m.refreshViewport()
			return *m, nil
		}
		if m.chat.PendingApproval == nil {
			m.AddTransientMessage("assistant", "当前没有待确认的安全审批。")
			m.refreshViewport()
			return *m, nil
		}
		if m.ui.ApprovalRunning {
			m.AddTransientMessage("assistant", "另一个工具仍在运行，请稍后再试 /y。")
			m.refreshViewport()
			return *m, nil
		}

		pending := *m.chat.PendingApproval
		m.chat.PendingApproval = nil
		m.ui.ApprovalRunning = true
		m.setStatusMessage("Running tool...")
		m.refreshViewport()
		return *m, func() tea.Msg {
			resolution, err := resolveApproval(context.Background(), m.controller, m.conversationRequest(), services.ToolApprovalRequest{
				Call:             pending.Call,
				ToolType:         pending.ToolType,
				Target:           pending.Target,
				AssistantMessage: "",
			}, services.ApprovalDecisionApprove)
			if err != nil {
				return StreamErrorMsg{Err: err}
			}
			return TurnResolvedMsg{Resolution: resolution}
		}

	case "/n":
		if len(args) > 0 {
			m.AddTransientMessage("assistant", "用法：/n")
			m.refreshViewport()
			return *m, nil
		}
		if m.chat.PendingApproval == nil {
			m.AddTransientMessage("assistant", "当前没有待确认的安全审批。")
			m.refreshViewport()
			return *m, nil
		}
		pending := *m.chat.PendingApproval
		m.chat.PendingApproval = nil
		return *m, func() tea.Msg {
			resolution, err := resolveApproval(context.Background(), m.controller, m.conversationRequest(), services.ToolApprovalRequest{
				Call:             pending.Call,
				ToolType:         pending.ToolType,
				Target:           pending.Target,
				AssistantMessage: "",
			}, services.ApprovalDecisionReject)
			if err != nil {
				return StreamErrorMsg{Err: err}
			}
			return TurnResolvedMsg{Resolution: resolution}
		}

	case "/exit", "/quit", "/q":
		return *m, tea.Quit

	case "/apikey":
		if len(args) == 0 {
			m.AddTransientMessage("assistant", "用法：/apikey <env_name>")
			m.refreshViewport()
			return *m, nil
		}
		return *m, func() tea.Msg {
			feedback, err := updateAPIKey(context.Background(), m.controller, args[0])
			return MutationFeedbackMsg{Feedback: feedback, Err: err}
		}

	case "/provider":
		if len(args) == 0 {
			m.AddTransientMessage("assistant", fmt.Sprintf("用法：/provider <name>\n支持的 provider：\n  - %s", strings.Join(services.SupportedProviders(), "\n  - ")))
			m.refreshViewport()
			return *m, nil
		}
		target := strings.Join(args, " ")
		return *m, func() tea.Msg {
			feedback, err := switchProvider(context.Background(), m.controller, target)
			return MutationFeedbackMsg{Feedback: feedback, Err: err}
		}

	case "/switch":
		if len(args) == 0 {
			m.AddTransientMessage("assistant", "用法：/switch <model>")
			m.refreshViewport()
			return *m, nil
		}
		target := strings.Join(args, " ")
		return *m, func() tea.Msg {
			feedback, err := switchModel(context.Background(), m.controller, target)
			return MutationFeedbackMsg{Feedback: feedback, Err: err}
		}

	case "/pwd", "/workspace":
		if len(args) > 0 {
			m.AddTransientMessage("assistant", "用法：/pwd 或 /workspace")
			m.refreshViewport()
			return *m, nil
		}
		root := strings.TrimSpace(m.chat.WorkspaceRoot)
		if root == "" {
			m.AddTransientMessage("assistant", "当前工作区：unknown")
		} else {
			m.AddTransientMessage("assistant", fmt.Sprintf("当前工作区：%s", root))
		}
		m.refreshViewport()
		return *m, nil

	case "/memory":
		return *m, func() tea.Msg {
			feedback, err := loadMemoryStats(context.Background(), m.controller)
			return MemoryFeedbackMsg{Feedback: feedback, Err: err}
		}

	case "/clear-memory":
		if len(args) == 0 || args[0] != "confirm" {
			m.AddTransientMessage("assistant", "该命令会清空持久记忆，请使用 /clear-memory confirm。")
			m.refreshViewport()
			return *m, nil
		}
		return *m, func() tea.Msg {
			feedback, err := clearPersistentMemory(context.Background(), m.controller)
			return MemoryFeedbackMsg{Feedback: feedback, Err: err}
		}

	case "/clear-context":
		return *m, func() tea.Msg {
			feedback, err := clearSessionContext(context.Background(), m.controller)
			return MemoryFeedbackMsg{Feedback: feedback, Err: err}
		}

	default:
		m.AddTransientMessage("assistant", fmt.Sprintf("未知命令：%s。输入 /help 查看可用命令。", cmd))
		m.refreshViewport()
		return *m, nil
	}
}

func (m *Model) applyMutationFeedback(feedback *services.MutationFeedback) {
	if feedback == nil {
		return
	}
	m.chat.APIKeyReady = feedback.APIKeyReady
	m.applySnapshot(feedback.Snapshot)
	if strings.TrimSpace(feedback.AssistantMessage) != "" {
		if feedback.ErrorKind == services.MutationErrorUnsupportedProvider && len(feedback.SupportedProviders) > 0 {
			m.AddTransientMessage("assistant", feedback.AssistantMessage+"\n支持的 provider：\n  - "+strings.Join(feedback.SupportedProviders, "\n  - "))
		} else {
			m.AddTransientMessage("assistant", feedback.AssistantMessage)
		}
	}
	m.setStatusMessage(feedback.StatusMessage)
	if feedback.ValidationErr != nil {
		m.setLastError(feedback.ValidationErr)
	}
	m.refreshViewport()
}

func (m *Model) applyMemoryFeedback(feedback *services.MemoryFeedback) {
	if feedback == nil {
		return
	}
	if feedback.Stats != nil {
		m.chat.MemoryStats = *feedback.Stats
	}
	if strings.TrimSpace(feedback.AssistantMessage) != "" {
		m.AddTransientMessage("assistant", feedback.AssistantMessage)
	}
	m.setStatusMessage(feedback.StatusMessage)
	m.refreshViewport()
}

func (m *Model) applySnapshot(snapshot services.UISnapshot) {
	if strings.TrimSpace(snapshot.ProviderName) != "" {
		m.chat.ProviderName = snapshot.ProviderName
	}
	if strings.TrimSpace(snapshot.CurrentModel) != "" {
		m.chat.ActiveModel = snapshot.CurrentModel
	}
	if strings.TrimSpace(snapshot.DefaultModel) != "" {
		m.chat.DefaultModel = snapshot.DefaultModel
	}
	m.chat.WorkspaceSummary = snapshot.WorkspaceSummary
}

func isAPIKeyRecoveryCommand(cmd string) bool {
	switch cmd {
	case "/apikey", "/provider", "/help", "/switch", "/pwd", "/workspace", "/y", "/n", "/exit", "/quit", "/q":
		return true
	default:
		return false
	}
}
