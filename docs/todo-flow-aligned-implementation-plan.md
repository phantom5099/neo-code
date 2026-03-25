# NeoCode Todo 模块实施方案（完全对齐 Flow 设计）

## 1. 文档目标

本文将当前 Todo 模块的增强方案重写为**完全符合** [flow-design.md](/C:/Users/10116/Desktop/study/gopro/neo-code/docs/flow-design.md) 的版本，目标是：

- 让 Todo 成为 Flow 可复用的稳定能力，而不是新的编排中心
- 保持 `single_shot / react / plan_execute` 为唯一的流程策略入口
- 让 TUI 从“半个流程控制器”退回为“事件渲染层 + 用户交互层”
- 在不破坏现有 `json_v1` 工具协议的前提下，逐步增强 Todo 的表达力与可维护性

本文覆盖：

- 模块边界
- 接口规范
- 依赖关系
- 调用流程
- 设计模式选择
- 分阶段实施计划
- 测试与风险控制

---

## 2. 设计原则

本方案严格遵守 [flow-design.md](/C:/Users/10116/Desktop/study/gopro/neo-code/docs/flow-design.md) 的以下原则：

1. **Flow-first**
   Flow 是一等公民；Todo 只是 Flow 通过稳定端口接入的一项能力。

2. **Ports and Adapters**
   Flow 主循环只依赖稳定端口，不直接依赖 `TodoService`、`MemoryService`、`GlobalRegistry` 等具体实现。

3. **orchestration/flow 收口**
   所有与 Flow 一起演进的内容统一进入 `internal/server/orchestration/flow/`，不继续堆入 `service` 根包。

4. **TUI 瘦身**
   TUI 不负责 assistant JSON 解析、工具执行、工具结果回灌或审批恢复状态机本体。

5. **兼容优先**
   第一阶段保留现有 `todo` 工具名和 `json_v1` 协议，优先做“旧实现，新边界”。

6. **少而准的模式**
   只使用文档已认可的模式：Strategy、Factory/Registry、Ports and Adapters、轻量 State Machine、Policy Object。
   不优先引入 Command、重型规则树或工作流引擎。

---

## 3. 总体架构

### 3.1 角色定位

- `domain/`：定义稳定契约与领域模型
- `service/`：保留稳定领域服务，如 Todo、Memory、WorkingMemory、Role、Security
- `orchestration/flow/`：承载 Flow 策略、上下文组装、最终落盘、工具执行适配、协议解析、终止/回退策略
- `tui/`：渲染事件、接收用户输入、发起审批恢复或用户命令

### 3.2 Todo 的定位

Todo 在新架构中不是独立流程引擎，而是三种能力之一：

- 被 `TurnContextAssembler` 读取，拼装为当前轮上下文
- 被 `TurnFinalizer` 更新，用于在回合结束后推进状态或写入补充信息
- 被 `FlowToolExecutor` 间接调用，通过 `todo` 工具显式增删改查

---

## 4. 目录建议

在遵守 [flow-design.md](/C:/Users/10116/Desktop/study/gopro/neo-code/docs/flow-design.md) 推荐目录的前提下，Todo 相关改造建议如下：

```text
internal/server/domain/
  todo.go
  flow.go

internal/server/service/
  todo_service.go

internal/server/infra/repository/
  todo_repository.go
  file_todo_repository.go

internal/server/infra/tools/
  todo.go

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

注意：

- Todo 领域模型和领域服务继续留在 `domain/` 与 `service/`
- Todo 的 Flow 接入点只能放在 `orchestration/flow/`
- 不新增 `service/agent_orchestrator.go`、`service/plan_context_provider.go` 之类文件

---

## 5. 模块边界与依赖关系

### 5.1 允许的依赖方向

```text
TUI -> services facade -> orchestration/flow -> domain ports
                                 |            |
                                 v            v
                              service      domain
                                 |
                                 v
                           infra/repository
                           infra/tools
