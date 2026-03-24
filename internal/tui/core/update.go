package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go-llm-demo/configs"
	"go-llm-demo/internal/server/domain"
	"go-llm-demo/internal/server/infra/provider"
	"go-llm-demo/internal/server/infra/tools"
	"go-llm-demo/internal/tui/infra"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	toolStatusPrefix  = "[TOOL_STATUS]"
	toolContextPrefix = "[TOOL_CONTEXT]"
)

// Update 处理 Bubble Tea 事件并驱动聊天状态更新。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.SetWidth(msg.Width)
		m.SetHeight(msg.Height)
		m.syncLayout()
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		m.autoScroll = m.viewport.AtBottom()
		return m, vpCmd

	case StreamChunkMsg:
		if m.generating {
			m.AppendLastMessage(msg.Content)
			m.refreshViewport()
		}
		return m, m.streamResponseFromChannel()

	case StreamDoneMsg:
		mu := m.mutex()
		mu.Lock()
		m.generating = false
		m.streamChan = nil

		var lastContent string
		shouldCheckToolCall := !m.toolExecuting && len(m.messages) > 0
		if len(m.messages) > 0 {
			lastMsg := &m.messages[len(m.messages)-1]
			lastMsg.Streaming = false
			if lastMsg.Role == "assistant" {
				lastContent = lastMsg.Content
			} else {
				shouldCheckToolCall = false
			}
		}
		mu.Unlock()

		// 检查最后一条AI消息是否为JSON格式的工具调用
		if shouldCheckToolCall {
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(lastContent), &jsonData); err == nil {
				if toolName, ok := jsonData["tool"].(string); ok && toolName != "" {
					mu := m.mutex()
					mu.Lock()
					if m.toolExecuting {
						mu.Unlock()
						return m, nil
					}
					m.toolExecuting = true
					mu.Unlock()

					paramsMap := map[string]interface{}{}
					if toolParams, ok := jsonData["params"].(map[string]interface{}); ok {
						paramsMap = tools.NormalizeParams(toolParams)
					}

					// 显示工具执行中提示（仅用于 UI，不参与模型上下文）
					m.AddMessage("system", formatToolStatusMessage(toolName, paramsMap))

					// 在goroutine中执行工具调用
					return m, func() tea.Msg {
						call := domain.ToolCall{Tool: toolName, Params: paramsMap}
						result := tools.GlobalRegistry.Execute(call)
						if result == nil {
							mu := m.mutex()
							mu.Lock()
							m.toolExecuting = false
							mu.Unlock()
							return ToolErrorMsg{Err: fmt.Errorf("工具执行失败: 空返回")}
						}
						return ToolResultMsg{Result: result}
					}
				}
			}
		}
		m.refreshViewport()

		return m, nil

	case StreamErrorMsg:
		mu := m.mutex()
		mu.Lock()
		m.generating = false
		m.streamChan = nil
		replacedPlaceholder := false
		if len(m.messages) > 0 {
			lastMsg := &m.messages[len(m.messages)-1]
			if lastMsg.Role == "assistant" && strings.TrimSpace(lastMsg.Content) == "" {
				lastMsg.Content = fmt.Sprintf("错误: %v", msg.Err)
				lastMsg.Streaming = false
				replacedPlaceholder = true
			}
		}
		mu.Unlock()
		if !replacedPlaceholder {
			m.AddMessage("assistant", fmt.Sprintf("错误: %v", msg.Err))
		}
		m.TrimHistory(m.historyTurns)
		m.refreshViewport()
		return m, nil

	case ShowHelpMsg:
		m.mode = ModeHelp
		m.refreshViewport()
		return m, nil

	case HideHelpMsg:
		m.mode = ModeChat
		m.refreshViewport()
		return m, nil

	case RefreshMemoryMsg:
		stats, err := m.client.GetMemoryStats(context.Background())
		if err == nil && stats != nil {
			m.memoryStats = *stats
		}
		m.refreshViewport()
		return m, nil

	case RefreshTodoMsg:
		todos, err := m.client.GetTodoList(context.Background())
		if err == nil {
			m.todos = todos
		}
		m.refreshViewport()
		return m, nil

	case ExitMsg:
		return m, tea.Quit

	case ToolResultMsg:
		mu := m.mutex()
		mu.Lock()
		m.toolExecuting = false
		mu.Unlock()
		// 将结构化工具上下文添加为系统消息，然后重新获取AI响应
		m.AddMessage("system", m.formatToolContextMessage(msg.Result))
		m.AddMessage("assistant", "")
		m.generating = true
		m.refreshViewport()

		// 构建包含工具结果的消息并重新请求AI
		messages := m.buildMessages()
		return m, tea.Batch(
			m.streamResponse(messages),
			func() tea.Msg { return RefreshTodoMsg{} },
		)

	case ToolErrorMsg:
		mu := m.mutex()
		mu.Lock()
		m.toolExecuting = false
		mu.Unlock()
		// 将工具执行错误添加为结构化系统上下文
		m.AddMessage("system", m.formatToolErrorContext(msg.Err))
		m.AddMessage("assistant", "")
		m.generating = true
		m.refreshViewport()

		// 构建包含错误信息的消息并重新请求AI
		messages := m.buildMessages()
		return m, tea.Batch(
			m.streamResponse(messages),
			func() tea.Msg { return RefreshTodoMsg{} },
		)
	}

	return m, cmd
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc && m.mode == ModeHelp {
		m.mode = ModeChat
		m.refreshViewport()
		return *m, nil
	}

	switch msg.Type {
	case tea.KeyF5:
		return m.handleSubmit()

	case tea.KeyF8:
		return m.handleSubmit()

	case tea.KeyPgUp:
		m.autoScroll = false
		m.viewport.HalfViewUp()
		return *m, nil

	case tea.KeyPgDown:
		m.viewport.HalfViewDown()
		m.autoScroll = m.viewport.AtBottom()
		return *m, nil

	case tea.KeyUp:
		if strings.TrimSpace(m.textarea.Value()) == "" && len(m.commandHistory) > 0 {
			if m.cmdHistIndex < len(m.commandHistory)-1 {
				m.cmdHistIndex++
			}
			if m.cmdHistIndex >= 0 && m.cmdHistIndex < len(m.commandHistory) {
				m.textarea.SetValue(m.commandHistory[len(m.commandHistory)-1-m.cmdHistIndex])
				m.textarea.CursorEnd()
				return *m, nil
			}
		}
	case tea.KeyDown:
		if m.cmdHistIndex > 0 {
			m.cmdHistIndex--
			m.textarea.SetValue(m.commandHistory[len(m.commandHistory)-1-m.cmdHistIndex])
			m.textarea.CursorEnd()
			return *m, nil
		}
		if m.cmdHistIndex == 0 {
			m.cmdHistIndex = -1
			m.textarea.Reset()
			return *m, nil
		}
	}

	m.cmdHistIndex = -1
	var inputCmd tea.Cmd
	m.textarea, inputCmd = m.textarea.Update(msg)
	m.refreshViewport()
	if m.viewport.AtBottom() {
		m.autoScroll = true
	}
	return *m, inputCmd
}

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.textarea.SetHeight(m.calculateInputHeight())
	m.syncLayout()

	if input == "" {
		return *m, nil
	}

	switch m.mode {
	case ModeHelp:
		m.mode = ModeChat
		return *m, nil
	}

	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}
	if !m.apiKeyReady {
		m.AddMessage("assistant", "当前 API Key 未通过校验，请使用 /apikey <env_name>、/provider <name>、/switch <model> 调整配置，或 /exit 退出。")
		return *m, nil
	}

	m.AddMessage("user", input)
	m.AddMessage("assistant", "")
	// 在请求发出前先裁剪原始消息，避免 UI 历史无限扩张并影响短期上下文质量。
	m.TrimHistory(m.historyTurns)
	m.generating = true
	m.autoScroll = true
	m.refreshViewport()

	m.commandHistory = append(m.commandHistory, input)
	m.cmdHistIndex = -1

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
	if !m.apiKeyReady && !isAPIKeyRecoveryCommand(cmd) {
		m.AddMessage("assistant", "当前 API Key 未通过校验，仅支持 /apikey <env_name>、/provider <name>、/help、/switch <model>、/pwd（/workspace）或 /exit。")
		return *m, nil
	}

	switch cmd {
	case "/help":
		m.mode = ModeHelp
	case "/exit", "/quit", "/q":
		return *m, tea.Quit
	case "/apikey":
		if len(args) == 0 {
			m.AddMessage("assistant", "用法: /apikey <env_name>")
			return *m, nil
		}
		cfg := configs.GlobalAppConfig
		if cfg == nil {
			m.AddMessage("assistant", "当前配置未加载，无法切换 API Key 环境变量名")
			return *m, nil
		}
		previousEnvName := cfg.AI.APIKey
		cfg.AI.APIKey = strings.TrimSpace(args[0])
		envName := cfg.APIKeyEnvVarName()
		if cfg.RuntimeAPIKey() == "" {
			m.apiKeyReady = false
			m.AddMessage("assistant", fmt.Sprintf("环境变量 %s 未设置。请继续使用 /apikey <env_name> 切换，或 /exit 退出。", envName))
			return *m, nil
		}
		err := provider.ValidateChatAPIKey(context.Background(), cfg)
		if err == nil {
			if writeErr := configs.WriteAppConfig(m.configPath, cfg); writeErr != nil {
				cfg.AI.APIKey = previousEnvName
				m.apiKeyReady = configs.RuntimeAPIKey() != ""
				m.AddMessage("assistant", fmt.Sprintf("切换 API Key 环境变量名失败: %v", writeErr))
				return *m, nil
			}
			m.apiKeyReady = true
			m.AddMessage("assistant", fmt.Sprintf("已切换 API Key 环境变量名为 %s，并通过校验。", envName))
			return *m, nil
		}
		m.apiKeyReady = false
		if errors.Is(err, provider.ErrInvalidAPIKey) {
			m.AddMessage("assistant", fmt.Sprintf("环境变量 %s 中的 API Key 无效：%v。请继续使用 /apikey <env_name>、/provider <name>、/switch <model> 调整配置，或 /exit 退出。", envName, err))
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("环境变量 %s 的 API Key 未通过校验：%v。请继续使用 /apikey <env_name>、/provider <name>、/switch <model> 调整配置，或 /exit 退出。", envName, err))
		return *m, nil
	case "/provider":
		if len(args) == 0 {
			m.AddMessage("assistant", fmt.Sprintf("用法: /provider <name>\n可用提供商:\n  - %s", strings.Join(provider.SupportedProviders(), "\n  - ")))
			return *m, nil
		}
		cfg := configs.GlobalAppConfig
		if cfg == nil {
			m.AddMessage("assistant", "当前配置未加载，无法切换提供商")
			return *m, nil
		}
		providerName, ok := provider.NormalizeProviderName(strings.Join(args, " "))
		if !ok {
			m.AddMessage("assistant", fmt.Sprintf("不支持的提供商: %s\n可用提供商:\n  - %s", strings.Join(args, " "), strings.Join(provider.SupportedProviders(), "\n  - ")))
			return *m, nil
		}
		cfg.AI.Provider = providerName
		cfg.AI.Model = provider.DefaultModelForProvider(providerName)
		m.activeModel = cfg.AI.Model
		if writeErr := configs.WriteAppConfig(m.configPath, cfg); writeErr != nil {
			m.AddMessage("assistant", fmt.Sprintf("切换提供商失败: %v", writeErr))
			return *m, nil
		}
		if cfg.RuntimeAPIKey() == "" {
			m.apiKeyReady = false
			m.AddMessage("assistant", fmt.Sprintf("已切换到提供商 %s，但当前环境变量 %s 未设置。请使用 /apikey <env_name> 或设置该环境变量。", providerName, cfg.APIKeyEnvVarName()))
			return *m, nil
		}
		if err := provider.ValidateChatAPIKey(context.Background(), cfg); err == nil {
			m.apiKeyReady = true
			m.AddMessage("assistant", fmt.Sprintf("已切换到提供商 %s，当前模型已重置为默认值: %s。", providerName, cfg.AI.Model))
			return *m, nil
		} else {
			m.apiKeyReady = false
			m.AddMessage("assistant", fmt.Sprintf("已切换到提供商 %s，但 API Key 未通过校验：%v。可继续使用 /apikey <env_name>、/provider <name>、/switch <model> 调整配置。", providerName, err))
			return *m, nil
		}
	case "/switch":
		if len(args) == 0 {
			m.AddMessage("assistant", "用法: /switch <model>")
			return *m, nil
		}
		cfg := configs.GlobalAppConfig
		if cfg == nil {
			m.AddMessage("assistant", "当前配置未加载，无法切换模型")
			return *m, nil
		}
		target := strings.Join(args, " ")
		cfg.AI.Model = target
		if writeErr := configs.WriteAppConfig(m.configPath, cfg); writeErr != nil {
			m.AddMessage("assistant", fmt.Sprintf("切换模型失败: %v", writeErr))
			return *m, nil
		}
		m.activeModel = target
		if cfg.RuntimeAPIKey() == "" {
			m.apiKeyReady = false
			m.AddMessage("assistant", fmt.Sprintf("已切换到模型: %s，但当前环境变量 %s 未设置。", target, cfg.APIKeyEnvVarName()))
			return *m, nil
		}
		if err := provider.ValidateChatAPIKey(context.Background(), cfg); err == nil {
			m.apiKeyReady = true
			m.AddMessage("assistant", fmt.Sprintf("已切换到模型: %s", target))
			return *m, nil
		} else {
			m.apiKeyReady = false
			m.AddMessage("assistant", fmt.Sprintf("已切换到模型 %s，但 API Key 未通过校验：%v。", target, err))
			return *m, nil
		}
	case "/pwd", "/workspace":
		if len(args) > 0 {
			m.AddMessage("assistant", "用法: /pwd 或 /workspace")
			return *m, nil
		}
		root := strings.TrimSpace(m.workspaceRoot)
		if root == "" {
			root = tools.GetWorkspaceRoot()
		}
		if strings.TrimSpace(root) == "" {
			m.AddMessage("assistant", "当前工作区: 未知")
			return *m, nil
		}
		m.AddMessage("assistant", fmt.Sprintf("当前工作区: %s", root))
	case "/memory":
		stats, err := m.client.GetMemoryStats(context.Background())
		if err != nil {
			m.AddMessage("assistant", fmt.Sprintf("读取记忆统计失败: %v", err))
			return *m, nil
		}
		m.memoryStats = *stats
		m.AddMessage("assistant", fmt.Sprintf(
			"记忆统计:\n  长期: %d\n  会话: %d\n  总计: %d\n  TopK: %d\n  最小分数: %.2f\n  文件: %s\n  类型: %s",
			stats.PersistentItems, stats.SessionItems, stats.TotalItems, stats.TopK, stats.MinScore, stats.Path, formatTypeStats(stats.ByType),
		))
	case "/clear-memory":
		if len(args) == 0 || args[0] != "confirm" {
			m.AddMessage("assistant", "此命令会清空长期记忆。请使用 /clear-memory confirm")
			return *m, nil
		}
		if err := m.client.ClearMemory(context.Background()); err != nil {
			m.AddMessage("assistant", fmt.Sprintf("清空长期记忆失败: %v", err))
			return *m, nil
		}
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.memoryStats = *stats
		}
		m.AddMessage("assistant", "已清空本地长期记忆")
	case "/clear-context":
		if err := m.client.ClearSessionMemory(context.Background()); err != nil {
			m.AddMessage("assistant", fmt.Sprintf("清空会话记忆失败: %v", err))
			return *m, nil
		}
		m.messages = nil
		stats, _ := m.client.GetMemoryStats(context.Background())
		if stats != nil {
			m.memoryStats = *stats
		}
		m.AddMessage("assistant", "已清空当前会话上下文")
	case "/run":
		if len(args) > 0 {
			code := strings.Join(args, " ")
			return *m, tea.Batch(
				tea.Printf("\n--- 运行代码 ---\n"),
				runCodeCmd(code),
			)
		}
	case "/explain":
		if len(args) > 0 {
			code := strings.Join(args, " ")
			return *m, m.sendCodeToAI(code)
		}
		return *m, nil
	default:
		m.AddMessage("assistant", fmt.Sprintf("未知命令: %s，输入 /help 查看帮助", cmd))
	}
	m.refreshViewport()

	return *m, nil
}

