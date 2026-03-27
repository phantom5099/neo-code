# NeoCode

NeoCode 是一个本地优先的 Go TUI AI Coding Agent。当前唯一正式入口是 `cmd/neocode`，界面层使用 Bubble Tea，配置默认位于用户目录下的 `~/.neocode/config.yaml`。

## 当前分层

- `cmd/neocode/`
  启动入口，负责 UTF-8 控制台准备、工作区准备、配置引导和拉起 TUI。
- `configs/`
  配置模型、默认路径、persona 文件加载与配置文件读写。
- `internal/provider/`
  统一适配不同模型提供方。当前支持 OpenAI 兼容协议，以及 Anthropic / Gemini 原生协议。
- `internal/tool/`
  工具定义、注册、执行和结果封装。
- `internal/tool/security/`
  工具安全策略加载和匹配。
- `internal/tool/protocol/`
  工具调用协议解析与 schema 提示生成。
- `internal/tool/web/`
  Web 工具底层 HTTP / 搜索实现。
- `internal/agentruntime/`
  Agent runtime 装配层，对外提供统一的应用能力。
- `internal/agentruntime/chat/`
  对话编排、上下文注入和 provider 调用。
- `internal/agentruntime/memory/`
  长期记忆、session memory 和召回逻辑。
- `internal/agentruntime/session/`
  working memory、workspace 会话快照和恢复摘要。
- `internal/agentruntime/todo/`
  agent 内部 todo 模型、仓储和服务。Todo 由 agent 自行规划，不再暴露给用户命令层。
- `internal/agentruntime/persona/`
  persona prompt 加载。
- `internal/tui/bootstrap/`
  TUI 启动前准备和程序装配。
- `internal/tui/core/`
  Bubble Tea 状态流转与事件分发，只保留 UI 编排。
- `internal/tui/components/`
  纯渲染组件。
- `internal/tui/state/`
  TUI 纯状态结构。
- `internal/tui/services/`
  TUI 面向 runtime / provider / tool 的薄适配层。

## 运行

```bash
go run ./cmd/neocode
```

指定工作区：

```bash
go run ./cmd/neocode --workspace ./
```

也可以通过环境变量指定：

```bash
set NEOCODE_WORKSPACE=F:\\your\\workspace
go run ./cmd/neocode
```

优先级：`--workspace` > `NEOCODE_WORKSPACE` > 当前工作目录。

## 常用开发命令

```bash
go build ./...
go test ./...
go fmt ./...
```

## TUI 交互

- `Enter`：直接发送
- `Alt+Enter`：换行
- `/provider <name>`：切换 provider
- `/switch <model>`：切换当前模型
- `/apikey <env_name>`：切换 API Key 对应的环境变量名
- `/memory`：查看记忆统计
- `/clear-memory confirm`：清空持久化记忆
- `/clear-context`：清空当前会话上下文
- `/pwd` 或 `/workspace`：查看当前工作区

说明：

- 不再保留 `/todo`、`/run`、`/explain` 这类业务命令。
- Todo 现在只作为 agent 内部规划工具存在，由 runtime 自行调用。

## 配置

默认配置文件位置：

```text
~/.neocode/config.yaml
```

默认数据目录：

```text
~/.neocode/data/
```

当前推荐配置结构：

```yaml
providers:
  - name: openai
    protocol: openai
    url: https://api.openai.com/v1/chat/completions
    model_id: gpt-5.4
    api_key_env: AI_API_KEY
  - name: anthropic
    protocol: anthropic
    url: https://api.anthropic.com/v1/messages
    model_id: claude-sonnet-4-5
    api_key_env: ANTHROPIC_API_KEY
  - name: gemini
    protocol: gemini
    url: https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse
    model_id: gemini-2.5-pro
    api_key_env: GEMINI_API_KEY

selected_provider: openai
current_model: gpt-5.4
```

补充说明：

- `api_key_env` 保存的是环境变量名，不是明文 API Key。
- 仍兼容旧的 `ai.provider / ai.model / ai.api_key` 配置字段，但新配置建议统一使用 `providers + selected_provider + current_model`。

## 工具协议

当前工具使用统一的标准 envelope：

```json
{"type":"tool_call","name":"read","arguments":{"filePath":"README.md"}}
```

兼容旧格式：

```json
{"tool":"read","params":{"filePath":"README.md"}}
```

内置工具按能力分组：

- `filesystem`：`read`、`write`、`edit`、`list`、`grep`
- `shell`：`bash`
- `web`：`webfetch`、`websearch`
- `runtime`：`todo`

## 当前说明

- 当前项目不再保留独立的 `api/proto` 契约层。
- 当前项目也没有单独的 `cmd/server`。
- 如果未来需要 HTTP / gRPC / daemon，再基于真实传输边界增加独立入口。
