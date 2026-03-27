# Repository Guidelines

## Project Structure & Module Organization
- `cmd/neocode/main.go` 是当前唯一应用入口，负责启动前准备并拉起 TUI。
- `configs/` 负责应用配置、默认路径、persona 文件加载和配置文件读写。
- `internal/provider/` 负责不同模型提供方的统一适配，例如 OpenAI 兼容协议、Anthropic、Gemini。
- `internal/tool/` 负责工具定义、注册、执行和结果封装。
- `internal/tool/security/` 负责工具安全策略加载与匹配。
- `internal/tool/protocol/` 负责工具调用协议解析与工具 schema 提示生成。
- `internal/tool/web/` 负责 webfetch / websearch 的底层 HTTP 和搜索解析实现。
- `internal/agentruntime/` 是 Agent Runtime 装配层，对外暴露统一的应用能力。
- `internal/agentruntime/chat/` 负责聊天请求模型与上下文注入后的发送编排。
- `internal/agentruntime/memory/` 负责结构化长期记忆、session memory 与召回逻辑。
- `internal/agentruntime/session/` 负责 working memory、workspace 会话快照与恢复摘要。
- `internal/agentruntime/todo/` 负责 agent 内部 todo 模型、仓储与服务。
- `internal/agentruntime/persona/` 负责按配置加载 persona prompt。
- `internal/tui/bootstrap/` 负责 TUI 启动前准备，例如工作区解析、配置初始化与程序装配。
- `internal/tui/core/` 负责 Bubble Tea 的状态流转与事件分发，只保留 UI 编排。
- `internal/tui/components/` 提供纯渲染组件。
- `internal/tui/state/` 保存 TUI 纯状态结构。
- `internal/tui/services/` 是 TUI 面向 runtime / provider / tool 的薄适配层。
- `docs/` 存放架构、契约与安全相关文档。

## Build, Test, and Development Commands
- `go build ./...` 编译所有 Go 包。
- `go test ./...` 运行所有测试。
- `go run ./cmd/neocode` 启动终端界面。
- `gofmt -w <file>` 或 `go fmt ./...` 统一格式。

## Coding Style & Naming Conventions
- 遵循惯用 Go 风格：包名短小、小写；导出标识符使用 `PascalCase`；未导出标识符使用 `camelCase`。
- 提交前执行 `gofmt`，避免手动对齐格式。
- 为导出符号补充完整句子注释。
- 避免硬编码路径、密钥、URL、阈值和环境差异项；优先通过配置、参数、环境变量或具名常量注入。
- TUI 不应直接依赖底层实现细节；优先通过 `internal/tui/services` 或 `internal/agentruntime` 暴露的稳定接口访问能力。

## Testing Guidelines
- 测试文件命名为 `*_test.go`，测试函数命名为 `TestXxx`。
- 默认使用 Go 标准库测试框架。
- 修改 `internal/provider/`、`internal/tool/`、`internal/agentruntime/` 或 `internal/tui/` 后，至少运行一次 `go test ./...`。

## Commit & Pull Request Guidelines
- 推荐使用 `feat:`、`fix:`、`docs:`、`refactor:` 等前缀。
- 涉及架构、目录、命令、配置或接口变更时，同步更新 `README.md`、`docs/`、脚本或示例配置。
- 提交前检查 `git status`、`gofmt` 和敏感信息泄露风险。

## Security & Configuration Tips
- `.env` 仅作为本地辅助配置，不应提交真实密钥。
- 默认配置文件位于 `~/.neocode/config.yaml`。
- `providers[].api_key_env` 保存的是环境变量名；为空时默认回退到 `AI_API_KEY`。
- `~/.neocode/config.yaml` 与 `~/.neocode/data/` 默认都不应提交真实运行数据。