func isAPIKeyRecoveryCommand(cmd string) bool {
	switch cmd {
	case "/apikey", "/provider", "/help", "/switch", "/pwd", "/workspace", "/exit", "/quit", "/q":
		return true
	default:
		return false
	}
}

func containsModel(models []string, target string) bool {
	for _, model := range models {
		if model == target {
			return true
		}
	}
	return false
}

func formatTypeStats(byType map[string]int) string {
	if len(byType) == 0 {
		return "无"
	}
	ordered := []string{
		domain.TypeUserPreference,
		domain.TypeProjectRule,
		domain.TypeCodeFact,
		domain.TypeFixRecipe,
		domain.TypeSessionMemory,
	}
	parts := make([]string, 0, len(byType))
	for _, key := range ordered {
		if count := byType[key]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", key, count))
		}
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, ", ")
}

func (m *Model) buildMessages() []infra.Message {
	mu := m.mutex()
	mu.Lock()
	defer mu.Unlock()
	result := make([]infra.Message, 0, len(m.messages))
	keepToolContextIndex := recentToolContextIndexes(m.messages, m.maxToolContextMessages)

	// 按照消息的原始时间顺序进行迭代
	for idx, msg := range m.messages {
		if msg.Role == "system" && isTransientToolStatusMessage(msg.Content) {
			continue
		}
		if msg.Role == "system" && isToolContextMessage(msg.Content) {
			content := msg.Content
			if _, ok := keepToolContextIndex[idx]; !ok {
				// 对于较旧的工具执行结果，只保留状态头信息，隐藏其具体输出以节省 Token。
				// 这样 Agent 仍然知道它已经执行过该工具及其结果。
				content = stripToolOutput(content)
			}
			result = append(result, infra.Message{
				Role:    msg.Role,
				Content: content,
			})
			continue
		}
		// 跳过空的 assistant 消息
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		// 将非空消息按其原始角色和内容添加到结果中
		result = append(result, infra.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return result
}

// stripToolOutput 移除工具上下文消息中的详细输出，仅保留工具名和执行状态。
func stripToolOutput(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		// 保留标识头、工具名、成功状态和元数据，丢弃具体的输出/错误正文。
		if strings.HasPrefix(line, toolContextPrefix) ||
			strings.HasPrefix(line, "tool=") ||
			strings.HasPrefix(line, "success=") ||
			strings.HasPrefix(line, "metadata=") {
			result = append(result, line)
		}
	}
	result = append(result, "output: (该历史输出已由系统自动压缩以节省 Token 窗口)")
	return strings.Join(result, "\n")
}

func (m *Model) streamResponse(messages []infra.Message) tea.Cmd {
	stream, err := m.client.Chat(context.Background(), messages, m.activeModel)
	if err != nil {
		return func() tea.Msg { return StreamErrorMsg{Err: err} }
	}

	m.streamChan = stream
	return func() tea.Msg {
		chunk, ok := <-stream
		if !ok {
			return StreamDoneMsg{}
		}
		return StreamChunkMsg{Content: chunk}
	}
}

func (m *Model) streamResponseFromChannel() tea.Cmd {
	if m.streamChan == nil {
		return nil
	}
	return func() tea.Msg {
		chunk, ok := <-m.streamChan
		if !ok {
			return StreamDoneMsg{}
		}
		return StreamChunkMsg{Content: chunk}
	}
}

func (m *Model) sendCodeToAI(code string) tea.Cmd {
	prompt := fmt.Sprintf("请解释以下代码：\n```\n%s\n```", code)
	m.AddMessage("user", prompt)
	m.AddMessage("assistant", "")
	m.TrimHistory(m.historyTurns)
	m.generating = true
	m.autoScroll = true
	m.refreshViewport()

	messages := m.buildMessages()
	return m.streamResponse(messages)
}

func isTransientToolStatusMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), toolStatusPrefix)
}

func isToolContextMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), toolContextPrefix)
}

func recentToolContextIndexes(messages []Message, keep int) map[int]struct{} {
	result := map[int]struct{}{}
	if keep <= 0 || len(messages) == 0 {
		return result
	}
	for i := len(messages) - 1; i >= 0 && len(result) < keep; i-- {
		msg := messages[i]
		if msg.Role == "system" && isToolContextMessage(msg.Content) {
			result[i] = struct{}{}
		}
	}
	return result
}

func formatToolStatusMessage(toolName string, params map[string]interface{}) string {
	detail := ""
	if filePath, ok := params["filePath"].(string); ok && strings.TrimSpace(filePath) != "" {
		detail = " file=" + strings.TrimSpace(filePath)
	} else if path, ok := params["path"].(string); ok && strings.TrimSpace(path) != "" {
		detail = " path=" + strings.TrimSpace(path)
	} else if workdir, ok := params["workdir"].(string); ok && strings.TrimSpace(workdir) != "" {
		detail = " workdir=" + strings.TrimSpace(workdir)
	}
	return fmt.Sprintf("%s tool=%s%s", toolStatusPrefix, strings.TrimSpace(toolName), detail)
}

func (m *Model) formatToolContextMessage(result *tools.ToolResult) string {
	if result == nil {
		return toolContextPrefix + "\n" + "tool=unknown\n" + "success=false\n" + "error:\n工具返回为空"
	}

	builder := strings.Builder{}
	builder.WriteString(toolContextPrefix)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("tool=%s\n", strings.TrimSpace(result.ToolName)))
	builder.WriteString(fmt.Sprintf("success=%t\n", result.Success))

	if len(result.Metadata) > 0 {
		if encoded, err := json.Marshal(result.Metadata); err == nil {
			builder.WriteString("metadata=")
			builder.WriteString(string(encoded))
			builder.WriteString("\n")
		}
	}

	if result.Success {
		output := strings.TrimSpace(result.Output)
		if output != "" {
			builder.WriteString("output:\n")
			builder.WriteString(truncateForContext(output, m.maxToolContextOutputSize))
		}
	} else {
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = strings.TrimSpace(result.Output)
		}
		if errText != "" {
			builder.WriteString("error:\n")
			builder.WriteString(truncateForContext(errText, m.maxToolContextOutputSize))
		}
	}

	return builder.String()
}

