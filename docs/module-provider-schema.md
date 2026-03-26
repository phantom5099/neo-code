# 模块细化设计：Provider 层的多模型 Schema 抹平策略

本文档详细说明了如何屏蔽 OpenAI 和 Anthropic (Claude) 在 API Schema（特别是 Tool Calling 和 上下文角色）上的差异，确保 Agent Runtime 的逻辑纯粹性。

## 1. 核心差异分析 (The Problem)

在设计抹平策略前，必须认清两大阵营的本质区别：

| **维度**      | **NeoCode 标准期望**     | **OpenAI (GPT系列)**                        | **Anthropic (Claude 3系列)**                     |
| ------------- | ------------------------ | ------------------------------------------- | ------------------------------------------------ |
| **Tool 定义** | 标准的 JSON Schema       | `tools` 数组，类型为 `function`             | `tools` 数组，直接包含 `input_schema`            |
| **Tool 发起** | 返回 `[]ToolCall` 结构体 | 在 Message 中附带 `tool_calls` 数组         | 作为 `content` 数组中的 `tool_use` 块            |
| **参数格式**  | JSON 字符串 (`string`)   | JSON 字符串 (`string`)                      | JSON 对象 (`object`/`map`)                       |
| **结果回灌**  | **Role 为 tool 的消息**  | **Role 必须为 tool**，需提供 `tool_call_id` | **Role 必须为 user**，内容块类型为 `tool_result` |

> **最大痛点：结果回灌 (Context Feedback)**
>
> OpenAI 认为工具的执行结果是一种独立的角色 (`role: "tool"`)；而 Anthropic 认为工具执行结果是“用户对大模型发起调用的回应”，所以它的 Role 必须是 `user`。

## 2. 统一领域模型 (NeoCode Standard Domain Model)

为了抹平上述差异，我们必须在 `internal/provider/types.go` 中定义足够包容的**内部标准结构**。Runtime 永远只操作这些结构。

Go



```
// internal/provider/types.go

// 统一的消息结构
type Message struct {
    Role       string      // 仅限: "system", "user", "assistant", "tool"
    Content    string      // 文本内容
    ToolCalls  []ToolCall  // 当 Role="assistant" 且决定调用工具时有值
    ToolCallID string      // 当 Role="tool" 时，记录该结果对应哪个 ToolCall
    IsError    bool        // 当 Role="tool" 时，标记工具执行是否失败
}

// 统一的工具调用结构
type ToolCall struct {
    ID        string
    Name      string
    Arguments string // 统一序列化为 JSON 字符串，方便验证和传递
}

// 统一的工具定义结构
type ToolSpec struct {
    Name        string
    Description string
    Schema      any // JSON Schema 对应的 Go Struct
}
```

## 3. 入口转换策略 (NeoCode -> API Request)

当 Runtime 调用 `Provider.Chat(ctx, req)` 时，各 Provider 适配器负责将标准 `Message` 翻译为各自的 HTTP Payload。

### 3.1 OpenAI 的入口映射 (`internal/provider/openai/adapter.go`)

- **普通消息:** 直译 (`user` -> `user`, `assistant` -> `assistant`)。
- **工具回灌:** 当遍历到 `Role == "tool"` 的 NeoCode Message 时，直接映射为 OpenAI 的 `{"role": "tool", "tool_call_id": msg.ToolCallID, "content": msg.Content}`。

### 3.2 Anthropic 的入口映射 (`internal/provider/anthropic/adapter.go`)

- **普通消息:** 直译。

- **工具回灌 (极其重要):** 当遍历到 `Role == "tool"` 的 NeoCode Message 时，**不能**生成 `role: "tool"`。必须将其包装为 Anthropic 要求的格式：

  JSON

  

  ```
  {
    "role": "user",
    "content": [
      {
        "type": "tool_result",
        "tool_use_id": "<msg.ToolCallID>",
        "content": "<msg.Content>",
        "is_error": <msg.IsError>
      }
    ]
  }
  ```

- **连续 Tool Result 的合并:** Anthropic 不允许连续出现两个 `role: "user"` 的消息。如果 Runtime 传来了连续多个 `Role: "tool"`（例如并发执行了多个工具），Anthropic Adapter 必须在内部将它们**合并到一个** `role: "user"` 的 content 数组中。

## 4. 出口转换策略 (API Response -> NeoCode)

对于流式输出（SSE Stream），每个 Provider 必须在内部完成**碎片拼接**，并在流结束时统一抛出标准的 `[]ToolCall`。

### 4.1 OpenAI 的流解析提取

OpenAI 会通过多次 chunk 下发工具的 name 和 arguments 碎片。

- **策略:** 在 Adapter 内部维护一个 `map[int]*ToolCall` 缓存。遇到 `delta.tool_calls` 时，按 `index` 拼接 `arguments` 字符串。流结束后，将 map 转换为 `[]ToolCall` 返回给 Runtime。

### 4.2 Anthropic 的流解析提取

Anthropic 会下发 `content_block_start` (包含 tool_use 和 id/name) 和 `content_block_delta` (包含 input 的 JSON 碎片)。

- **策略:** 同样在内部维护状态机。注意 Anthropic 的 `input` 是对象，但在拼接时依然把它当作字符串拼起来。流结束后，不做结构体反序列化，直接将这段完整的 JSON 字符串原封不动地放入 NeoCode 的 `ToolCall.Arguments` 中（保持与 OpenAI 行为一致）。

## 5. 抽象接口最终形态

经过防腐层的隔离，对于 Runtime 而言，所有的 Provider 都长这样，极其干净：

Go



```
type ChatRequest struct {
    Model        string
    SystemPrompt string
    Messages     []Message  // 内部标准 Message
    Tools        []ToolSpec // 内部标准 Tool 定义
}

type ChatResponse struct {
    Message      Message // 返回的 Message (可能包含 Content 文本，也可能包含 ToolCalls 数组)
    FinishReason string  // "stop", "tool_calls", "length", etc.
}

type Provider interface {
    Name() string
    // Chat 负责发起请求。如果支持流式，内部应通过 eventBus (Channel) 发送过程事件，
    // 但最终必须返回一个完整的 ChatResponse 以终结本次调用。
    Chat(ctx context.Context, req ChatRequest, eventBus chan<- runtime.RuntimeEvent) (ChatResponse, error)
}
```