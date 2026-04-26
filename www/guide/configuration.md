---
title: 配置指南
description: 最小配置跑起来，然后按需调整模型、shell、超时和自定义 Provider。
---

# 配置指南

## 总原则

- `config.yaml` 只保存最小运行时状态
- provider 元数据来自代码内置定义或 custom provider 文件
- API Key 只从环境变量读取
- YAML 采用严格解析，未知字段直接报错

这意味着 NeoCode 当前不会：

- 自动清理旧版 `providers` / `provider_overrides`
- 自动兼容 `workdir`、`default_workdir` 等旧字段

## 最小配置

如果你只想让 NeoCode 跑起来，这个配置就够了：

```yaml
selected_provider: openai
current_model: gpt-5.4
shell: bash
```

Windows 用户把 `shell` 改成 `powershell`。其他字段都有默认值，不需要一开始就全部填写。

配置文件位置：`~/.neocode/config.yaml`

## 常见任务速查

### 切换模型

在 TUI 里直接切换，选择会自动保存：

```text
/provider          # 切换 Provider
/model             # 切换模型
```

如果模型列表为空，检查对应的环境变量是否已设置。

### 换 Shell

```yaml
shell: powershell    # Windows
shell: bash          # macOS / Linux
```

### 调工具超时

```yaml
tool_timeout_sec: 30    # 默认 20 秒
```

### 会话太长跑偏了

先试 `/compact`。如果经常跑偏，可以在配置里调大保留的最近消息数：

```yaml
context:
  compact:
    manual_keep_recent_messages: 20    # 默认 10
```

## 完整配置示例

```yaml
selected_provider: openai
current_model: gpt-5.4
shell: bash
tool_timeout_sec: 20
runtime:
  max_no_progress_streak: 3
  max_repeat_cycle_streak: 3
  assets:
    max_session_asset_bytes: 20971520
    max_session_assets_total_bytes: 20971520

tools:
  webfetch:
    max_response_bytes: 262144
    supported_content_types:
      - text/html
      - text/plain
      - application/json

context:
  compact:
    manual_strategy: keep_recent
    manual_keep_recent_messages: 10
    micro_compact_retained_tool_spans: 6
    read_time_max_message_spans: 24
    max_summary_chars: 1200
    micro_compact_disabled: false
  budget:
    prompt_budget: 0
    reserve_tokens: 13000
    fallback_prompt_budget: 100000
    max_reactive_compacts: 3
```

## 字段说明

### 基础字段

| 字段 | 说明 |
|------|------|
| `selected_provider` | 当前选中的 provider 名称 |
| `current_model` | 当前选中的模型 ID |
| `shell` | 默认 shell，Windows 默认 `powershell`，其他平台默认 `bash` |
| `tool_timeout_sec` | 工具执行超时（秒） |

### `context` 字段

| 字段 | 说明 |
|------|------|
| `context.compact.manual_strategy` | `/compact` 手动压缩策略，支持 `keep_recent` / `full_replace` |
| `context.compact.manual_keep_recent_messages` | `keep_recent` 策略下保留的最近消息数 |
| `context.compact.micro_compact_retained_tool_spans` | 默认保留原始内容的最近可压缩工具块数量，默认 `6` |
| `context.compact.read_time_max_message_spans` | context 读时保留的 message span 上限 |
| `context.compact.max_summary_chars` | compact summary 最大字符数 |
| `context.compact.micro_compact_disabled` | 是否关闭默认启用的 micro compact |
| `context.budget.prompt_budget` | 显式输入预算；`> 0` 时直接使用，`0` 表示自动推导 |
| `context.budget.reserve_tokens` | 自动推导输入预算时，从模型窗口中预留给输出、tool call、system prompt 的缓冲 |
| `context.budget.fallback_prompt_budget` | 模型窗口不可用或推导失败时使用的保底输入预算 |
| `context.budget.max_reactive_compacts` | 单次 Run 内允许的 reactive compact 最大次数 |

### `runtime` 字段

| 字段 | 说明 |
|------|------|
| `runtime.max_no_progress_streak` | 连续"无进展"轮次提醒阈值，默认 `5` |
| `runtime.max_repeat_cycle_streak` | 连续"重复调用同一工具参数"提醒阈值，默认 `3` |
| `runtime.max_turns` | 单次 Run 的最大推理轮数上限，默认 `40` |
| `runtime.assets.max_session_asset_bytes` | 单个 session asset 最大字节数，默认 20 MiB |
| `runtime.assets.max_session_assets_total_bytes` | 单次请求可携带的 session asset 总字节上限，默认 20 MiB |

### `verification` 字段