func (m *Model) formatToolErrorContext(err error) string {
	errText := "未知错误"
	if err != nil {
		errText = err.Error()
	}
	return toolContextPrefix + "\n" + "tool=unknown\n" + "success=false\n" + "error:\n" + truncateForContext(errText, m.maxToolContextOutputSize)
}

func truncateForContext(text string, maxLen int) string {
	trimmed := strings.TrimSpace(text)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}

	// 采用“头尾保留”策略，中间用省略号代替。
	// 这在读取代码或查看长日志时非常有用（可以看到开头和结尾的报错）。
	totalLen := len(trimmed)
	suffix := fmt.Sprintf("\n... (已截断 %d 字符，总计 %d)\n", totalLen-maxLen, totalLen)

	// 保留头部的 60% 和尾部的 40% 空间
	headLen := int(float64(maxLen-len(suffix)) * 0.6)
	tailLen := (maxLen - len(suffix)) - headLen

	if headLen < 0 {
		headLen = 0
	}
	if tailLen < 0 {
		tailLen = 0
	}

	return trimmed[:headLen] + suffix + trimmed[totalLen-tailLen:]
}

func runCodeCmd(code string) tea.Cmd {
	return func() tea.Msg {
		ext, runner := detectLanguage(code)
		if ext == "" {
			return StreamErrorMsg{Err: fmt.Errorf("无法识别代码语言")}
		}

		tmpFile, err := os.CreateTemp("", "neocode-*."+ext)
		if err != nil {
			return StreamErrorMsg{Err: fmt.Errorf("创建临时文件失败: %w", err)}
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(code); err != nil {
			return StreamErrorMsg{Err: fmt.Errorf("写入临时文件失败: %w", err)}
		}
		tmpFile.Close()

		var cmd *exec.Cmd
		if runner != "" {
			cmd = exec.Command(runner, tmpFile.Name())
		} else {
			cmd = exec.Command("go", "run", tmpFile.Name())
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			return StreamErrorMsg{Err: err}
		}

		return StreamDoneMsg{}
	}
}

func detectLanguage(code string) (string, string) {
	code = strings.TrimSpace(code)

	if strings.HasPrefix(code, "#!/bin/bash") || strings.HasPrefix(code, "#!/bin/sh") {
		return "sh", "bash"
	}
	if strings.HasPrefix(code, "package main") || strings.Contains(code, "func main()") {
		return "go", ""
	}
	if strings.HasPrefix(code, "def ") || strings.HasPrefix(code, "class ") {
		return "py", "python"
	}
	if strings.HasPrefix(code, "fn ") || strings.HasPrefix(code, "impl ") {
		return "rs", "rustc"
	}
	if strings.HasPrefix(code, "console.log") || strings.Contains(code, "=>") {
		return "js", "node"
	}

	return "", ""
}

func (m *Model) calculateInputHeight() int {
	lines := strings.Count(m.textarea.Value(), "\n") + 1
	if lines < 3 {
		return 3
	}
	if lines > 8 {
		return 8
	}
	return lines
}

func (m *Model) syncLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	inputWidth := m.width
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.textarea.SetWidth(inputWidth)
	m.textarea.SetHeight(m.calculateInputHeight())
	m.textarea.Prompt = "> "

	statusHeight := 1
	inputHeight := m.textarea.Height() + 2
	helpHeight := 0
	if m.mode == ModeHelp {
		helpHeight = minInt(20, m.height-statusHeight-3)
	}
	contentHeight := m.height - statusHeight - inputHeight - helpHeight
	if contentHeight < 3 {
		contentHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = contentHeight
}

func (m *Model) refreshViewport() {
	m.syncLayout()
	content := m.renderChatContent()
	m.viewport.SetContent(content)
	if m.autoScroll || m.viewport.AtBottom() {
		m.viewport.GotoBottom()
		m.autoScroll = true
	}
}
