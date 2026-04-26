---
title: 日常使用
description: 会话管理、记忆、Skills 和子代理——你每天都在用的操作。
---

# 日常使用

## 会话管理

### 切换会话

```text
/session                 # 打开会话选择器，切换到其他会话
```

### 压缩长会话

对话太长时，Agent 的回复质量会下降。执行一次压缩可以清理旧上下文：

```text
/compact
```

### 什么时候新建会话，什么时候继续

| 场景 | 建议 |
|---|---|
| 刚完成一个功能，要开始不相关的 bug 修复 | 新建会话 |
| 同一仓库内继续完善刚才的功能 | 继续当前会话 |
| 切换到完全不同的项目 | 新建会话 + 切换工作区 |
| 会话已经很长，回复开始跑偏 | 先 `/compact`，不行就新建会话 |

## 记忆

记忆帮你保存跨会话的偏好和项目事实，不用每次重复告诉 Agent。

### 常用操作

```text
/memo                              # 查看所有记忆
/remember 我习惯用 powershell       # 保存一条记忆
/forget powershell                  # 删除匹配的记忆
```

### 记忆 vs Skills

- **记忆**：保存事实和偏好，跨会话生效。例如"我习惯用 powershell"、"本项目用 Go 1.25"
- **Skills**：保存工作流提示，当前会话生效。例如"先静态阅读再修改"

一句话决策：需要跨会话记住 → 用记忆；当前任务需要特殊工作方式 → 用 Skill。

## Skills

Skills 是工作流提示层，影响 Agent 在当前会话中的行为偏好。详细写法和加载规则见 [Skills 使用](./skills)。

### 常用操作

```text
/skills                  # 查看当前可用的 Skills
/skill use go-review     # 在当前会话启用某个 Skill
/skill off go-review     # 停用某个 Skill
/skill active            # 查看当前会话已激活的 Skills
```

一句话判断：当前任务需要特殊工作方式 → 启用 Skill；需要长期记住事实 → 用记忆；需要真实外部工具 → 用 [MCP](./mcp)。

## 子代理

Agent 可以启动子代理来并行处理子任务，比如用一个 researcher 搜索信息，用一个 reviewer 审查结果。你不需要手动触发——Agent 会根据任务需要自行决定是否使用子代理。

如果你想让 Agent 优先使用子代理，可以在对话中这样说：

```text
请用 researcher 角色搜索 internal/runtime 下所有与 compact 相关的函数签名
请用 reviewer 角色审查刚才的修改是否满足测试覆盖率要求
```

## 下一步

- 想配置模型和 Provider：[配置指南](./configuration)
- 想了解 Agent 能做什么、权限怎么选：[工具与权限](./tools-permissions)
- 想编写或启用 Skill：[Skills 使用](./skills)
- 遇到问题：[排障与常见问题](./troubleshooting)
