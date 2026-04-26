---
title: Daily Use
description: Session management, memory, skills, and subagents — the operations you use every day.
---

# Daily Use

## Session management

### Switch workspace

```text
/cwd                     # View current workspace
/cwd /path/to/project    # Switch to another project
```

### Switch session

```text
/session                 # Open session picker, switch to another session
```

### Compress long sessions

When the conversation gets too long, agent quality drops. Run a compression to clean up old context:

```text
/compact
```

### New session vs. continue

| Scenario | Recommendation |
|---|---|
| Finished a feature, starting unrelated bug fix | New session |
| Continuing to refine the same feature | Continue current session |
| Switching to a completely different project | New session + switch workspace |
| Session is long and responses drift | Try `/compact` first, then new session if needed |

## Memory

Memory saves preferences and project facts across sessions — no need to repeat yourself.

### Common operations

```text
/memo                              # View all memories
/remember I prefer powershell      # Save a memory
/forget powershell                 # Delete matching memory
```

### Memory vs. Skills

- **Memory**: Saves facts and preferences, persists across sessions. E.g. "I prefer powershell", "This project uses Go 1.25"
- **Skills**: Saves workflow hints, active in current session. E.g. "Read before modifying"

Quick rule: Need it across sessions → memory. Need a special workflow for the current task → skill.

## Skills

Skills are workflow hints that influence agent behavior in the current session. For authoring and loading rules, see [Skills](./skills).

### Common operations

```text
/skills                  # View available skills
/skill use go-review     # Activate a skill in current session
/skill off go-review     # Deactivate a skill
/skill active            # View active skills in current session
```

Quick rule: need a special workflow for the current task → enable a Skill; need a long-term fact → use memory; need a real external tool → use [MCP](./mcp).

## Subagents

The agent can launch subagents to handle subtasks in parallel — e.g. a researcher to search, a reviewer to check results. You don't need to trigger this manually; the agent decides when to use subagents.

If you want to encourage subagent use, say something like:

```text
Use the researcher role to search all compact-related function signatures in internal/runtime
Use the reviewer role to check if the recent changes meet test coverage requirements
```

## Next steps

- Configure models and providers: [Configuration](./configuration)
- What the agent can do and when it needs approval: [Tools & permissions](./tools-permissions)
- Write or activate a Skill: [Skills](./skills)
- Something wrong: [Troubleshooting](./troubleshooting)
