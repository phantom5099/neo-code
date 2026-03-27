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
	if !m.chat.APIKeyReady {
		m.AddMessage("assistant", "The current API Key could not be validated. Use /apikey <env_name>, /provider <name>, or /switch <model> to update the configuration, or /exit to quit.")
		return *m, nil
	}
	if m.chat.PendingApproval != nil {
		m.AddMessage("assistant", "A security approval is pending. Use /y to allow once or /n to reject before sending a new message.")
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
			m.AddMessage("assistant", fmt.Sprintf("Failed to switch the API key environment variable name: %v", err))
			return *m, nil
		}

		m.chat.APIKeyReady = result.APIKeyReady
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched the API key environment variable name to %s and validated it successfully.", result.APIKeyEnvVar))
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Environment variable %s is not set. Use /apikey <env_name> to switch to another one, or /exit to quit.", result.APIKeyEnvVar))
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrInvalidAPIKey) {
			m.AddMessage("assistant", fmt.Sprintf("The API key in environment variable %s is invalid: %v. Use /apikey <env_name>, /provider <name>, or /switch <model> to update the configuration, or /exit to quit.", result.APIKeyEnvVar, result.ValidationErr))
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("The API key in environment variable %s could not be validated: %v. Use /apikey <env_name>, /provider <name>, or /switch <model> to update the configuration, or /exit to quit.", result.APIKeyEnvVar, result.ValidationErr))
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
			m.AddMessage("assistant", fmt.Sprintf("Failed to switch provider: %v", err))
			return *m, nil
		}

		m.chat.ActiveModel = result.Model
		m.chat.APIKeyReady = result.APIKeyReady
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s. The current model was reset to the default: %s.", result.Provider, result.Model))
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s, but environment variable %s is not set. Use /apikey <env_name> or set that environment variable.", result.Provider, result.APIKeyEnvVar))
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("Switched provider to %s, but the API key could not be validated: %v. You can continue using /apikey <env_name>, /provider <name>, or /switch <model> to adjust the configuration.", result.Provider, result.ValidationErr))
		return *m, nil

	case "/switch":
		if len(args) == 0 {
			m.AddMessage("assistant", "Usage: /switch <model>")
			return *m, nil
		}
		target := strings.Join(args, " ")
		result, err := switchModelConfig(context.Background(), m.chat.ConfigPath, target)
		if err != nil {
			m.AddMessage("assistant", fmt.Sprintf("Failed to switch model: %v", err))
			return *m, nil
		}

		m.chat.ActiveModel = result.Model
		m.chat.APIKeyReady = result.APIKeyReady
		if result.ValidationErr == nil {
			m.AddMessage("assistant", fmt.Sprintf("Switched model to: %s", result.Model))
			return *m, nil
		}
		if errors.Is(result.ValidationErr, services.ErrAPIKeyMissing) {
			m.AddMessage("assistant", fmt.Sprintf("Switched model to %s, but environment variable %s is not set.", result.Model, result.APIKeyEnvVar))
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("Switched model to %s, but the API key could not be validated: %v.", result.Model, result.ValidationErr))
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
			m.AddMessage("assistant", fmt.Sprintf("Failed to read memory stats: %v", err))
			return *m, nil
		}
		m.chat.MemoryStats = *stats
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
			m.AddMessage("assistant", fmt.Sprintf("Failed to clear persistent memory: %v", err))
			return *m, nil
		}
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.chat.MemoryStats = *stats
		}
		m.AddMessage("assistant", "Cleared local persistent memory")

	case "/clear-context":
		if err := m.client.ClearSessionMemory(context.Background()); err != nil {
			m.AddMessage("assistant", fmt.Sprintf("Failed to clear session memory: %v", err))
			return *m, nil
		}
		m.chat.Messages = nil
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.chat.MemoryStats = *stats
		}
		m.AddMessage("assistant", "Cleared the current session context")

	default:
		m.AddMessage("assistant", fmt.Sprintf("Unknown command: %s. Enter /help to view the available commands.", cmd))
	}

	m.refreshViewport()
	return *m, nil
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
