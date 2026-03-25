# NeoCode 思考流程改造统一方案（优化版）

## 1. 文档定位

本文整合并替代此前关于 NeoCode 思考流程改造的多份草案，形成一份统一的团队讨论与实施指导文档。目标是同时回答以下问题：

- 当前项目是否适合推进思考流程改造
- 这次改造应该落实到什么边界
- 当记忆模块和工具模块后续继续大改时，如何保证 Flow 模块能独立落地
- Flow 相关新增能力应该如何组织目录与职责
- 是否应引入设计模式，如果引入，应引入哪些、落在哪些位置

本文强调两点：

- 先稳定边界，再扩展流程
- 引入设计模式，但只引入“对当前项目真正有帮助的模式”，避免过度框架化

---

## 2. 结论摘要

当前项目适合推进思考流程改造，而且现有代码结构已经具备较好的基础：

- 服务端当前仍是“单次请求 -> 单次模型调用”，见 [chat_service.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/service/chat_service.go#L29)
- 工具闭环主要在 TUI 内完成，见 [update.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update.go#L63)
- 角色、记忆、工作记忆、安全、工具注册、配置系统均已存在，可被编排层统一收口

但如果目标是长期可演进，那么本次改造不能只做“功能追加”，而应同时完成三类建设：

1. 建立 Flow 编排层  
2. 冻结 Flow 与记忆/工具之间的稳定能力接口  
3. 用适合当前项目的设计模式组织流程，而不是继续用散落的 `if-else` 和 UI 闭环来驱动  

---

## 3. 当前问题与改造动机

### 3.1 当前主要问题
当前系统的核心问题不是“不能调用工具”，而是“流程能力没有统一归位”。

具体表现为：

- Service 只负责单轮问答
- TUI 负责工具闭环、审批暂停、结果回灌
- 记忆和工具能力被主链路直接调用，缺乏中间抽象
- 新增流程时，很容易牵一发动全身

### 3.2 如果不改，会发生什么
如果继续沿用当前模式，后续新增任何一种思考流程时，都需要重复处理这些问题：

- 何时调模型
- 何时调工具
- 工具结果如何回灌
- 审批如何暂停与恢复
- 记忆何时注入，何时写回
- 何时终止，何时回退

这会直接导致：

- 流程逻辑分散
- TUI 越来越重
- 函数命名和接口不断震荡
- 测试难以稳定

---

## 4. 推荐的设计模式

本项目可以考虑引入设计模式，但应遵循“少而准”的原则。推荐引入以下模式：

## 4.1 Strategy 模式：用于 Flow 本身
最适合当前项目的模式是 Strategy。

不同思考流程本质上就是不同执行策略：

- `single_shot`
- `react`
- `plan_execute`

它们共享依赖、共享上下文能力、共享工具执行器，但控制步骤不同。因此最自然的建模方式就是：

```go
type Flow interface {
	Run(ctx context.Context, req *ChatRequest) (<-chan ChatEvent, error)
}
```

每个 Flow 都是一个策略实现。

### 为什么适合
- 与当前“可切换思考流程”的目标天然匹配
- 新增流程时只新增实现，不改主链路
- 比把流程分支继续塞进 `chat_service.go` 更可维护

---

## 4.2 Factory + Registry 模式：用于 Flow 创建与切换
Flow 不应靠 `switch name { ... }` 散落在多个地方创建。

建议引入 Factory + Registry：

```go
type FlowFactory func(deps FlowDeps) Flow
```

配合注册表：

- 注册 `single_shot`
- 注册 `react`
- 注册 `plan_execute`
- 统一负责名称归一化、查找、默认值、未知值回退

### 为什么适合
- 与配置化切换、`/flow <name>` 命令天然匹配
- 能避免流程名称字符串散落在 TUI、Service、Config 校验中
- 新增流程时对主链路零侵入

---

## 4.3 Ports and Adapters：用于 Flow 与记忆/工具解耦
严格来说这不是一个传统 GoF 模式，而是一种非常适合当前项目的架构模式。

Flow 不应直接依赖：

- `MemoryService` 内部算法
- `WorkingMemoryService` 内部状态建模
- `GlobalRegistry`
- 工具安全审批内部实现

Flow 只应依赖几个稳定端口：

- `TurnContextAssembler`
- `TurnFinalizer`
- `FlowToolExecutor`
- `ToolCallParser`

然后由适配器桥接现有实现。

### 为什么适合
- 当前项目最大的风险就是“记忆和工具模块未来还会大改”
- 这个模式可以让 Flow 成为相对稳定的上层模块
- 底层重构时只改适配器，不改 Flow 策略本体

---

## 4.4 State Machine 模式：用于 `react` / `plan_execute` 的内部控制
对于 `single_shot`，不必引入状态机；但对于 `react` 和 `plan_execute`，建议显式引入轻量状态机思想。

建议至少把这些状态明确化：

- `PreparingContext`
- `CallingLLM`
- `ParsingToolCall`
- `WaitingApproval`
- `ExecutingTool`
- `Summarizing`
- `Completed`
- `Failed`

不一定需要引入外部状态机框架，但流程代码应按状态推进，而不是在一个函数里不断嵌套条件。

### 为什么适合
- ReAct 和 Plan/Execute 都天然是多阶段流程
- 状态化后更容易处理暂停/恢复、终止条件和错误回退
- 审批恢复特别适合用状态视角来建模

---

## 4.5 Rule Set / Policy Object：用于终止、审批、回退决策
你提到“规则树”，这个思路可以用，但不建议做成过重的通用规则树引擎。

更适合当前项目的做法是：

- 在几个高风险决策点引入轻量规则对象或策略组合
- 不做全局通用规则树 DSL

推荐用于以下场景：

### 终止策略
例如：
- 是否达到 `max_loops`
- 是否达到 `max_tool_calls`
- 是否当前已完成
- 是否应强制回退

### 审批策略
例如：
- 当前工具调用是否应 allow / ask / deny
- ask 后如何恢复
- reject 后是否继续生成解释性回复

### 回退策略
例如：
- planner schema 校验失败后回退到 `react`
- 工具协议解析失败时改为普通文本继续处理
- 工具异常时是否停止流程或继续下一轮总结

### 为什么是“规则对象”而不是“规则树引擎”
因为当前项目规模还没到需要引入完整规则树框架的程度。过早引入会造成：

- 规则表达复杂度高于业务复杂度
- 团队后续维护成本变大
- 调试困难

因此建议是：

- 可以引入“策略对象 / policy object”
- 不建议一开始引入重量级规则树或工作流引擎

---

## 4.6 不建议优先引入的模式
以下模式当前不建议优先上：

### Template Method
原因：
- 当前流程差异不仅是步骤顺序差异，还有阶段能力差异
- Strategy + 状态机更自然

### Command 模式
原因：
- 当前项目里的“动作”虽然可以抽象成 `CallLLM / ExecuteTool / Emit / Finalize`
- 但暂时没有必要将所有动作对象化，否则会把简单编排变复杂

### Visitor
原因：
- 当前没有复杂 AST 或多层对象遍历需求
- 不适用于当前主要问题

---

## 5. 推荐总体架构

## 5.1 目录结构

```text
internal/server/domain/
  chat.go
  tool.go
  memory.go
  working_memory.go
  flow.go

internal/server/service/
  chat_service.go
  memory_service.go
  working_memory_service.go
  todo_service.go
  role_service.go
  security_service.go

internal/server/orchestration/flow/
  engine.go
  registry.go
  ports.go
  policy.go
  single_shot.go
  react.go
  plan_execute.go
  context_assembler.go
  turn_finalizer.go
  tool_executor.go
  tool_protocol_json.go
```

## 5.2 各层职责
### `domain/`
放稳定契约：

- Flow 接口
- 事件结构
- Flow 状态
- 审批请求
- 抽象端口定义

### `service/`
保留稳定领域服务：

- 记忆服务
- 工作记忆服务
- 角色服务
- TODO 服务
- 安全服务

### `orchestration/flow/`
放所有与 Flow 一起变化的内容：

- 流程实现
- 工具协议解析
- 上下文组装
- 最终写回
- 工具执行适配
- 终止/审批/回退策略

---

## 6. 稳定能力接口

建议在 `domain/flow.go` 或 `orchestration/flow/ports.go` 中定义以下接口。

## 6.1 TurnContextAssembler
```go
type TurnContextAssembler interface {
	Assemble(ctx context.Context, req *ChatRequest, state *FlowState) ([]ContextBlock, error)
}
```

职责：

- 统一组装 role、working memory、todo、retrieved memory
- Flow 不直接依赖各类 context service

---

## 6.2 TurnFinalizer
```go
type TurnFinalizer interface {
	Finalize(ctx context.Context, req *ChatRequest, result *TurnResult) error
}
```

职责：

- 最终回复结束后刷新 working memory
- 最终回复结束后写 long-term memory
- 未来可扩展 todo、trace、metrics

---

## 6.3 FlowToolExecutor
```go
type FlowToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (*ToolExecution, error)
	ResolveApproval(ctx context.Context, token string, approved bool) (*ToolExecution, error)
}
```

职责：

- 工具查找
- 安全检查
- 审批 token 生成与恢复
- 工具结果标准化

---

## 6.4 ToolCallParser
```go
type ToolCallParser interface {
	ParseAssistantMessage(text string) (*ToolCall, bool, error)
}
```

职责：

- 从 assistant 文本中识别工具调用
- 当前实现为 `json_v1`
- 未来可替换为 native function calling

---

## 6.5 FlowPolicy
建议新增统一策略对象：

```go
type FlowPolicy interface {
	ShouldStop(state *FlowState) (bool, string)
	ShouldFallback(err error, state *FlowState) (bool, FlowName)
}
```

如果不做统一接口，也至少应在 `policy.go` 中集中放置终止与回退策略。

---

## 7. 三类 Flow 的职责

## 7.1 `single_shot`
- 最简单的策略实现
- 只做单轮上下文组装、单次模型调用、最终写回
- 不进入工具循环
- 作用是保持兼容现状，并作为所有新架构的基准流程

## 7.2 `react`
- 使用状态机思想驱动：
  - 组装上下文
  - 调模型
  - 解析工具调用
  - 执行工具
  - 审批暂停/恢复
  - 继续迭代
- 终止逻辑由 policy 控制，不写死在流程体中

## 7.3 `plan_execute`
- 先调用 planner
- 对计划做 schema 校验
- 步骤执行复用 `react` 的工具执行能力
- 校验失败或 planner 不可靠时，允许按 policy 回退到 `react`

---

## 8. 如何保证记忆模块和工具模块大改时不影响 Flow

## 8.1 核心原则
Flow 不依赖实现，只依赖能力。

### 具体意味着
Flow 不应直接依赖：

- [MemoryService.BuildContext](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/domain/memory.go#L44)
- [MemoryService.Save](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/domain/memory.go#L44)
- [WorkingMemoryService.BuildContext](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/domain/working_memory.go#L36)
- [WorkingMemoryService.Refresh](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/domain/working_memory.go#L36)
- [GlobalRegistry.Execute](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/infra/tools/tool.go#L67)
- [ApproveSecurityAsk](C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/infra/tools/security.go#L33)

Flow 只应依赖：

- `TurnContextAssembler`
- `TurnFinalizer`
- `FlowToolExecutor`
- `ToolCallParser`

---

## 8.2 记忆模块大改时的改动边界
如果未来记忆模块重构为：

- 多源检索
- 向量数据库
- 结构化工作记忆
- 新的写回策略

应只改这些位置：

- `memory_service.go`
- `working_memory_service.go`
- `context_assembler.go`
- `turn_finalizer.go`

不应改：

- `single_shot.go`
- `react.go`
- `plan_execute.go`

如果必须改 Flow 主循环，说明边界设计失败。

---

## 8.3 工具模块大改时的改动边界
如果未来工具模块重构为：

- 远端执行
- 原生 function calling
- 新审批模型
- 新工具元数据结构

应只改这些位置：

- `infra/tools/*`
- `tool_executor.go`
- `tool_protocol_json.go` 或新的 parser
- 安全相关适配层

不应改：

- Flow 主循环
- TUI 主事件处理
- Flow 状态机逻辑

---

## 9. TUI 的新职责

TUI 应从当前的“半个流程控制器”退回为“事件渲染层 + 用户交互层”。

## 9.1 TUI 保留职责
- 渲染文本流
- 渲染工具状态
- 渲染审批提示
- 发送 `/y` `/n`
- 发送 `/flow <name>`、`/provider`、`/switch`

## 9.2 TUI 移除职责
- assistant JSON 工具调用解析
- 本地执行工具
- 工具结果回灌为 system message
- 审批恢复状态机本体

相关现状参考：

- [update.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update.go#L82)
- [update.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update.go#L165)
- [runtime_services.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/services/runtime_services.go#L46)

---

## 10. 配置与切换

建议在 [app_config.go](C:/Users/10116/Desktop/study/gopro/neo-code/configs/app_config.go#L20) 的 `AI` 下新增结构化 `flow` 配置：

```yaml
ai:
  provider: "openll"
  api_key: "AI_API_KEY"
  model: "gpt-5.4"
  flow:
    name: "single_shot"
    tool_protocol: "json_v1"
    max_loops: 8
    max_tool_calls: 12
    planner_model: ""
    executor_model: ""
    plan_template_path: ""
    executor_template_path: ""
```

建议支持环境变量覆盖：

- `NEOCODE_FLOW`

TUI 新增：

- `/flow <name>`

状态栏新增：

- 当前 `model`
- 当前 `flow`

---

## 11. 推荐实施顺序

## 阶段 A：先建模式与边界，不改核心实现
实施内容：

- 新建 `domain/flow.go`
- 新建 `orchestration/flow/ports.go`
- 新建 `orchestration/flow/registry.go`
- 新建 `orchestration/flow/policy.go`

此阶段只完成：

- Strategy
- Factory/Registry
- Ports and Adapters
- 轻量 Policy

暂不改记忆算法和工具系统。

---

## 阶段 B：旧实现接入新适配层
实施内容：

- `context_assembler.go` 桥接当前记忆/角色/todo/working memory
- `turn_finalizer.go` 桥接当前写回逻辑
- `tool_executor.go` 桥接当前工具注册表和安全检查
- `tool_protocol_json.go` 承接当前 JSON 协议

此阶段目标是“旧实现，新边界”。

---

## 阶段 C：落地 `single_shot` 与 `react`
实施内容：

- `single_shot.go`
- `react.go`
- TUI 改为事件流消费
- `/y` `/n` 改为调用审批恢复接口

---

## 阶段 D：落地 `plan_execute`
实施内容：

- `plan_execute.go`
- planner schema 校验
- 回退策略
- 复用 `react` 执行器

---

## 阶段 E：推进记忆与工具模块重构
前提：

- Flow 已经只依赖稳定接口
- Flow 主循环已有合同测试

这样后续底层模块可独立演进。

---

## 12. 测试策略

## 12.1 Flow 合同测试
Flow 测试必须只依赖 fake 端口，不依赖真实实现。

应验证：

- 流程切换
- 状态推进
- 审批暂停与恢复
- 终止与回退
- `single_shot` / `react` / `plan_execute` 的输出行为

## 12.2 适配器测试
分别验证：

- `context_assembler.go`
- `turn_finalizer.go`
- `tool_executor.go`
- `tool_protocol_json.go`

这些测试负责保护 Flow 与底层模块之间的桥接逻辑。

## 12.3 TUI 回归测试
现有大量测试都围绕 TUI 本地工具闭环，应逐步改为：

- TUI 接收事件
- TUI 展示事件
- TUI 发起审批恢复

重点受影响测试参考：

- [update_test.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update_test.go#L949)
- [update_test.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update_test.go#L1009)
- [update_test.go](C:/Users/10116/Desktop/study/gopro/neo-code/internal/tui/core/update_test.go#L1381)

---

## 13. 风险与规避

### 风险一：模式引入过多，复杂度上升
规避：
- 只引入四类模式：Strategy、Factory/Registry、Ports and Adapters、轻量 State Machine/Policy
- 不引入重量级规则树引擎和工作流框架

### 风险二：Flow 继续碰底层实现
规避：
- 通过包结构和合同测试约束 Flow 只能依赖端口接口

### 风险三：`service` 再次变成杂糅层
规避：
- 所有 Flow 相关适配与策略统一进入 `orchestration/flow/`

### 风险四：审批恢复设计不完整
规避：
- 明确 token 化审批恢复接口
- 不再依赖 TUI 本地状态机拼装流程恢复

---

## 14. 建议团队优先确认的决策

这份方案建议团队优先讨论并确认以下几点：

- 是否接受以 Strategy 模式建模思考流程
- 是否接受以 Factory + Registry 管理 Flow 创建与切换
- 是否接受用 Ports and Adapters 保护 Flow 不受记忆与工具重构影响
- 是否接受用轻量 Policy/State Machine 管理 `react` 和 `plan_execute`
- 是否接受将 Flow 相关适配层统一放入 `internal/server/orchestration/flow/`
- 是否接受本轮只升级内部事件流，不改外部传输契约

---

## 15. 最终建议

建议团队采纳以下统一方向：

“NeoCode 将引入独立的 Flow 编排上下文，以 Strategy 模式定义不同思考流程，以 Factory + Registry 模式实现配置化切换，并通过 Ports and Adapters 将记忆模块与工具模块隔离在稳定边界之外。`react` 与 `plan_execute` 的内部控制采用轻量状态机与策略对象管理终止、审批和回退决策。所有与 Flow 一起演进的适配层统一放入 `internal/server/orchestration/flow/`，而不是继续堆入 `service` 根包。外部传输契约本轮保持兼容，待内部编排稳定后再评估进一步升级。”

