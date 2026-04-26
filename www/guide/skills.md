---
title: Skills 使用
description: 用 SKILL.md 固化工作流提示，让 Agent 在当前会话中按指定方式工作。
---

# Skills 使用

Skills 是 NeoCode 的工作流提示层。它把一段可复用的任务说明、参考资料和工具偏好注入到当前会话上下文里，帮助 Agent 按你指定的方式工作。

Skills 不会绕过权限，不会注册新工具，也不会替代 MCP。它只是告诉 Agent：“这类任务应该这样做”。

## 什么时候用 Skills

| 你想做的事 | 建议 |
|---|---|
| 让 Agent 每次代码审查都按固定清单检查 | 用 Skill |
| 让 Agent 做某类任务前先读指定文档 | 用 Skill |
| 让 Agent 优先考虑某些工具或流程 | 用 Skill |
| 接入一个真实可调用的外部服务 | 用 [MCP](./mcp) |
| 保存长期个人偏好或项目事实 | 用 `/remember` 记忆 |

## Skills 放在哪里

本地 Skills 默认放在：

```text
~/.neocode/skills/
```

一个 Skill 可以是一个目录：

```text
~/.neocode/skills/go-review/SKILL.md
```

也可以直接是根目录下的 `SKILL.md`。更推荐用子目录，这样每个 Skill 都有清晰的名称和边界。

## 创建一个 Skill

示例：`~/.neocode/skills/go-review/SKILL.md`

```md
---
id: go-review
name: Go Review
description: Review Go changes for correctness, boundaries, and tests.
version: v1
scope: explicit
tool_hints:
  - filesystem_grep
  - bash
---

# Go Review

## Instruction

先阅读改动相关的 Go 文件和测试，再给出审查结论。优先关注行为回归、模块边界、错误处理和测试缺口。不要因为风格偏好要求大改。

## References

- title: Repo rules
  path: AGENTS.md
  summary: Follow repository boundaries and testing expectations.

## Examples

- 用户要求 review 时，先列出高风险问题，再给简短总结。

## ToolHints

- filesystem_grep
- bash
```

### 常用字段

| 字段 | 说明 |
|---|---|
| `id` | Skill 标识。没写时会从目录名推导 |
| `name` | 展示名称。没写时会尝试使用第一个一级标题 |
| `description` | 简短说明，用于列表中识别用途 |
| `version` | 版本标记。没写时默认为 `v1` |
| `scope` | 作用范围。没写时默认为 `explicit` |
| `tool_hints` | 工具偏好提示，不等于权限授权 |

### 常用段落

| 段落 | 作用 |
|---|---|
| `Instruction` | 核心工作流说明，最重要 |
| `References` | 参考资料摘要 |
| `Examples` | 示例任务或示例行为 |
| `ToolHints` | 建议优先考虑的工具 |

如果没有写 `Instruction` 段落，NeoCode 会把正文内容当作 instruction 使用。

## 启用和停用

在 TUI 里使用：

```text
/skills                  # 查看当前可用的 Skills
/skill use go-review     # 在当前会话启用某个 Skill
/skill off go-review     # 停用某个 Skill
/skill active            # 查看当前会话已激活的 Skills
```

注意：

- `/skill use <id>` 只影响当前会话。
- 已激活的 Skills 会随 session 记录恢复。
- Skills 管理需要当前有活动 session；如果刚启动还没有会话，先发送一条消息或切换 session。
- Gateway 模式暂不支持 Skills 管理，请切换到 local runtime 使用这些命令。

## Agent 会看到什么

每轮构建上下文时，NeoCode 会把当前激活的 Skills 渲染到 `Skills` section。内容包括：

- Skill 名称和 ID
- `Instruction`
- 最多 3 条 `ToolHints`
- 最多 3 条 `References`
- 最多 2 条 `Examples`

这些内容会影响 Agent 的计划和回复，但不会改变工具权限。比如 Skill 里写了“优先使用 bash”，遇到高风险命令时仍然需要正常审批。

## Skills vs 记忆 vs MCP

| 能力 | 解决什么问题 | 是否执行工具 | 是否跨会话自动生效 |
|---|---|---|---|
| Skills | 当前任务的工作流和任务约束 | 否 | 否，需要在会话中启用 |
| 记忆 | 长期偏好和项目事实 | 否 | 是 |
| MCP | 外部可调用工具 | 是 | 取决于配置是否启用 |

一句话判断：

- 想改变 Agent 做事方式：用 Skills。
- 想让 Agent 记住事实：用记忆。
- 想增加真实工具能力：用 MCP。

## 常见问题

### `/skills` 看不到我的 Skill

检查：

- 文件是否位于 `~/.neocode/skills/`
- 文件名是否为 `SKILL.md`
- Skill 文件大小是否过大
- frontmatter 是否是合法 YAML
- `id` 是否重复

NeoCode 会跳过无效 Skill，但不会因为一个 Skill 写错就阻止其他 Skills 加载。

### Skill 启用了但效果不明显

优先把 `Instruction` 写得更具体。不要只写“请更认真”，而要写清楚检查顺序、输出结构和不该做什么。

更好的写法：

```md
## Instruction

先阅读相关实现和测试，再审查改动。输出时先列风险，再列测试缺口，最后给简短总结。不要要求无关重构。
```

### Skill 能不能授权工具

不能。Skill 里的 `tool_hints` 只是提示 Agent 优先考虑某些工具，不会跳过权限审批，也不会让不存在的工具变得可用。

## 下一步

- 想接入外部工具：[MCP 工具接入](./mcp)
- 想保存长期偏好：[日常使用](./daily-use)
- 想理解权限边界：[工具与权限](./tools-permissions)
