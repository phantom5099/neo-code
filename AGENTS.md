# NeoCode AI Coding Agent - 系统上下文与实现指南

## 1. 项目概述
NeoCode 是一个基于 Go 和 Bubble Tea 构建的本地终端 Coding Agent MVP。它采用 ReAct / Tool-Calling 循环架构。

**核心目标:** 实现最小可行性的闭环系统：
`用户输入 -> Agent 推理 -> 调用工具 -> 获取结果 -> 继续推理 -> UI 展示`

## 2. 技术栈
* **语言:** Go (1.21+)
* **TUI 框架:** `github.com/charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss`
* **配置:** `gopkg.in/yaml.v3`
* **HTTP 客户端:** 标准库 `net/http` (用于 Provider API 调用)

## 3. 核心架构与严格边界 (必须遵守)
编写代码时，必须始终遵循以下设计原则：
1. **严格解耦:** TUI (`tui/`) **绝对不能**直接调用 Provider (`provider/`) 或执行 Tools (`tools/`)。
2. **Runtime 为调度中心:** UI、模型和工具之间的所有通信都必须通过 Agent Runtime (`runtime/`) 路由。
3. **面向接口编程:** 依赖于接口 (`Provider`, `Tool`, `Runtime`)，而不是具体的实现。
4. **流式通信 (Streaming):** Provider 接收到流式 chunk 后，通过 Go Channel (`chan RuntimeEvent`) 发送 `agent_chunk` 事件给 Runtime，Runtime 将其透传给 TUI 渲染。必须妥善处理 Channel 的生命周期和并发安全。
5. **Tool Call 职责划分:** Provider 负责将各大 API（如 OpenAI）特有的 Tool Call 格式解析并转换为系统标准的 `[]ToolCall` 结构体。Runtime 检查此结构体，若不为空，则中断当前对话流，转而执行工具。
6. **上下文管理:** Runtime 需实现简单的滑动窗口机制（例如保留 System Prompt，外加最近 10 轮的对话和工具结果），防止 Token 溢出。
7. **配置管理与并发安全:** Config 必须通过 `ConfigManager` 进行管理，使用 `sync.RWMutex` 保护读写。TUI (`tui/`) 可以调用 `Update()` 修改当前选中模型，Runtime (`runtime/`) 在每次发起请求前，必须通过 `Get()` 获取最新配置。
8. **Provider 动态构建:** Runtime 不应持有静态的 Provider 实例。应实现一个 `ProviderFactory`，根据当前 Config 动态实例化对应的 API Client，以支持对话中途无缝切换模型。
9. **密钥安全:** `config.yaml` 中仅允许配置 API Key 的环境变量名 (`api_key_env`)，程序运行时需使用 `os.Getenv` 获取真实密钥，禁止在日志或 UI 中明文打印 API Key。
10. **防腐层设计 (Anti-Corruption Layer):** Provider 模块必须作为防腐层存在。Runtime (`runtime/`) 只能使用 `provider` 包定义的标准 `Message` 和 `ToolCall` 结构体。绝不允许在 Runtime 层出现任何特定于 OpenAI 或 Anthropic 的数据结构（如 `tool_use` 块或特定的 `role` 转换逻辑）。
11. **Tool Role 差异抹平:** 针对工具结果回灌，Runtime 统一使用 `Role: "tool"` 构造历史消息。`provider/anthropic` 必须在内部将连续的 `Role: "tool"` 消息自动转换并合并为 Anthropic API 要求的 `Role: "user", Type: "tool_result"` 格式，确保 API 不报 400 错误。
12. **流式 ToolCall 拼接:** Provider 适配器负责在内部完成流式 Tool Call 碎片的拼接。只有当获取到完整的 JSON 参数字符串后，才允许向 Runtime 返回结构化的 `[]ToolCall`。
13. **会话持久化:** MVP 阶段使用纯 JSON 文件进行 Session 存储，默认路径为 `~/.neocode/sessions/`。必须通过抽象接口 `SessionStore` 进行读写。
14. **分离加载策略:** 为保证 TUI 性能，侧边栏列表必须使用轻量级的 `SessionSummary` 进行渲染。只有当用户明确选中并进入某个会话时，才允许将该会话的完整 `[]Message` 读取到内存中。
15. **磁盘写入时机:** 严禁在 TUI 输入流或频繁的 UI 刷新期间进行磁盘 I/O。`SessionStore.Save()` 只能在完整的业务节点触发（如用户发送消息后、模型完整回复后、工具执行结束后）。

## 4. MVP 第一阶段实现计划
严格聚焦于 Phase 1，不要过度设计 Phase 2/3 的功能。
* **步骤 1:** 实现 `config` (YAML 加载、结构体验证)。
* **步骤 2:** 实现 `Provider` 接口及一个具体适配器 (如 OpenAI 兼容接口，需支持流式解析与 Tool Call 结构化)。
* **步骤 3:** 实现 `Tool` 接口、`Registry` 注册表及一个具体工具 (如 `filesystem.read_file`)。
* **步骤 4:** 实现 `Agent Runtime` (核心循环、滑动窗口状态管理、事件总线 Channel)。
* **步骤 5:** 实现 `TUI` (基础聊天输入/输出视图，对接 Runtime 的事件通道)。

## 5. 编码规范
* **错误处理:** 绝不吞咽错误。使用上下文包装错误 (例如：`fmt.Errorf("executing tool %s: %w", toolName, err)`)。
* **并发控制:** 所有 Provider 和 Tool 调用必须传入 `context.Context`，以支持超时和用户取消。TUI 事件循环与 Agent Runtime 循环并发运行，需通过 Channel 或 Bubble Tea 的 `Cmd` 机制安全同步。
* **简洁性:** 编写符合 Go 习惯的代码。尽可能使用标准库。

## 6. 目录结构
确保所有生成的代码文件放置在以下约定的目录树中：
```text
.
├── cmd/neocode/main.go
├── internal/
│   ├── app/bootstrap.go
│   ├── config/loader.go, model.go, validate.go
│   ├── provider/provider.go, openai/openai.go
│   ├── runtime/runtime.go, executor.go, prompt_builder.go, session.go, events.go
│   ├── tools/registry.go, types.go, filesystem/fs.go
│   └── tui/app.go, state.go, keymap.go, views/, components/