#### 1. 流式输出 (Streaming) 的并发与生命周期设计
* **事件定义:** 定义 `RuntimeEvent` 作为 Channel 传输的载荷。
  ```go
  type EventType string
  const (
      EventAgentChunk   EventType = "agent_chunk"
      EventAgentDone    EventType = "agent_done" // 一次完整的模型回复结束
      EventToolStart    EventType = "tool_start"
      EventToolResult   EventType = "tool_result"
      EventError        EventType = "error"
  )
  type RuntimeEvent struct {
      Type    EventType
      Payload any // 根据 Type 转型为 string, ToolCall 等
  }
  ```

- 生命周期: 1. Runtime 的 Run() 方法接收一个 EventBus chan RuntimeEvent。
  2. Runtime 开启 Goroutine 调用 Provider。
  3. Provider 内部读取 HTTP SSE (Server-Sent Events) 流，每读到一个 content chunk，就非阻塞地写入 Channel。
  4. 关键点: 当 Provider 遇到 [DONE] 或 io.EOF 时，它关闭自己这边的读取，并向 Channel 发送 EventAgentDone。此时 Runtime 知道这一轮对话结束，可以准备下一次输入或开始处理 Tool Calls。
  5. 若用户在 TUI 按下 Ctrl+C 或发起新请求，通过取消 context.Context 来强行中断 Provider 的网络流。

#### 2. Tool Call 的解析与拦截逻辑

- **Provider 的职责陷阱与解决:** 在流式 API（如 OpenAI）中，Tool Call 的 JSON 参数通常是被**切分成碎片的**（例如先发 `{"na`，再发 `me": "read_`）。
  - **设计:** Provider 在处理流时，遇到普通文本则立即发 `agent_chunk`；遇到 Tool Call 数据则**暂存在内存中进行拼接**，不发送给前端。直到流结束（Stream EOF），Provider 将拼接好的完整 JSON 反序列化，组装成标准的 `[]ToolCall` 结构体，最后随 `ChatResponse` 或特定的 `EventAgentDone` 一并返回给 Runtime。
- Runtime 的拦截执行: 1. Runtime 收到 Provider 返回的完整结构。
  2. 判断 len(ToolCalls) > 0。
  3. 如果大于 0，Runtime 暂停向 Provider 发起新请求，循环遍历 ToolCalls。
  4. 发送 EventToolStart 给 TUI。调用 ToolManager 执行本地代码。
  5. 获得 ToolResult，发送 EventToolResult 给 TUI。
  6. 继续 Loop: 将 ToolResult 封装成 Role 为 tool 的 Message 追加到 Context 中，自动再次触发 Provider 调用（无需用户介入）。

#### 3. 上下文长度管理 (Sliding Window)

- **结构设计:** 在 `SessionStore` 中维护 `[]Message`。
- **滑动规则:** 假设阈值为保留最近 N 轮（一轮 = 用户提问 + 助手回答 + 可能的工具执行）。
  - **System Prompt 永不丢弃:** 永远固定在 `Messages[0]`。
  - **Tool 强绑定:** 在裁剪历史时，绝不能出现“保留了 ToolResult，但丢弃了引发它的 ToolCall Assistant Message”的情况，这会导致大模型 API 报错（上下文逻辑断裂）。
  - **裁剪逻辑:** 当 `len(Messages)` 超过阈值时，从索引 1 开始向后扫描，找到一个完整的“对话边界”（即 User 的下一条新消息之前），将该边界之前的旧消息丢弃。