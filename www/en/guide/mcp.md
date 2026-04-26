---
title: MCP Tools
description: Connect external MCP servers to NeoCode and let the agent call them through the normal tool system.
---

# MCP Tools

MCP is useful when you already have external capabilities that NeoCode should call, such as internal documentation search, issue lookup, private platform operations, or team-specific automation.

In NeoCode, MCP tools are not a special bypass. They enter the normal tool registry and still follow tool naming, approval, and exposure rules.

## When to use MCP

| Goal | Recommendation |
|---|---|
| Let the agent query internal docs, issue systems, or private platforms | Use MCP |
| Add a real callable external tool | Use MCP |
| Make the agent follow a workflow | Use [Skills](./skills) |
| Save personal preferences or project facts | Use `/remember` memory |

## Current support

NeoCode currently supports `stdio` MCP servers only. NeoCode starts a local child process from your config and communicates with it through standard input and output.

After registration, tool names use this format:

```text
mcp.<server-id>.<tool-name>
```

For example, if the server id is `docs` and it exposes a `search` tool, the full tool name is `mcp.docs.search`.

## Configure an MCP server

Config file:

```text
~/.neocode/config.yaml
```

Minimal example:

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

### Field reference

| Field | Description |
|---|---|
| `id` | Stable server identifier, used in `mcp.<id>.<tool>` |
| `enabled` | Only `true` servers are registered on startup |
| `source` | Currently only `stdio` is supported. Empty also means `stdio` |
| `version` | Version label for your own config tracking |
| `stdio.command` | Startup command. Required when the server is enabled |
| `stdio.args` | Startup arguments |
| `stdio.workdir` | Child process working directory. Relative paths are supported |
| `stdio.start_timeout_sec` | Startup timeout |
| `stdio.call_timeout_sec` | Per-call tool timeout |
| `stdio.restart_backoff_sec` | Restart backoff |
| `env` | Environment variables passed to the MCP child process |

::: tip
Put secrets in system environment variables and reference them with `value_env`. Do not write tokens, API keys, or passwords directly into `config.yaml`.
:::

## Environment variables

Each `env` entry must set `name`, and must set exactly one of `value` or `value_env`.

Recommended:

```yaml
env:
  - name: MCP_TOKEN
    value_env: MCP_TOKEN
```

This reads the system environment variable `MCP_TOKEN` from the shell that starts NeoCode, then passes it into the MCP child process as `MCP_TOKEN`.

Not recommended:

```yaml
env:
  - name: MCP_TOKEN
    value: real-token-here
```

## Startup behavior

On startup, NeoCode:

1. Reads `tools.mcp.servers`.
2. Skips servers with `enabled: false`.
3. Starts each enabled `stdio` server.
4. Calls `tools/list` once to build the initial tool snapshot.
5. Registers tools as `mcp.<server-id>.<tool-name>`.

If an enabled server fails to start, misses an environment variable, or fails `tools/list`, NeoCode fails startup. This is intentional: a broken tool config should be visible immediately.

## Control exposed MCP tools

If a server exposes many tools, use `exposure` to control what the agent can see.

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

Recommended use:

| Config | Effect |
|---|---|
| `allowlist` | Expose only matching MCP tools |
| `denylist` | Hide matching MCP tools. Deny wins over allow |
| `agents` | Set visible tools by agent name |

Patterns can be full tool names or globs, such as `mcp.docs.*`. To target a server, use `mcp.docs`.

## Verify availability

After starting NeoCode, ask the agent to list tools:

```text
List all the tools you currently have available.
```

After confirming `mcp.docs.search` is present, make a direct call:

```text
Call mcp.docs.search with {"query":"hello"} and return the tool result.
```

If the tool requires specific arguments, follow your MCP server schema.

## Common issues

### `tool not found`

Check in order:

- `enabled` is `true`
- `id` is correct, so the tool name is `mcp.<id>.<tool>`
- `stdio.command` is executable in the current environment
- `stdio.workdir` points to the right directory
- system variables referenced by `env.value_env` are set
- the MCP server supports `tools/list`
- `exposure.allowlist` or `exposure.denylist` did not filter the tool out

### Startup says an environment variable is empty

If your config contains:

```yaml
env:
  - name: MCP_TOKEN
    value_env: MCP_TOKEN
```

you must set `MCP_TOKEN` in the same shell that starts NeoCode.

### The server starts, but calls fail

Check the MCP server logs, the tool input schema, and `stdio.call_timeout_sec`. NeoCode wraps call failures as tool errors, but the business error usually comes from the MCP server.

## Next steps

- Control agent workflow: [Skills](./skills)
- Understand approvals: [Tools & Permissions](./tools-permissions)
- Review full config: [Configuration](./configuration)
