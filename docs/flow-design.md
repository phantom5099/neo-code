# NeoCode Flow 改造方案 v1.2（可执行版）

## 1. 文档定位

本文是面向落地的改造方案，不是概念讨论稿。目标是回答三件事：

- 本轮到底改什么，不改什么
- 如何把 TUI 里的流程控制迁移到服务端编排层
- 如何保证后续 memory/tools 重构时，Flow 主循环不用跟着大改

本文原则：

- 先收敛边界，再扩展能力
- 先完成可回滚的小步迁移，再谈新范式

---

## 2. 本轮目标、非目标与完成标准

### 2.1 本轮目标

- 把工具调用解析与执行闭环从 TUI 移到服务端编排层
- 把审批流程从 TUI 本地挂起，迁移为由 Flow 编排层持有的可恢复挂起记录；首版以 token 作为最小恢复句柄
- 把 TUI 退回为事件渲染与用户输入层

现状参考：

- 服务端仍是单次调用链路：[chat_service.go](../internal/server/service/chat_service.go)
- 工具解析/执行在 TUI：[update.go](../internal/tui/core/update.go)

### 2.2 本轮非目标

- 不引入完整 `plan_execute` 落地
- 不重写 memory 检索算法
- 不替换现有工具系统与安全策略实现

### 2.3 完成标准

- 聊天入口不再承担流程分支判断与工具闭环职责，统一委托给 Flow 编排层
- `update.go` 不再解析 assistant JSON 工具调用
- 新增 Flow 合同测试，覆盖暂停/恢复与终止条件

---

## 3. 当前痛点

- 流程控制散落：服务层做一次问答，TUI 做工具闭环和审批恢复
- 边界不稳：memory、tools 直接耦合在主链路，新增流程改动面过大
- 测试不稳：流程行为更多依赖 UI 侧状态，难做纯后端合同测试

---

## 4. 本轮只引入两类模式

### 4.1 Strategy（必须）

Flow 本质是策略切换，先只落地两种：

- `single_shot`
- `react`

`plan_execute` 只保留文档层扩展说明，不进入当前代码结构。

### 4.2 Ports and Adapters（必须）

Flow 只依赖稳定端口，不直接调用具体 memory/tools 实现。  
Factory/Registry、State、Policy 作为实现细节使用，不单独扩张为本轮“模式目标”。

---

## 5. 目标架构与目录

```text
internal/server/domain/
  flow.go

internal/server/orchestration/flow/
  engine.go
  registry.go
  ports.go
  single_shot.go
  react.go
  context_assembler.go
  turn_finalizer.go
  tool_executor.go
  tool_protocol_json.go
```

说明：

- `domain/flow.go`：稳定契约（接口、事件、状态、错误码）
- `orchestration/flow/`：流程实现与适配层
- `service/`：继续保留记忆、角色等领域服务，不承载流程编排细节

---

## 6. 契约定义

## 6.1 接口

### Flow接口

```go
type Flow interface {
	Run(ctx context.Context, req *ChatRequest) (<-chan ChatEvent, error)
}
```

### FlowEngine接口

```go
type FlowEngine interface {
    Run(ctx context.Context, req *ChatRequest) (<-chan ChatEvent, error)
    Resume(ctx context.Context, token string, approved bool) (<-chan ChatEvent, error)
}
```

### 事件顺序约束（示例）

- 每次 `Run` 必须以 `EventStarted` 开始
- 文本流可发多个 `EventDelta`
- 需要审批时发 `EventApprovalRequired` 并暂停
- 正常结束必须发 `EventCompleted`
- 异常结束必须发 `EventFailed`
- 同一轮只允许一个终态：`Completed` 或 `Failed`

### 最小可编码契约
- 事件与错误
```go
type EventType string
const (
    EventStarted   EventType = "started"
    EventDelta     EventType = "delta"
    EventToolCall  EventType = "tool_call"
    EventApproval  EventType = "approval_required"
    EventCompleted EventType = "completed"
    EventFailed    EventType = "failed"
)

type ErrorCode string
const (
    ErrNone         ErrorCode = ""
    ErrParser       ErrorCode = "parser_error"
    ErrToolFailed   ErrorCode = "tool_failed"
    ErrFinalize     ErrorCode = "finalize_failed"
    ErrApproval     ErrorCode = "approval_invalid"
    ErrCanceled     ErrorCode = "canceled"
    ErrLimitReached ErrorCode = "limit_reached"
)

type ChatEvent struct {
    Type       EventType
    Text       string
    Tool       *ToolCall
    Approval   *PendingApproval
    ErrorCode  ErrorCode
    ErrorMsg   string
    TurnID     string//
    RunID      string//
    Seq        int
}
```
- 状态与结果
```go
type FlowState struct {
    RunID        string
    TurnID       string
    Loops        int
    ToolCalls    int
    MaxLoops     int
    MaxToolCalls int
    Mode         string // single_shot | react
}

type TurnResult struct {
    Completed bool
    Failed    bool
    ErrorCode ErrorCode
    Text      string
}
```

- 终态唯一性规则


## 6.2 核心端口

