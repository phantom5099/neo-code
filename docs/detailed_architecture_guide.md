# NeoCode 架构说明

## 设计目标

NeoCode 现在采用更常见的 Go 工程分层，而不是传统的 `domain / service / infra` 四层。核心目标是：

- provider 解决不同模型提供方和协议差异
- TUI 只负责 Bubble Tea 交互与状态编排
- tools 统一工具定义、schema、执行与结果处理
- config 统一管理配置和默认目录
- agent runtime 负责 agent 核心编排与模块协作

这套设计参考了主流 Go 项目的常见组织方式：

- 入口尽量薄，职责集中在 `cmd/...`
- 核心能力按模块分目录，不做“为了分层而分层”的 repository / service 过度拆分
- 协议适配、运行时编排、UI、配置、工具分别独立

## 顶层结构

### `cmd/neocode/`

唯一正式入口。只做：

- 解析启动参数
- 初始化 UTF-8 控制台
- 准备工作区
- 交互式确认配置
- 拉起 Bubble Tea 程序

它不承担业务逻辑。

### `configs/`

负责：

- `~/.neocode/config.yaml` 的读写
- provider 配置和当前模型选择
- 默认数据目录和 persona 路径
- 旧配置字段兼容

### `internal/provider/`

负责：

- provider 名称归一化
- 默认模型选择
- endpoint 解析
- API key 校验
- OpenAI 兼容协议
- Anthropic 原生协议
- Gemini 原生协议

这里的目标是对外只暴露统一的模型调用姿势。

### `internal/tool/`

负责：

- 工具定义
- 工具注册表
- 参数标准化
- 执行结果封装
- 工作区约束

当前内置工具按能力分组：

- `filesystem`：`read`、`write`、`edit`、`list`、`grep`
- `shell`：`bash`
- `web`：`webfetch`、`websearch`
- `runtime`：`todo`

其中：

- `internal/tool/security/` 负责安全策略
- `internal/tool/protocol/` 负责工具协议和 schema 提示
- `internal/tool/web/` 负责 Web 能力底层实现

### `internal/agentruntime/`

负责运行时装配，对上层暴露统一应用能力。

子模块职责：

- `chat/`：对话请求编排、上下文注入、provider 调用
- `memory/`：长期记忆和 session memory
- `session/`：working memory、workspace 快照与恢复摘要
- `todo/`：agent 内部 todo 服务
- `persona/`：persona prompt 加载

### `internal/tui/`

负责 Bubble Tea 界面。

子模块职责：

- `bootstrap/`：TUI 启动装配
- `core/`：Bubble Tea 状态流转与事件分发
- `components/`：纯渲染组件
- `state/`：纯状态结构
- `services/`：TUI 面向 runtime / provider / tool 的薄适配层

TUI 不再暴露 `/todo` 这类业务命令，也不再承接旧的代码执行辅助命令。

## 工具协议

当前推荐格式：

```json
{"type":"tool_call","name":"read","arguments":{"filePath":"README.md"}}
```

兼容旧格式：

```json
{"tool":"read","params":{"filePath":"README.md"}}
```

工具 schema 由 runtime 根据注册表动态注入，而不是再把 JSON 协议硬编码在 persona 里。

## 配置路径

默认配置目录：

```text
~/.neocode/
```

默认配置文件：

```text
~/.neocode/config.yaml
```

默认数据目录：

```text
~/.neocode/data/
```

如果调用方传入自定义配置路径，默认 persona / data 路径也会跟随该配置目录生成，避免路径写死到用户主目录。

## 当前结论

- `cmd/server` 已移除，因为当前项目没有真实独立 server 边界
- `api/proto` 已移除，因为当前没有独立传输契约层
- TUI 已明显瘦身，只保留 Bubble Tea UI 编排
- todo 已变成 agent 内部工具，而不是用户命令
- provider / tool / config / agent runtime / TUI 的边界已经基本清晰
