# 模块细化设计：Tools 与 TUI

本文档细化了 NeoCode MVP 架构中工具执行（特别是长耗时与流式输出）、安全确认机制（Human-in-the-loop）以及 TUI 界面与底层 Runtime 事件的优雅桥接方案。

## 1. Tools 模块：流式输出与安全拦截

### 1.1 支持流式输出的工具接口 (Streaming Output)

为了支持像 `npm install` 这种长时间运行且需要实时反馈的命令，工具的执行结果不能仅仅是同步返回一个最终的 `ToolResult`。我们需要允许工具在运行期间持续发出输出块（Chunks）。

**设计方案：回调函数注入 (Callback Injection)** 不在 `Execute` 签名里硬塞 Channel，而是通过 Context 或参数注入一个回调函数/接口，保持签名的干净。

Go



```
// 定义回调签名
type ChunkEmitter func(chunk []byte)

type ToolCallInput struct {
    ID        string
    Name      string
    Arguments []byte
    SessionID string
    Workdir   string
    // 注入发射器，工具在执行时可以调用它输出流式数据
    EmitChunk ChunkEmitter 
}

// Bash Tool 的具体实现示例思路
func (b *BashTool) Execute(ctx context.Context, input ToolCallInput) (ToolResult, error) {
    cmd := exec.CommandContext(ctx, "bash", "-c", parseArgs(input.Arguments))
    cmd.Dir = input.Workdir

    // 拦截 Stdout 和 Stderr
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    
    cmd.Start()

    // 开启 Goroutine 实时读取输出并调用 EmitChunk
    go func() {
        scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
        for scanner.Scan() {
            if input.EmitChunk != nil {
                input.EmitChunk(scanner.Bytes())
            }
        }
    }()

    cmd.Wait()
    // 返回最终结果（可以包含 exit code 等 metadata）
    return ToolResult{/*...*/}, nil
}
```

**Runtime 的处理：** Runtime 在调用 `ToolManager.Execute` 时，为其构造一个 `EmitChunk` 闭包，这个闭包的作用就是将捕获到的 `[]byte` 包装成 `EventToolChunk` 发送到事件总线。

### 1.2 执行确认机制 (Human-in-the-loop, HITL)

当模型决定调用高危工具（如 `bash`）时，必须暂停执行并等待用户授权。这就要求 Runtime 和 TUI 之间存在**双向通信**，且 Runtime 的当前流程必须安全地挂起（Block）。

**设计方案：同步拦截通道**

1. **触发拦截：** Runtime 遍历到需要确认的 ToolCall（例如 `bash`）。
2. **挂起与通知：** Runtime 向 TUI 发送一个 `EventRequireConfirmation` 事件，并立刻在一个专门的确认 Channel 上阻塞等待结果。
3. **用户交互：** TUI 接收到事件，在界面底部弹出一个提示（例如：`[Model wants to run: rm -rf /] Allow? (Y/n)`）。
4. **回传决策：** 用户按下 `Y` 或 `N`，TUI 通过 Runtime 暴露的 `ConfirmAction(callID string, allowed bool)` 方法（该方法底层向阻塞的 Channel 写入 boolean）将结果传回。
5. **恢复执行或阻断：** Runtime 收到结果。如果 `allowed == true`，真正调用 Tool；如果 `false`，中止调用，并将一条特定的 ToolResult（例如 `{"error": "User denied execution"}`）回灌给模型，让模型知道它的操作被拒绝了。

------

## 2. TUI 模块：优雅的事件桥接与状态管理

### 2.1 解耦的事件适配器 (Event Adapter for Bubble Tea)

在 Bubble Tea 中，直接在后台跑一个死循环的 Goroutine 给界面强塞数据容易导致竞争和渲染撕裂。最符合框架哲学的做法是：**将 Channel 的读取包装成单次触发的 tea.Cmd，并在每次收到消息后递归调用自身。**

**设计方案：订阅器模式**

1. **定义载体 Msg：**

   Go

   

   ```
   // 这是专门给 Bubble Tea Update 循环看的 Msg
   type RuntimeMsg struct {
       Event runtime.RuntimeEvent
   }
   // 当 Channel 关闭时的特定 Msg
   type RuntimeClosedMsg struct{} 
   ```

2. **编写非阻塞的监听 Cmd (Adapter)：** 这个函数将作为适配层，每次只从 Channel 读一条数据，读完就返回给 TUI。

   Go

   

   ```
   // 接收 Runtime 的订阅 Channel
   func ListenForRuntimeEvent(sub <-chan runtime.RuntimeEvent) tea.Cmd {
       return func() tea.Msg {
           event, ok := <-sub
           if !ok {
               return RuntimeClosedMsg{} // 订阅结束
           }
           // 将底层 Event 转换为 Bubble Tea Msg
           return RuntimeMsg{Event: event} 
       }
   }
   ```

3. **在 TUI 的 Update 循环中无缝接入：**

   Go

   

   ```
   func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       switch msg := msg.(type) {
   
       // 处理来自 Runtime 的事件
       case RuntimeMsg:
           var cmd tea.Cmd
   
           // 根据具体的 Event Type 决定怎么更新界面
           switch msg.Event.Type {
           case runtime.EventAgentChunk:
               m.chatView.AppendText(msg.Event.Payload.(string))
           case runtime.EventToolChunk:
               m.terminalView.AppendLine(msg.Event.Payload.(string))
           case runtime.EventRequireConfirmation:
               m.showConfirmPrompt = true
           }
   
           // 最关键的一步：处理完当前事件后，再次触发监听 Cmd！
           // 这样就形成了一个“非死循环”的、优雅的事件流监听
           return m, tea.Batch(cmd, ListenForRuntimeEvent(m.runtimeEventCh))
   
       case RuntimeClosedMsg:
           // Runtime 退出，做一些收尾工作
           return m, nil
   
       // ... 处理常规的用户键盘输入 (tea.KeyMsg) 等 ...
       }
   }
   ```

   **优势：** 这种做法彻底隔离了底层的并发复杂性，TUI 的 `Update` 方法依然是单线程、确定性的，极大地降低了死锁和 UI 渲染错乱的风险。

### 2.2 TUI 状态分层建议

由于加入了流式工具和确认框，`UIState` 的结构需要稍作扩充：

Go



```
type UIState struct {
    // 基础对话状态
    ActiveSessionID string
    InputText       string
    
    // 运行状态标识
    IsAgentRunning  bool
    StatusText      string // 例如 "Thinking...", "Running npm install..."
    
    // 拦截与确认状态
    PendingConfirm  *runtime.ToolCall // 如果不为 nil，说明正在等待用户输入 Y/N
    
    // 侧边栏/浮窗状态
    ShowToolTerminal bool // 是否展开显示工具的实时流式输出
}
```