| 字段 | 说明 |
|------|------|
| `verification.enabled` | 是否启用验证引擎，默认 `true` |
| `verification.final_intercept` | 是否在任务收尾前拦截并触发验证，默认 `true` |
| `verification.max_no_progress` | 验证无进展时的最大重试次数，默认 `3` |
| `verification.max_retries` | 验证失败后的最大重试次数，默认 `2` |
| `verification.verifiers.<name>.enabled` | 是否启用该验证器 |
| `verification.verifiers.<name>.required` | 该验证器是否为硬性要求 |
| `verification.verifiers.<name>.timeout_sec` | 该验证器的执行超时 |
| `verification.verifiers.<name>.fail_closed` | 验证器异常时是否按失败处理 |
| `verification.execution_policy.allowed_commands` | 验证器可执行的命令白名单 |
| `verification.execution_policy.denied_commands` | 验证器禁止执行的命令黑名单 |

### `tools` 字段

| 字段 | 说明 |
|------|------|
| `tools.webfetch.max_response_bytes` | WebFetch 最大响应字节数 |
| `tools.webfetch.supported_content_types` | WebFetch 允许的内容类型 |
| `tools.mcp.servers` | MCP server 列表，见下方 MCP 配置 |

## 环境变量

API Key 只从环境变量读取，不写入配置文件。

| Provider | 环境变量 |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenLL | `AI_API_KEY` |
| Qiniu | `QINIU_API_KEY` |
| ModelScope | `MODELSCOPE_API_KEY` |

```bash
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="AI..."
export MODELSCOPE_API_KEY="ms-..."
```

## 自定义 Provider

如果你用的模型服务不在内置列表里，可以通过配置文件接入。

配置文件位置：`~/.neocode/providers/<name>/provider.yaml`

示例（OpenAI 兼容接口）：

```yaml
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
model_source: discover
base_url: https://llm.example.com/v1
chat_api_mode: chat_completions
chat_endpoint_path: /chat/completions
discovery_endpoint_path: /models
```

也可以在 TUI 里用 `/provider add` 交互式添加。

### 手动指定模型列表

如果 provider 不支持模型发现（`/models` 接口），使用 `model_source: manual`：

```yaml
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
model_source: manual
base_url: https://llm.example.com/v1
chat_endpoint_path: /chat/completions
models:
  - id: gpt-4o-mini
    name: GPT-4o Mini
    context_window: 128000
```

### 自定义 Provider 字段说明

| 字段 | 说明 |
|------|------|
| `name` | provider 标识，用于 `selected_provider` |
| `driver` | 驱动类型，目前支持 `openaicompat` |
| `api_key_env` | API Key 的环境变量名 |
| `model_source` | `discover`（自动发现）或 `manual`（手动列表） |
| `base_url` | 服务 base URL |
| `chat_api_mode` | `chat_completions` 或 `responses` |
| `chat_endpoint_path` | 聊天接口路径 |
| `discovery_endpoint_path` | 模型发现接口路径（`discover` 模式） |

## MCP 工具

如果你有 MCP server，可以在 `config.yaml` 中通过 `tools.mcp.servers` 注册。当前支持 `stdio` server，工具注册后会以 `mcp.<server-id>.<tool>` 命名。

```yaml
tools:
  mcp:
    servers:
      - id: docs
        enabled: true
        source: stdio
        version: v1
        stdio:
          command: node
          args:
            - ./mcp-server.js
          workdir: ./mcp
          start_timeout_sec: 8
          call_timeout_sec: 20
          restart_backoff_sec: 1
        env:
          - name: MCP_TOKEN
            value_env: MCP_TOKEN
```

完整字段、暴露策略、验证方法和排障步骤见 [MCP 工具接入](./mcp)。

## 不允许写进 config.yaml 的字段

以下字段如果出现在主配置文件中，加载会直接报错：

`providers`、`provider_overrides`、`workdir`、`default_workdir`、`base_url`、`api_key_env`、`models`

## 常见错误

### 旧字段被拒绝

如果在 `config.yaml` 中包含 `workdir`、`providers` 等字段，当前版本会报未知字段错误。处理方式是手动删除这些字段。

### 旧 `context.auto_compact` 字段

如果配置中只存在 `context.auto_compact`，启动时 preflight 会自动迁移为 `context.budget`，并写入 `config.yaml.bak` 备份。如果 `context.auto_compact` 与 `context.budget` 同时存在，启动会直接报错，需要手动合并后再启动。

### API Key 未设置

```text
config: environment variable OPENAI_API_KEY is empty
```

在当前 shell 中设置对应环境变量后再启动 NeoCode。

## CLI 运行参数覆盖

工作目录不写入 `config.yaml`，只通过启动参数覆盖：

```bash
neocode --workdir /path/to/workspace
```

## 下一步

- 想了解日常操作：[日常使用](./daily-use)
- 想了解 Agent 能做什么：[工具与权限](./tools-permissions)
- 遇到问题：[排障与常见问题](./troubleshooting)
- 切换模型：[升级与版本检查](./update)
