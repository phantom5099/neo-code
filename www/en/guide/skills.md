---
title: Skills
description: Use SKILL.md files to codify workflow guidance for the current NeoCode session.
---

# Skills

Skills are NeoCode's workflow guidance layer. A Skill injects reusable task instructions, references, examples, and tool preferences into the current session context so the agent works in a specific way.

Skills do not bypass approval, register tools, or replace MCP. They tell the agent how to approach a class of tasks.

## When to use Skills

| Goal | Recommendation |
|---|---|
| Make code reviews follow a fixed checklist | Use a Skill |
| Make the agent read specific docs before a task | Use a Skill |
| Encourage a certain workflow or tool preference | Use a Skill |
| Add a real callable external service | Use [MCP](./mcp) |
| Save long-term personal preferences or project facts | Use `/remember` memory |

## Where Skills live

Local Skills are loaded from:

```text
~/.neocode/skills/
```

A Skill can be a directory:

```text
~/.neocode/skills/go-review/SKILL.md
```

A root-level `SKILL.md` is also supported. Subdirectories are clearer because each Skill gets its own name and boundary.

## Create a Skill

Example: `~/.neocode/skills/go-review/SKILL.md`

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

Read the relevant Go files and tests before reviewing. Focus on behavior regressions, module boundaries, error handling, and test gaps. Do not request broad refactors for style preferences.

## References

- title: Repo rules
  path: AGENTS.md
  summary: Follow repository boundaries and testing expectations.

## Examples

- When the user asks for a review, list high-risk findings first, then a short summary.

## ToolHints

- filesystem_grep
- bash
```

### Common fields

| Field | Description |
|---|---|
| `id` | Skill identifier. If omitted, NeoCode derives it from the directory name |
| `name` | Display name. If omitted, NeoCode tries the first H1 heading |
| `description` | Short explanation shown in lists |
| `version` | Version label. Defaults to `v1` |
| `scope` | Scope. Defaults to `explicit` |
| `tool_hints` | Tool preference hints. This is not authorization |

### Common sections

| Section | Purpose |
|---|---|
| `Instruction` | Main workflow guidance. This is the most important part |
| `References` | Reference summaries |
| `Examples` | Example tasks or expected behavior |
| `ToolHints` | Suggested tools to consider |

If there is no `Instruction` section, NeoCode uses the body as the instruction.

## Activate and deactivate

Use these commands in the TUI:

```text
/skills                  # View available skills
/skill use go-review     # Activate a skill in current session
/skill off go-review     # Deactivate a skill
/skill active            # View active skills in current session
```

Notes:

- `/skill use <id>` affects only the current session.
- Activated Skills are restored with the session record.
- Skill management requires an active session. If NeoCode just started, send one message or switch sessions first.
- Gateway mode does not currently support Skills management. Use local runtime for these commands.

## What the agent sees

For each turn, NeoCode renders active Skills into the `Skills` context section. It includes:

- Skill name and ID
- `Instruction`
- up to 3 `ToolHints`
- up to 3 `References`
- up to 2 `Examples`

This affects planning and responses, but not tool permissions. For example, a Skill can say "prefer bash", but high-risk commands still go through normal approval.

## Skills vs memory vs MCP

| Capability | Solves | Executes tools | Automatically applies across sessions |
|---|---|---|---|
| Skills | Current task workflow and constraints | No | No, activate per session |
| Memory | Long-term preferences and project facts | No | Yes |
| MCP | External callable tools | Yes | Depends on config |

Quick rule:

- Change how the agent works: use Skills.
- Make the agent remember a fact: use memory.
- Add real tool capability: use MCP.

## Common issues

### `/skills` does not show my Skill

Check:

- The file is under `~/.neocode/skills/`
- The filename is `SKILL.md`
- The Skill file is not too large
- Frontmatter is valid YAML
- `id` is not duplicated

NeoCode skips invalid Skills, but one broken Skill does not block all others from loading.

### The Skill is active but has little effect

Make `Instruction` more specific. Avoid vague text like "be more careful"; define the reading order, output structure, and what not to do.

Better:

```md
## Instruction

Read the related implementation and tests first. Output risks first, then test gaps, then a short summary. Do not request unrelated refactors.
```

### Can a Skill authorize tools?

No. `tool_hints` only suggests tools for the agent to consider. It does not skip approval and does not make unavailable tools available.

## Next steps

- Connect external tools: [MCP Tools](./mcp)
- Save long-term preferences: [Daily Use](./daily-use)
- Understand permission boundaries: [Tools & Permissions](./tools-permissions)
