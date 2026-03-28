# NeoCode

## Name

NeoCode

## Description

NeoCode 是一个基于 Go 和 Bubble Tea 构建的本地编码 Agent。它在终端中运行 ReAct 闭环，可以对话、调用工具、持久化会话，并以流式方式展示推理结果。

## Capabilities

- 通过 provider 无关的 runtime 读取、理解并操作当前工作区代码
- 调用文件系统、Shell 和 Web 工具完成读取、写入、搜索、精准修改等任务
- 将会话历史持久化到本地，并在 TUI 侧边栏中恢复旧会话
- 在瀑布流终端界面中实时展示模型输出和工具执行过程
- 通过 Slash Command 切换当前 provider 和模型

## Project Structure

- `internal/config`：配置结构、YAML 加载、环境变量管理和并发安全访问
- `internal/provider`：provider 接口、catalog、driver 注册和厂商适配器
- `internal/provider/builtin`：内建 provider 定义和注册入口
- `internal/runtime`：事件流、Prompt 编排与 ReAct 主循环
- `internal/tui`：Bubble Tea 状态机、渲染层、Slash Command 和事件桥接

## Provider Rules

- 内建 provider 定义属于代码，不属于用户 YAML
- `config.yaml` 只保存 `selected_provider`、`current_model` 和 `provider_overrides`
- 运行时 provider 列表由“内建 provider + overrides”合并得到
- API Key 不得写入 YAML，只能通过环境变量提供
- TUI 不提供 `/set url` 或 `/set key`

## Setup Commands

- 启动应用：`go run ./cmd/neocode`
- 运行测试：`go test ./...`
- 格式化代码：`gofmt -w ./cmd ./internal`