```

### 5.2 不允许的依赖

Flow 层不得直接依赖：

- `TodoService` 的具体实现细节
- `MemoryService.BuildContext(...)`
- `WorkingMemoryService.Refresh(...)`
- `tools.GlobalRegistry.Execute(...)`
- `tools.ApproveSecurityAsk(...)`
- TUI 状态结构

### 5.3 Todo 与 Flow 的正确边界

正确方式：

- `context_assembler.go` 依赖 `TodoService`，把 Todo 摘要装配为 `ContextBlock`
- `turn_finalizer.go` 依赖 `TodoService`，根据当前轮结果决定是否推进 Todo
- `tool_executor.go` 依赖工具注册表，间接执行 `todo` 工具

错误方式：

- `react.go` 直接调用 `TodoService.AddTodo(...)`
- `plan_execute.go` 直接依赖 `todo_repository`
- TUI 继续解析模型输出 JSON 并直接执行 `todo` 工具

---

## 6. 接口规范

## 6.1 Flow 稳定端口

这些接口应定义在 `domain/flow.go` 或 `orchestration/flow/ports.go`，并成为 Flow 的唯一稳定依赖。

### TurnContextAssembler

```go
type TurnContextAssembler interface {
	Assemble(ctx context.Context, req *ChatRequest, state *FlowState) ([]ContextBlock, error)
}
```

职责：

- 统一组装 role
- 统一组装 working memory
- 统一组装 todo context
- 统一组装 long-term memory context

Todo 相关要求：

- Todo 只能通过这里进入 Flow 上下文
- 不允许在 `single_shot.go`、`react.go`、`plan_execute.go` 中手写 Todo prompt 拼装逻辑

### TurnFinalizer

```go
type TurnFinalizer interface {
	Finalize(ctx context.Context, req *ChatRequest, result *TurnResult) error
}
```

职责：

- 刷新 working memory
- 保存 long-term memory
- 更新 Todo 派生状态
- 记录 trace / metrics 的扩展点

Todo 相关要求：

- Todo 的自动推进只能经由这里扩展
- 不能把 Todo 状态推进逻辑散落在 TUI 或 Flow 策略实现中

### FlowToolExecutor

```go
type FlowToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (*ToolExecution, error)
	ResolveApproval(ctx context.Context, token string, approved bool) (*ToolExecution, error)
}
```

职责：

- 查找工具
- 安全检查
- 审批 token 生成与恢复
- 返回标准化工具执行结果

Todo 相关要求：

- Flow 若要显式修改 Todo，必须通过 `todo` 工具进入
- 不允许绕过工具执行器直接修改仓储

### ToolCallParser

```go
type ToolCallParser interface {
	ParseAssistantMessage(text string) (*ToolCall, bool, error)
}
```

职责：

- 识别 assistant 输出中的工具调用
- 当前承接 `json_v1`
- 未来可替换为 native function calling

Todo 相关要求：

- `todo` 工具协议解析只能在 parser 中演进
- TUI 不再承担 assistant JSON 解析

### FlowPolicy

```go
type FlowPolicy interface {
	ShouldStop(state *FlowState) (bool, string)
	ShouldFallback(err error, state *FlowState) (bool, FlowName)
}
```

职责：

- 终止条件
- planner 失败回退
- 工具异常后的继续/中断决策

Todo 相关要求：

- Todo 不能承担 Flow 是否继续执行的判定职责
- Todo 只能提供上下文和状态，不替代 FlowPolicy

---

## 6.2 Todo 领域接口

Todo 仍然属于领域服务，不属于 Flow 端口。

建议在不破坏现有契约的前提下，增强 [todo.go](/C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/domain/todo.go)。

### 领域模型

保持现有核心字段：

```go
type Todo struct {
	ID       string       `json:"id"`
	Content  string       `json:"content"`
	Status   TodoStatus   `json:"status"`
	Priority TodoPriority `json:"priority"`
}
```

建议逐步新增可选字段：

```go
type Todo struct {
	ID           string       `json:"id"`
	Content      string       `json:"content"`
	Status       TodoStatus   `json:"status"`
	Priority     TodoPriority `json:"priority"`
	Detail       string       `json:"detail,omitempty"`
	Dependencies []string     `json:"dependencies,omitempty"`
	Source       string       `json:"source,omitempty"`
	CreatedAt    time.Time    `json:"created_at,omitempty"`
	UpdatedAt    time.Time    `json:"updated_at,omitempty"`
}
```

说明：

- `content/status/priority` 继续保持外部兼容
- 新字段只作为渐进增强，不影响当前工具协议的最小集

### TodoService

保守演进版本：

```go
type TodoService interface {
	AddTodo(ctx context.Context, content string, priority TodoPriority) (*Todo, error)
	UpdateTodoStatus(ctx context.Context, id string, status TodoStatus) error
	ListTodos(ctx context.Context) ([]Todo, error)
	ClearTodos(ctx context.Context) error
	RemoveTodo(ctx context.Context, id string) error
}
```

增强建议：

```go
type TodoService interface {
	AddTodo(ctx context.Context, content string, priority TodoPriority) (*Todo, error)
	UpdateTodoStatus(ctx context.Context, id string, status TodoStatus) error
	UpdateTodo(ctx context.Context, todo Todo) (*Todo, error)
	ListTodos(ctx context.Context) ([]Todo, error)
	ListActiveTodos(ctx context.Context) ([]Todo, error)
	ClearTodos(ctx context.Context) error
	RemoveTodo(ctx context.Context, id string) error
}
```

注意：

- 即使增强 `TodoService`，Flow 也不直接依赖它
- Flow 只通过 `TurnContextAssembler` / `TurnFinalizer` / `FlowToolExecutor` 间接使用 Todo 能力

### TodoRepository

```go
type TodoRepository interface {
	Add(ctx context.Context, todo Todo) (*Todo, error)
	UpdateStatus(ctx context.Context, id string, status TodoStatus) error
	Update(ctx context.Context, todo Todo) (*Todo, error)
	List(ctx context.Context) ([]Todo, error)
	Clear(ctx context.Context) error
	Remove(ctx context.Context, id string) error
}
```

第一阶段可以保留内存仓储，第二阶段增加文件仓储实现。

---

## 6.3 todo 工具协议

为了符合“外部契约保持兼容”的要求，保留当前工具名与主协议：

```json
{
  "tool": "todo",
  "params": {
    "action": "add|update|list|remove|clear",
    "content": "write tests",
    "priority": "medium",
    "id": "todo-1",
    "status": "completed"
  }
}
```

实施建议：

- 阶段 A/B 不修改外部协议
- 阶段 C 以后再考虑增加可选动作，如 `replace`、`advance`
- 即使新增动作，也必须保持老协议可用

---

## 7. 调用流程

## 7.1 `single_shot` 流程

```text
TUI -> flow engine(single_shot)
    -> TurnContextAssembler
    -> ChatProvider.Chat
    -> TurnFinalizer
    -> TUI 渲染最终回答
