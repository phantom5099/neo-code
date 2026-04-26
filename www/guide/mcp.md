---
title: MCP 工具接入
description: 把外部 MCP server 接入 NeoCode，并让 Agent 安全地调用这些工具。
---

# MCP 工具接入

MCP 适合把你已有的外部能力接进 NeoCode，例如内部文档搜索、Issue 查询、私有平台操作或团队已有的工具服务。

在 NeoCode 里，MCP 工具不是特殊通道。它们会进入统一工具注册表，继续遵守工具命名、权限审批和暴露策略。

## 什么时候用 MCP

| 你想做的事 | 建议 |
|---|---|
| 让 Agent 查询公司文档、任务系统或私有平台 | 用 MCP |
| 给 Agent 增加一个真实可调用的外部工具 | 用 MCP |
| 只是让 Agent 按某个流程工作 | 用 [Skills](./skills) |
| 只是保存个人偏好或项目事实 | 用 `/remember` 记忆 |

## 当前支持范围

NeoCode 当前只支持 `stdio` 类型的 MCP server。也就是说，NeoCode 会按配置启动一个本地子进程，通过标准输入输出与它通信。

工具注册成功后，工具名会变成：

```text
mcp.<server-id>.<tool-name>
```

例如 server id 是 `docs`，它提供 `search` 工具，那么 Agent 看到的完整工具名就是 `mcp.docs.search`。

## 配置 MCP server

配置文件位置：

```text
~/.neocode/config.yaml
```

最小示例：

```yaml
tools:
  mcp:
    servers:
      - id: docs
        enabled: true
        source: stdio
        version: v1
        stdio:
          command: node
          args:
            - ./mcp-server.js
          workdir: ./mcp
          start_timeout_sec: 8
          call_timeout_sec: 20
          restart_backoff_sec: 1
        env:
          - name: MCP_TOKEN
            value_env: MCP_TOKEN
```

### 字段说明

| 字段 | 说明 |
|---|---|
| `id` | MCP server 的稳定标识，会出现在 `mcp.<id>.<tool>` 里 |
| `enabled` | 只有 `true` 的 server 会在启动时注册 |
| `source` | 当前仅支持 `stdio`，不填时也按 `stdio` 处理 |
| `version` | 版本标记，主要用于你自己识别配置 |
| `stdio.command` | 启动命令。server 启用时必填 |
| `stdio.args` | 启动参数列表 |
| `stdio.workdir` | 子进程工作目录，支持相对路径 |
| `stdio.start_timeout_sec` | 启动超时 |
| `stdio.call_timeout_sec` | 单次工具调用超时 |
| `stdio.restart_backoff_sec` | 重启退避时间 |
| `env` | 传给 MCP 子进程的环境变量 |

::: tip
密钥建议放在系统环境变量里，然后用 `value_env` 引用。不要把 token、API Key 或密码直接写进 `config.yaml`。
:::

## 环境变量

`env` 每一项必须设置 `name`，并且只能在 `value` 和 `value_env` 里二选一。

推荐写法：

```yaml
env:
  - name: MCP_TOKEN
    value_env: MCP_TOKEN
```

这表示 NeoCode 从当前系统环境变量 `MCP_TOKEN` 读取值，再传给 MCP 子进程里的 `MCP_TOKEN`。

不推荐写法：

```yaml
env:
  - name: MCP_TOKEN
    value: real-token-here
```

## 启动行为

NeoCode 启动时会：

1. 读取 `tools.mcp.servers`。
2. 跳过 `enabled: false` 的 server。
3. 启动每个启用的 `stdio` server。
4. 调用一次 `tools/list`，初始化工具快照。
5. 把工具注册成 `mcp.<server-id>.<tool-name>`。

如果某个启用的 server 启动失败、环境变量缺失或 `tools/list` 失败，NeoCode 会直接启动失败。这是有意的：配置写错时尽早暴露，比运行中才发现工具不可用更清楚。

## 控制哪些 MCP 工具可见

如果你接入的 MCP server 暴露了很多工具，可以用 `exposure` 控制 Agent 能看到哪些工具。

```yaml
tools:
  mcp:
    exposure:
      allowlist:
        - mcp.docs.*
      denylist:
        - mcp.docs.delete*
      agents:
        - agent: default
          allowlist:
            - mcp.docs.search
    servers:
      - id: docs
        enabled: true
        source: stdio
        stdio:
          command: node
          args:
            - ./mcp-server.js
```

规则建议：

| 配置 | 作用 |
|---|---|
| `allowlist` | 只允许匹配的 MCP 工具暴露给 Agent |
| `denylist` | 隐藏匹配的 MCP 工具，优先级高于 allowlist |
| `agents` | 按 agent 名称设置可见工具 |

匹配项可以写完整工具名，也可以写通配符，例如 `mcp.docs.*`。如果只想控制某个 server，也可以写 `mcp.docs`。

## 验证是否可用

启动 NeoCode 后，先让 Agent 列出工具：

```text
请先列出你当前可用工具的完整名称。
```

确认列表里有 `mcp.docs.search` 之后，再做一次明确调用：

```text
请调用 mcp.docs.search，参数 {"query":"hello"}，并返回工具结果。
```

如果工具需要特定参数，以你的 MCP server 的 schema 为准。

## 常见问题

### `tool not found`

按顺序检查：

- `enabled` 是否为 `true`
- `id` 是否写对，工具名是否应该是 `mcp.<id>.<tool>`
- `stdio.command` 是否在当前环境里可执行
- `stdio.workdir` 是否指向正确目录
- `env.value_env` 对应的系统环境变量是否已设置
- MCP server 是否支持 `tools/list`
- `exposure.allowlist` 或 `exposure.denylist` 是否把工具过滤掉了

### 启动时报环境变量为空

如果配置了：

```yaml
env:
  - name: MCP_TOKEN
    value_env: MCP_TOKEN
```

就必须先在启动 NeoCode 的同一个 shell 中设置 `MCP_TOKEN`。

### server 能启动，但工具调用失败

优先检查 MCP server 自己的日志、工具参数 schema 和 `stdio.call_timeout_sec`。NeoCode 会把调用错误收敛成工具错误返回，但具体业务错误通常来自 MCP server。

## 下一步

- 想控制 Agent 的工作流：[Skills 使用](./skills)
- 想理解权限审批：[工具与权限](./tools-permissions)
- 想查看完整配置：[配置指南](./configuration)
