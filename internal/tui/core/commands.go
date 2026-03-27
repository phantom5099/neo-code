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
	getWorkspaceRoot     = services.GetWorkspaceRoot
	parseAssistantTools  = services.ParseAssistantToolCalls
	executeToolCall      = services.ExecuteToolCall
	approveToolCall      = services.ApproveToolCall
	updateAPIKeyEnvVar   = services.UpdateAPIKeyEnvVar
	switchProviderConfig = services.SwitchProvider
	switchModelConfig    = services.SwitchModel
	readUISnapshot       = services.ReadUISnapshot
)

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.textarea.SetHeight(m.calculateInputHeight())
	m.syncLayout()

	if input == "" {
		return *m, nil
	}

	switch m.ui.Mode {
	case state.ModeHelp:
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
		m.AddErrorMessage("当前 API Key 无法通过校验，请使用 /apikey、/provider 或 /switch 调整配置。")
		m.setLastError(errors.New("api key unavailable"))
		return *m, nil
	}
	if m.chat.PendingApproval != nil {
		m.AddErrorMessage("当前有待确认的安全审批，请先使用 /y 或 /n 处理。")
		return *m, nil
	}

	m.AddMessage("user", input)
	m.AddMessage("assistant", "")
	m.TrimHistory(m.chat.HistoryTurns)
	m.chat.Generating = true
	m.ui.AutoScroll = true
	m.refreshViewport()

	m.chat.CommandHistory = append(m.chat.CommandHistory, input)
	m.chat.CmdHistIndex = -1
	m.chat.CommandDraft = ""
	m.setStatusMessage("")

	messages := m.buildMessages()
	return *m, m.streamResponse(messages)
}