```

Todo 参与方式：

- 只通过 `TurnContextAssembler` 提供 Todo 摘要
- 不进入工具循环
- 可由 `TurnFinalizer` 做非常保守的收尾更新

适用场景：

- 保持现状兼容
- 作为新架构基线

## 7.2 `react` 流程

```text
TUI -> flow engine(react)
    -> Assemble context
    -> Call LLM
    -> Parse tool call
    -> FlowToolExecutor.Execute
    -> 生成 ToolExecution 事件
    -> 继续下一轮
    -> TurnFinalizer
```

Todo 参与方式：

- Todo 摘要由 `TurnContextAssembler` 注入
- 模型若需显式修改 Todo，必须调用 `todo` 工具
- 工具执行完成后，由 `TurnFinalizer` 做状态补充或归档

审批要求：

- `FlowToolExecutor` 负责 token 化审批
- TUI 只展示审批并发起 `/y` `/n`
- 审批恢复走 `ResolveApproval(...)`

## 7.3 `plan_execute` 流程

```text
TUI -> flow engine(plan_execute)
    -> planner 生成计划
    -> schema 校验
    -> 合法则进入执行阶段
    -> 执行阶段复用 react 的工具执行能力
    -> TurnFinalizer
```

Todo 参与方式：

- planner 产生的计划可以映射为 Todo 列表
- 计划写入不应直接发生在 `plan_execute.go`
- 应通过 `todo` 工具或 `TurnFinalizer` 的明确策略完成

回退要求：

- planner schema 校验失败时，由 `FlowPolicy` 决定是否回退到 `react`

## 7.4 TUI 手工 `/todo` 流程

```text
用户输入 /todo -> TUI 命令处理 -> ChatClient facade -> TodoService -> Repository
```

注意：

- `/todo` 是用户交互入口，不是 Flow 主循环
- TUI 可以保留 `/todo` 命令，但不能继续承担工具协议解析与本地工具闭环
- `/todo` 与 agent 自动 `todo` 工具操作最终应落到同一 `TodoService`

---

## 8. Todo 上下文策略

Todo 不能再像当前 [chat_service.go](/C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/service/chat_service.go#L117) 那样在主链路直接手写拼接。

建议由 `context_assembler.go` 统一生成：

```text
[TODO]
In Progress:
- todo-3: implement parser

