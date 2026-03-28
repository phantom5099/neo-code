# NeoCode

NeoCode 是一个基于 Go 和 Bubble Tea 的本地编码 Agent。它在终端中运行 ReAct 闭环，能够对话、调用工具、持久化会话，并以流式方式展示模型输出。

## 当前 Provider 策略

- 内建 provider 定义随代码版本发布。
- `config.yaml` 不再持久化完整 `providers` 列表。
- `config.yaml` 只保存当前选择状态和通用运行配置。
- 运行时的 `providers` 完全来自代码内建定义。
- API Key 只从环境变量读取，不写入 YAML。
- provider 实例自己定义 `base_url`、默认模型、可选模型列表和 `api_key_env`。
- `base_url` 不在 TUI 中展示给用户。
- driver 只负责协议构造与响应解析，不决定 `models`、`base_url` 或 `api_key_env`。

这意味着：

- 新用户启动后会自动拿到当前版本最新的内建 provider。
- 未来代码新增 provider 时，新用户不需要修改 YAML。
- 老配置文件中的 `providers` / `provider_overrides` 会在加载时被清理为新的最小状态格式。

## 配置文件

默认路径：
[`~/.neocode/config.yaml`](~/.neocode/config.yaml)

当前落盘结构示例：

```yaml
selected_provider: openai
current_model: gpt-5.4
workdir: .
shell: powershell
max_loops: 8
tool_timeout_sec: 20
```

其中：

- `selected_provider` 和 `current_model` 是用户当前选择。
- provider 的 `base_url`、`models`、`api_key_env` 和 `driver` 都由开发者在代码中预设。
- 完整 provider 列表不落盘，用户不需要在 YAML 中维护供应商元数据。

## Slash Commands

- `/provider`：打开 provider 选择器。
- `/model`：打开当前 provider 的模型选择器。

## 运行

```bash
go run ./cmd/neocode
```

## 开发

```bash
gofmt -w ./cmd ./internal
go test ./...
```