func (m *Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return *m, nil
	}

	cmd := fields[0]
	args := fields[1:]
	if !m.chat.APIKeyReady && !isAPIKeyRecoveryCommand(cmd) {
		m.AddMessage("assistant", "The current API Key could not be validated. Only /apikey <env_name>, /provider <name>, /help, /switch <model>, /pwd (/workspace), and /exit are available.")
		return *m, nil
	}

	switch cmd {
	case "/help":
		m.ui.Mode = state.ModeHelp

	case "/y":
		if len(args) > 0 {
			m.AddMessage("assistant", "Usage: /y")
			return *m, nil
		}
		if m.chat.PendingApproval == nil {
			m.AddMessage("assistant", "There is no pending security approval.")
			return *m, nil
		}
		if m.chat.ToolExecuting {
			m.AddMessage("assistant", "Another tool is still running. Please retry /y after it finishes.")
			return *m, nil
		}

		pending := *m.chat.PendingApproval
		m.chat.PendingApproval = nil
		if strings.TrimSpace(pending.Call.Tool) == "" {
			m.AddMessage("assistant", "The pending tool request is incomplete and cannot be executed.")
			return *m, nil
		}

		m.AddMessage("assistant", fmt.Sprintf("Approved. Running tool %s.", pending.Call.Tool))
		m.AddMessage("system", formatToolStatusMessage(pending.Call.Tool, pending.Call.Params))

		mu := m.mutex()
		mu.Lock()
		if m.chat.ToolExecuting {
			m.chat.PendingApproval = &pending
			mu.Unlock()
			return *m, nil
		}
		m.chat.ToolExecuting = true
		mu.Unlock()

		m.refreshViewport()
		return *m, func() tea.Msg {
			approveToolCall(pending.ToolType, pending.Target)
			result := executeToolCall(pending.Call)
			if result == nil {
				mu := m.mutex()
				mu.Lock()
				m.chat.ToolExecuting = false
				mu.Unlock()
				return ToolErrorMsg{Err: fmt.Errorf("tool execution failed: empty result")}
			}
			return ToolResultMsg{Result: result, Call: pending.Call}
		}

	case "/n":
		if len(args) > 0 {
			m.AddMessage("assistant", "Usage: /n")
			return *m, nil
		}
		if m.chat.PendingApproval == nil {
			m.AddMessage("assistant", "There is no pending security approval.")
			return *m, nil
		}

		pending := *m.chat.PendingApproval
		m.chat.PendingApproval = nil
		toolName := strings.TrimSpace(pending.Call.Tool)
		if toolName == "" {
			toolName = "unknown"
		}
		m.AddMessage("assistant", fmt.Sprintf("Rejected tool %s for target %s.", toolName, pending.Target))
		return *m, nil

	case "/exit", "/quit", "/q":
		return *m, tea.Quit

	case "/apikey":
		if len(args) == 0 {
			m.AddMessage("assistant", "Usage: /apikey <env_name>")
			return *m, nil
		}
		result, err := updateAPIKeyEnvVar(context.Background(), m.chat.ConfigPath, args[0])
		if err != nil {
			m.AddErrorMessage(fmt.Sprintf("切换 API Key 环境变量失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}

		m.chat.APIKeyReady = result.APIKeyReady
		m.refreshRuntimeSnapshot()
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched the API key environment variable name to %s and validated it successfully.", result.APIKeyEnvVar))
			m.setStatusMessage("已更新 API Key 环境变量")
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Environment variable %s is not set. Use /apikey <env_name> to switch to another one, or /exit to quit.", result.APIKeyEnvVar))
			m.setLastError(result.ValidationErr)
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrInvalidAPIKey) {
			m.AddMessage("assistant", fmt.Sprintf("The API key in environment variable %s is invalid: %v. Use /apikey <env_name>, /provider <name>, or /switch <model> to update the configuration, or /exit to quit.", result.APIKeyEnvVar, result.ValidationErr))
			m.setLastError(result.ValidationErr)
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("The API key in environment variable %s could not be validated: %v. Use /apikey <env_name>, /provider <name>, or /switch <model> to update the configuration, or /exit to quit.", result.APIKeyEnvVar, result.ValidationErr))
		m.setLastError(result.ValidationErr)
		return *m, nil

	case "/provider":
		if len(args) == 0 {
			m.AddMessage("assistant", fmt.Sprintf("Usage: /provider <name>\nSupported providers:\n  - %s", strings.Join(services.SupportedProviders(), "\n  - ")))
			return *m, nil
		}
		result, err := switchProviderConfig(context.Background(), m.chat.ConfigPath, strings.Join(args, " "))
		if err != nil {
			if _, ok := services.NormalizeProviderName(strings.Join(args, " ")); !ok {
				m.AddMessage("assistant", fmt.Sprintf("Unsupported provider: %s\nSupported providers:\n  - %s", strings.Join(args, " "), strings.Join(services.SupportedProviders(), "\n  - ")))
				return *m, nil
			}
			m.AddErrorMessage(fmt.Sprintf("切换 provider 失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}

		m.refreshRuntimeSnapshot()
		m.chat.ActiveModel = result.Model
		m.chat.APIKeyReady = result.APIKeyReady
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s. The current model was reset to the default: %s.", result.Provider, result.Model))
			m.setStatusMessage("provider 已切换")
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s, but environment variable %s is not set. Use /apikey <env_name> or set that environment variable.", result.Provider, result.APIKeyEnvVar))
			m.setLastError(result.ValidationErr)
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s, but the API key could not be validated: %v. You can continue using /apikey <env_name>, /provider <name>, or /switch <model> to adjust the configuration.", result.Provider, result.ValidationErr))
		m.setLastError(result.ValidationErr)
		return *m, nil

	case "/switch":
		if len(args) == 0 {
			m.AddMessage("assistant", "Usage: /switch <model>")
			return *m, nil
		}
		target := strings.Join(args, " ")
		result, err := switchModelConfig(context.Background(), m.chat.ConfigPath, target)
		if err != nil {
			m.AddErrorMessage(fmt.Sprintf("切换模型失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}

		m.refreshRuntimeSnapshot()
		m.chat.ActiveModel = result.Model
		m.chat.APIKeyReady = result.APIKeyReady
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched model to: %s", result.Model))
			m.setStatusMessage("模型已切换")
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Switched model to %s, but environment variable %s is not set.", result.Model, result.APIKeyEnvVar))
			m.setLastError(result.ValidationErr)
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("Switched model to %s, but the API key could not be validated: %v.", result.Model, result.ValidationErr))
		m.setLastError(result.ValidationErr)
		return *m, nil

	case "/pwd", "/workspace":
		if len(args) > 0 {
			m.AddMessage("assistant", "Usage: /pwd or /workspace")
			return *m, nil
		}
		root := strings.TrimSpace(m.chat.WorkspaceRoot)
		if root == "" {
			root = getWorkspaceRoot()
		}
		if strings.TrimSpace(root) == "" {
			m.AddMessage("assistant", "Current workspace: unknown")
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("Current workspace: %s", root))

	case "/memory":
		stats, err := m.client.GetMemoryStats(context.Background())
		if err != nil {
			m.AddErrorMessage(fmt.Sprintf("读取记忆统计失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}
		m.chat.MemoryStats = *stats
		m.setStatusMessage("Memory 已刷新")
		m.AddMessage("assistant", fmt.Sprintf(
			"Memory stats:\n  Persistent: %d\n  Session: %d\n  Total: %d\n  TopK: %d\n  Min score: %.2f\n  File: %s\n  Types: %s",
			stats.PersistentItems, stats.SessionItems, stats.TotalItems, stats.TopK, stats.MinScore, stats.Path, formatTypeStats(stats.ByType),
		))

	case "/clear-memory":
		if len(args) == 0 || args[0] != "confirm" {
			m.AddMessage("assistant", "This command will clear persistent memory. Use /clear-memory confirm")
			return *m, nil
		}
		if err := m.client.ClearMemory(context.Background()); err != nil {
			m.AddErrorMessage(fmt.Sprintf("清除持久记忆失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.chat.MemoryStats = *stats
		}
		m.AddMessage("assistant", "Cleared local persistent memory")
		m.setStatusMessage("已清除持久记忆")

	case "/clear-context":
		if err := m.client.ClearSessionMemory(context.Background()); err != nil {
			m.AddErrorMessage(fmt.Sprintf("清除会话上下文失败：%v", err))
			m.setLastError(err)
			return *m, nil
		}
		m.chat.Messages = nil
		m.chat.TouchedFiles = nil
		m.chat.CommandDraft = ""
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.chat.MemoryStats = *stats
		}
		m.chat.WorkspaceSummary = ""
		m.AddMessage("assistant", "Cleared the current session context")
		m.setStatusMessage("会话上下文已清空")

	default:
		m.AddMessage("assistant", fmt.Sprintf("Unknown command: %s. Enter /help to view the available commands.", cmd))
	}

	m.refreshViewport()
	return *m, nil
}

func (m *Model) refreshRuntimeSnapshot() {
	snapshot := readUISnapshot(context.Background(), m.client)
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

func formatTypeStats(byType map[string]int) string {
	if len(byType) == 0 {
		return "none"
	}

	ordered := []string{
		services.TypeUserPreference,
		services.TypeProjectRule,
		services.TypeCodeFact,
		services.TypeFixRecipe,
		services.TypeSessionMemory,
	}

	parts := make([]string, 0, len(byType))
	for _, key := range ordered {
		if count := byType[key]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", key, count))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