Pending:
- todo-4: add tests
- todo-5: update docs

Blocked:
- todo-6: waiting user decision
```

规则：

- 优先显示 `in_progress`
- `pending` 最多显示前 3 到 5 项
- `completed` 默认不注入
- Todo 过多时只摘要，不全量塞入 prompt

这样做的好处：

- 减少上下文噪音
- 保持 Flow 策略代码无 Todo 拼接细节
- 后续 Todo 模型增强时只需改 `context_assembler.go`

---

## 9. Todo 状态机

文档推荐对 `react / plan_execute` 使用轻量状态机；这里 Todo 自身也应有轻量状态迁移规则，但它是**领域状态机**，不是 Flow 状态机。

建议状态：

- `pending`
- `in_progress`
- `completed`
- 第二阶段可增加 `blocked`

建议迁移：

```text
pending -> in_progress
in_progress -> completed
in_progress -> pending
blocked -> in_progress
pending -> cancelled   (可选，后续阶段)
```

约束：

- 状态迁移校验放在 `TodoService` 内部
- TUI 不再自己定义“切一下状态就行”
- `todo` 工具更新状态时也走同一校验逻辑

---

## 10. 设计模式选择

## 10.1 应采用的模式

### Strategy

用于三种 Flow：

- `single_shot`
- `react`
- `plan_execute`

Todo 不是 Strategy 主体，只是被策略消费的一种能力。

### Factory + Registry

用于创建和切换 Flow：

- 注册 `single_shot`
- 注册 `react`
- 注册 `plan_execute`

Todo 不参与 Factory，只作为底层能力提供给适配器。

### Ports and Adapters

这是 Todo 改造中最关键的约束：

- Flow 不直接依赖 Todo 实现
- Todo 能力通过 `TurnContextAssembler`、`TurnFinalizer`、`FlowToolExecutor` 间接进入 Flow

### Lightweight State Machine

用于：

- `react` / `plan_execute` 的流程状态推进
- TodoService 内部的状态迁移校验

### Policy Object

用于：

- Flow 终止条件
- planner 失败回退
- 工具审批恢复
- 工具异常后继续/中断决策

## 10.2 不建议优先引入的模式

### Command

不引入 `AddTodoCommand`、`UpdateTodoCommand` 这类完整 Command 模式体系。

原因：

- 与 [flow-design.md](/C:/Users/10116/Desktop/study/gopro/neo-code/docs/flow-design.md) 的“少而准”原则不一致
- 当前阶段会把简单编排进一步对象化，复杂度高于收益

### 重型规则树 / DSL

不引入通用规则引擎，不做全局 DSL。

原因：

- 当前规模不需要
- 调试和维护成本过高

---

## 11. 分阶段实施

## 阶段 A：先建边界，不改核心实现

目标：

- 建立 Flow 的稳定端口
- 明确目录与依赖边界

工作项：

- 新建 `domain/flow.go`
- 新建 `orchestration/flow/ports.go`
- 新建 `orchestration/flow/registry.go`
- 新建 `orchestration/flow/policy.go`

Todo 相关要求：

- 不重写 TodoService
- 不改工具协议
- 不改 Memory 算法
- 只为后续 Todo 接入预留合法位置

## 阶段 B：旧实现接入新适配层

目标：

- 做到“旧实现，新边界”

工作项：

- `context_assembler.go` 桥接 role / working memory / todo / memory
- `turn_finalizer.go` 桥接 working memory 刷新、memory save、todo 补充更新
- `tool_executor.go` 桥接当前工具注册表与安全检查
- `tool_protocol_json.go` 承接当前 `json_v1`

Todo 相关要求：

- Todo 从 [chat_service.go](/C:/Users/10116/Desktop/study/gopro/neo-code/internal/server/service/chat_service.go) 的直接拼接中移出
- TUI 不再处理 assistant JSON tool call

## 阶段 C：落地 `single_shot` 与 `react`

目标：

- 让 Flow engine 接管当前闭环

工作项：

- 实现 `single_shot.go`
- 实现 `react.go`
- TUI 改为消费 Flow 事件
- `/y` `/n` 改为调用审批恢复接口

Todo 相关要求：

- `/todo` 保留为用户命令
- agent 自动修改 Todo 通过 `todo` 工具执行
- Todo 状态推进从 TUI 迁出，统一由 TodoService 校验

## 阶段 D：落地 `plan_execute`

目标：

- 在不破坏边界的前提下引入规划能力

工作项：

- 实现 `plan_execute.go`
- planner schema 校验
- fallback 策略
- 执行阶段复用 `react`

Todo 相关要求：

- planner 输出可映射为 Todo
- 映射逻辑放在 `plan_execute` 配套适配器中，不直接侵入 Todo 仓储

## 阶段 E：增强 Todo 领域能力

前提：

- Flow 已稳定运行在新架构中
- Flow 合同测试已建立

工作项：

- 引入文件型 Todo 仓储
- 增加 `detail/dependencies/source/updated_at`
- 增强筛选、排序、归档
- 视情况增加 `blocked`

注意：

- 这一阶段增强 Todo，但不修改 Flow 主循环
- 若需要改 `react.go` 或 `plan_execute.go` 才能支持 Todo 新能力，说明边界设计失败

---

## 12. 测试策略

## 12.1 Flow 合同测试

用 fake 端口测试：

- `TurnContextAssembler`
- `TurnFinalizer`
- `FlowToolExecutor`
- `ToolCallParser`

验证：

- 流程切换
- 工具执行
- 审批暂停与恢复
- 终止与回退

## 12.2 Todo 领域测试

验证：

- 状态迁移是否合法
- 排序与筛选逻辑
- 文件仓储与内存仓储的一致性
- `/todo add` 多词内容场景

## 12.3 适配器测试

重点覆盖：

- `context_assembler.go`
- `turn_finalizer.go`
- `tool_executor.go`
- `tool_protocol_json.go`

验证 Todo 是否正确通过适配层进入 Flow，而非被 Flow 直接依赖。

## 12.4 TUI 回归测试

TUI 测试目标改为：

- 接收 Flow 事件
- 展示工具状态
- 展示审批
- 发起 `/y` `/n`
- 处理 `/todo`

而不是继续测试“本地 JSON 解析 + 本地工具执行 + system message 回灌”。

---

## 13. 风险与优化建议

### 风险 1：Todo 继续演化成新的编排中心

规避：

- 不新增 `PlanService` 作为 Flow 的直接依赖
- 不让 `react.go` 或 `plan_execute.go` 直接 import Todo 实现细节

### 风险 2：TUI 迁移不彻底

规避：

- 第一优先迁走 assistant JSON 解析
- 第二优先迁走工具执行
- 第三优先迁走审批恢复状态机

### 风险 3：Todo 增强过早，拖慢 Flow 重构

规避：

- 阶段 A/B 只做边界
- Todo 富模型增强放到阶段 E

### 风险 4：外部协议破坏兼容

规避：

- 保留 `todo` 工具名
- 保留 `json_v1`
- 保留 `add/update/list/remove/clear`

---

## 14. 最终建议

建议团队采用以下方向：

- 先按 [flow-design.md](/C:/Users/10116/Desktop/study/gopro/neo-code/docs/flow-design.md) 建立 `Flow engine + Ports + Registry + Policy`
- Todo 不升级为新的流程中心，而是保留为领域能力
- Todo 与 Flow 的连接统一收敛到 `internal/server/orchestration/flow/`
- TUI 只做渲染与交互，不做编排
- 在 Flow 稳定后，再逐步增强 Todo 的模型、持久化和规划表达力

一句话概括：

**先让 Todo 成为“被 Flow 正确消费的能力”，再让它变强；不要让 Todo 在 Flow 稳定之前抢走编排中心的位置。**