```go
type TurnContextAssembler interface {
	Assemble(ctx context.Context, req *ChatRequest, state *FlowState) ([]ContextBlock, error)
}

type TurnFinalizer interface {
	Finalize(ctx context.Context, req *ChatRequest, result *TurnResult) error
}

type FlowToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (*ToolExecution, error)
	ResolveApproval(ctx context.Context, token string, approved bool) (*ToolExecution, error)
}

type ToolCallParser interface {
	ParseAssistantMessage(text string) (*ToolCall, bool, error)
}
```

## 6.3 审批恢复 token 约束（本轮最小实现）
- token 的唯一目的，是在审批挂起后重新定位一条待恢复的工具调用
- token 不承诺跨进程、跨设备、跨实例恢复能力
- token 必须在当前运行期内唯一，且能定位到一条待审批记录
- token 可以设置过期时间，但首版允许仅在当前进程生命周期内有效
- ResolveApproval 必须对重复提交安全：同一个 token 被重复确认时，不应重复执行工具
- token 无效、已消费、已过期时，应返回明确错误

本轮不要求：
- 引入正式 session_id
- 引入正式 request_id
- 定义远程 API 级别的审批恢复协议
- 保证应用重启后的 token 可恢复

---

## 7. Flow 范式范围

### 7.1 `single_shot`

- 一次上下文组装 + 一次模型调用 + 一次写回
- 禁止工具闭环
- 用于问答、解释、轻量总结

### 7.2 `react`

- 多轮：LLM -> 解析工具 -> 执行工具 -> 观察回灌 -> 继续
- 支持审批暂停与恢复
- 用于代码修改、排障、探索性任务

### 7.3 `plan_execute`

- 本轮不注册、不暴露、不配置 plan_execute。文档仅保留其作为后续扩展方向，不产生任何实现占位代码

---

## 8. 边界与改动规则

## 8.1 memory 重构时

优先只改：

- `memory_service.go`
- `working_memory_service.go`
- `context_assembler.go`
- `turn_finalizer.go`

默认不改：

- `single_shot.go`
- `react.go`

例外条件：

- 新能力要求改变流程控制语义（例如从“轮次驱动”改为“阶段图驱动”）

## 8.2 tools 重构时

优先只改：

- `infra/tools/*`
- `tool_executor.go`
- `tool_protocol_json.go` 或替代 parser

默认不改：

- Flow 主循环

例外条件：

- 工具返回协议从同步结果改为异步任务句柄

---

## 9. TUI 新职责

TUI 仅保留：

- 渲染事件流（delta、tool、approval、final）
- 发送用户输入与审批决策（如 `/y`、`/n`）
- 展示模型/provider/flow 当前配置

TUI 移除：

- assistant JSON 工具调用解析
- 本地执行工具
- 工具结果拼装并回灌 system message
- 审批恢复状态机主体

现状参考：

- [update.go](../internal/tui/core/update.go)
- [runtime_services.go](../internal/tui/services/runtime_services.go)

---

## 10. 迁移顺序（按切片，不按模式名）

### Slice 1：抽离 ToolCallParser

- 保持当前线上默认协议实现与行为一致（不绑定具体协议名）
- TUI 调 parser 的逻辑迁到 flow 层
- 验收：`update.go` 不再 `json.Unmarshal` assistant 最终文本

### Slice 2：抽离 FlowToolExecutor

- 接入现有 `GlobalRegistry` 与 security checker
- 先增加内存级审批挂起记录；是否持久化作为下一步增强项
- 验收：工具执行不再由 TUI 直接触发

### Slice 3：落地 react 主循环

- 事件化输出替代“本地消息拼装”
- 接入 `TurnContextAssembler` 与 `TurnFinalizer`
- 验收：同一轮具备 Started/Completed 或 Started/Failed 的终态闭环

### Slice 4：切 TUI 为纯渲染层

- TUI 只消费事件并提交审批决策
- 验收：删除 TUI 中工具闭环逻辑分支

---

## 11. 测试与验收

## 11.1 Flow 合同测试

必须覆盖：

- 流程切换：`single_shot` 与 `react`
- 终止条件：`max_loops`、`max_tool_calls`、上下文取消
- 审批暂停/恢复：同意、拒绝、重复提交、过期 token
- 错误回退：parser 失败、工具失败、finalize 失败

## 11.2 适配层测试

必须覆盖：

- `context_assembler.go`
- `turn_finalizer.go`
- `tool_executor.go`
- `tool_protocol_json.go`

## 11.3 TUI 回归测试

目标：

- 只验证事件渲染与用户交互，不再验证工具执行细节

---

## 12. 风险与回滚

风险 1：迁移过程出现双写逻辑（TUI 和 Flow 同时处理工具）  
应对：通过在yaml配置开关 `flow.runtime.enabled` 控制单一路径。

风险 2：审批 token 不可恢复导致会话中断  
应对：首版在编排层保存待审批记录，增加过期与幂等测试；是否写入 workspace 会话状态作为后续增强项。

风险 3：事件契约不稳定导致 UI 频繁返工  
应对：先冻结 `ChatEvent` 最小集合，再扩展字段。

---

## 13. 需要先拍板的决策

- 本轮仅交付 `single_shot` + `react`，`plan_execute` 不进入交付范围
- Flow 主循环事件契约先冻结，字段扩展走兼容策略
- 首版审批挂起记录仅保存在内存，还是直接落到 workspace 状态文件
- 是否接受通过配置开关分阶段灰度切流

---
