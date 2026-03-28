# NeoCode

NeoCode 是一个基于 Go 和 Bubble Tea 的本地编码 Agent。它在终端中运行 ReAct 闭环，能够对话、调用工具、持久化会话，并以流式方式展示模型输出。

## 当前 Provider 策略

- 内建 provider 定义跟随代码版本发布
- `config.yaml` 不再持久化完整 `providers` 列表
- `config.yaml` 只保存用户状态和显式覆写
- 运行时的 `providers` 由“内建定义 + 用户 overrides”合并得到
- API Key 只从环境变量读取，不写入 YAML

这意味着：

- 新用户启动后会自动拿到当前版本最新的内建 provider
- 未来代码新增 provider 时，新用户不需要修改 YAML
- 老用户如果没有覆写某个字段，也会跟随最新内建默认值

## 配置文件

默认路径：

[`~/.neocode/config.yaml`](~/.neocode/config.yaml)

当前落盘结构示例：

```yaml
selected_provider: openai
current_model: gpt-5.4
provider_overrides:
  - name: openai
    base_url: https://example.com/v1
    model: gpt-5.4
    api_key_env: AI_API_KEY
workdir: .
shell: powershell
max_loops: 8
tool_timeout_sec: 20
```

其中：

- `selected_provider` 和 `current_model` 是用户当前选择
- `provider_overrides` 只保存和内建默认值不同的部分
- 完整 provider 列表不落盘

## Slash Commands

- `/provider`：打开 provider 选择器
- `/model`：打开当前 provider 的模型选择器

## 运行

```bash
go run ./cmd/neocode
```

## 开发

```bash
gofmt -w ./cmd ./internal
go test ./...
```
