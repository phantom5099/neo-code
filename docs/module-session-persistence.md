# 模块细化设计：Session 会话历史与本地持久化

本文档定义了 NeoCode Agent 会话历史的存储介质、数据结构、读写时机以及 TUI 侧边栏的懒加载策略。

## 1. 存储介质选择 (Storage Strategy)

在 Go CLI 工具中，通常有两种主流持久化方案：

- **SQLite (如 modernc.org/sqlite 或 gorm)**: 适合复杂查询，结构化好。但在 MVP 阶段略显笨重，且如果用了基于 CGO 的驱动，会导致跨平台交叉编译变得复杂。
- **本地 JSON 文件**: 每个 Session 存为一个 `.json` 文件。极其轻量，肉眼可读（方便调试），无需引入复杂的 ORM 依赖。

**MVP 决策：采用本地 JSON 文件存储。** 所有的会话文件将默认存储在 `~/.neocode/sessions/` 目录下。

## 2. 数据结构设计 (Data Models)

为了兼顾存储完整性和 TUI 侧边栏的加载性能，我们需要区分“完整会话”和“会话摘要”。

Go



```
// internal/runtime/session.go

// Session 代表一个完整的会话，包含所有历史消息，用于写入文件和加载当前对话
type Session struct {
    ID        string             `json:"id"`
    Title     string             `json:"title"`
    CreatedAt time.Time          `json:"created_at"`
    UpdatedAt time.Time          `json:"updated_at"`
    Messages  []provider.Message `json:"messages"` // 内部标准的 Message 数组
}

// SessionSummary 仅用于 TUI 侧边栏列表展示，不包含庞大的 Messages 数组
type SessionSummary struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    UpdatedAt time.Time `json:"updated_at"`
}

// SessionStore 定义了持久化接口，方便未来无缝迁移到 SQLite
type SessionStore interface {
    Save(ctx context.Context, session *Session) error
    Load(ctx context.Context, id string) (*Session, error)
    ListSummaries(ctx context.Context) ([]SessionSummary, error)
    Delete(ctx context.Context, id string) error
}
```

## 3. 持久化时机 (When to Save?)

如果不加控制，每次敲击键盘都去写磁盘显然是不合理的。我们需要定义精确的触发点（Trigger Points）。

**设计方案：异步/节点触发保存** Runtime 在处理完一个完整的逻辑节点后，调用 `SessionStore.Save()` 覆写对应的 JSON 文件。

具体的触发节点如下：

1. **用户输入后：** 当用户的提问被追加到 `Messages` 数组后。
2. **工具执行完毕后：** 当 ToolResult 回灌到 `Messages` 数组后。
3. **模型回复结束时 (Stream EOF)：** 当 `EventAgentDone` 被触发，完整的 Assistant 消息组装完毕后。

## 4. TUI 侧边栏与懒加载 (Lazy Loading for UI)

Bubble Tea 的状态机需要高效运转，不能在启动时就把所有历史对话的完整内容读进内存。

### 4.1 启动与侧边栏渲染

1. TUI 启动时，调用 `SessionStore.ListSummaries()`。
2. Store 实现层读取 `~/.neocode/sessions/` 下的所有文件，**只反序列化外层的基础字段**（忽略或延迟解析 `messages` 数组），生成 `[]SessionSummary` 并按 `UpdatedAt` 倒序排列。
3. TUI 将这些 Summary 渲染在左侧边栏。

### 4.2 切换会话 (Session Switching)

1. 用户在侧边栏按下 `Up/Down` 选中某个旧会话，按下 `Enter`。
2. TUI 触发一个事件给 Runtime：`LoadSession(id)`。
3. Runtime 调用 `SessionStore.Load(id)` 读取完整的 JSON 文件到内存，替换当前的活跃 Session。
4. Runtime 将该 Session 的历史消息解析为文本，通过 `EventSessionLoaded` 发送给 TUI，TUI 清空当前中间的聊天视图并重新渲染。

## 5. 自动标题生成 (Auto-Titling)

一个体验良好的 Agent 不应该让历史记录全是 "New Chat"。我们需要一种轻量级的标题生成机制。

**MVP 策略：截取法** 在创建新 Session 并接收到用户的**第一条消息**时，直接截取该消息的前 15 个字符（或前几个中文字词）作为 `Title`。 *(注意：在 Phase 2 时，可以在后台起一个 Goroutine，用极低成本的模型如 gpt-4o-mini 根据第一轮对话生成精炼标题，但 MVP 阶段保持简单。)*