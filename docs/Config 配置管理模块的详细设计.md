# 模块细化设计：Config 模块与多模型切换

本文档细化了 NeoCode MVP 架构中配置文件的加载、热重载机制、并发安全访问，以及如何在 TUI 中优雅地进行多模型和 Provider 的动态切换。

## 1. 配置的并发安全与热重载 (Thread Safety & Hot Reload)

由于 TUI 的渲染线程（Update 循环）和 Agent Runtime 的推理线程是并发运行的，配置文件不能是一个简单的全局静态变量。

### 1.1 线程安全访问设计 (RWMutex)

建议封装一个 `ConfigManager`，使用读写锁 (`sync.RWMutex`) 或原子操作 (`atomic.Value`) 来保证并发安全。

Go



```
// internal/config/manager.go
type Manager struct {
    mu     sync.RWMutex
    config *Config
    path   string
}

// Get 返回当前配置的深拷贝或只读副本，防止外部意外修改
func (m *Manager) Get() Config {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return *m.config // 结构体值拷贝
}

// Update 更新内存配置，并可选择是否持久化到文件
func (m *Manager) Update(fn func(c *Config)) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 执行更新逻辑
    fn(m.config)
    
    // 可选：将新配置写回 ~/.neocode/config.yaml
    return m.saveToFile()
}
```

### 1.2 热重载触发机制

在 MVP 阶段，你有两种选择：

- **选项 A（主动触发）:** 用户在 TUI 中按下特定快捷键（如 `Ctrl+R`），或者输入命令 `:reload`，TUI 调用 `ConfigManager.Reload()` 重新读取 YAML 文件。
- **选项 B（被动监听，进阶）:** 使用 `fsnotify` 库监听 `~/.neocode/config.yaml`。文件一旦修改，后台静默重载，并通过 Channel 发送一个 `ConfigReloadedMsg` 给 TUI，TUI 状态栏闪烁提示“配置已更新”。
- **MVP 建议:** 采用 **选项 A**。实现简单，且能避免用户在编辑器里保存了一半导致解析错误的边际情况。

------

## 2. 多模型与 Provider 动态切换

在实际编码中，用户经常需要根据任务复杂度切换模型（例如：写大段逻辑用 `claude-3-7-sonnet`，问简单语法用 `gpt-4o-mini`）。

### 2.1 TUI 交互设计

- **快捷键唤出:** 按下 `Ctrl+M`，TUI 弹出一个浮层列表（使用 Bubble Tea 的 `list` 组件）。
- **数据来源:** 列表数据直接来源于 `ConfigManager.Get().Providers`。
- **切换动作:** 用户选中目标模型并回车后，触发以下流程：

### 2.2 切换时的状态流转

当用户在 TUI 中切换了模型，系统需要做两件事：更新配置，并通知底层重建 HTTP Client。

1. **TUI 发起更新:**

   Go

   

   ```
   // TUI 内部逻辑
   configManager.Update(func(c *Config) {
       c.SelectedProvider = "anthropic"
       c.CurrentModel = "claude-3-7-sonnet-latest"
   })
   ```

2. **Runtime 动态获取:**

   - **解耦的关键:** Runtime **不要**在初始化时就把 Provider 实例“写死”。
   - **工厂模式:** 每次开始新一轮 `Run()` 之前，Runtime 都应该调用一个 `ProviderFactory`，传入当前的 `Config`。
   - 如果 `SelectedProvider` 变了，工厂会返回一个新的 Provider 实例（包含全新的 BaseURL 和 Auth Header）。

   Go

   

   ```
   // internal/runtime/executor.go
   func (r *Runtime) Run(ctx context.Context, input UserInput) error {
       // 1. 每次 Run 的最开始，获取最新配置
       cfg := r.configManager.Get()
   
       // 2. 动态构建或获取当前 Provider
       provider, err := r.providerFactory.Build(cfg.SelectedProvider, cfg)
       if err != nil {
           return err
       }
   
       // 3. 继续后续的对话与工具调用循环...
   }
   ```

   - **优势:** 这样设计，哪怕用户在对话进行到一半（但还没点发送）时切换了模型，下一条消息也会无缝使用新模型发送，且不需要重启整个应用。

------

## 3. 凭证与环境变量管理 (Secrets & Env)

`config.yaml` 绝对不能明文存储 API Key。这在开源工具中是致命的安全隐患。

### 3.1 环境变量映射机制

- 在配置结构体中只保留环境变量的**键名**。
- 启动时，`ConfigManager` 负责从系统环境变量或 `.env` 文件中提取实际的 Key。

YAML



```
# ~/.neocode/config.yaml
providers:
  - name: openai
    type: openai
    model: gpt-4.5-preview
    api_key_env: OPENAI_API_KEY # 只存变量名
```

Go



```
// internal/config/loader.go
func (m *Manager) Load() error {
    // 1. 尝试加载当前目录的 .env 文件 (使用 godotenv 库)
    _ = godotenv.Load() 
    
    // ... 解析 YAML 到 config 结构体 ...

    // 2. 校验密钥是否存在
    for _, p := range config.Providers {
        if os.Getenv(p.APIKeyEnv) == "" {
            log.Printf("Warning: Environment variable %s not set for provider %s", p.APIKeyEnv, p.Name)
        }
    }
    return nil
}
```