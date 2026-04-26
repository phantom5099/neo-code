---
title: Configuration
description: Minimal config to get running, then adjust models, shell, timeout, and custom providers as needed.
---

# Configuration

## General principles

- `config.yaml` only stores minimal runtime state
- Provider metadata comes from built-in definitions or custom provider files
- API keys are read from environment variables only
- YAML uses strict parsing — unknown fields cause errors

This means NeoCode currently will not:

- Auto-clean legacy `providers` / `provider_overrides` fields
- Auto-compat `workdir`, `default_workdir`, or other legacy fields

## Minimal config

To get NeoCode running, this is all you need:

```yaml
selected_provider: openai
current_model: gpt-5.4
shell: bash
```

Windows users should change `shell` to `powershell`. All other fields have defaults.

Config file location: `~/.neocode/config.yaml`

## Common tasks

### Switch models

Switch directly in the TUI — the selection is saved automatically:

```text
/provider          # Switch provider
/model             # Switch model
```

If the model list is empty, check that the corresponding environment variable is set.

### Change shell

```yaml
shell: powershell    # Windows
shell: bash          # macOS / Linux
```

### Adjust tool timeout

```yaml
tool_timeout_sec: 30    # Default is 20 seconds
```

### Long sessions drifting

Try `/compact` first. If it happens often, increase the retained messages in config:

```yaml
context:
  compact:
    manual_keep_recent_messages: 20    # Default is 10
```

## Full config example

```yaml
selected_provider: openai
current_model: gpt-5.4
shell: bash
tool_timeout_sec: 20
runtime:
  max_no_progress_streak: 3
  max_repeat_cycle_streak: 3
  assets:
    max_session_asset_bytes: 20971520
    max_session_assets_total_bytes: 20971520

tools:
  webfetch:
    max_response_bytes: 262144
    supported_content_types:
      - text/html
      - text/plain
      - application/json

context:
  compact:
    manual_strategy: keep_recent
    manual_keep_recent_messages: 10
    micro_compact_retained_tool_spans: 6
    read_time_max_message_spans: 24
    max_summary_chars: 1200
    micro_compact_disabled: false
  budget:
    prompt_budget: 0
    reserve_tokens: 13000
    fallback_prompt_budget: 100000
    max_reactive_compacts: 3
```

## Field reference

### Basic fields

| Field | Description |
|-------|-------------|
| `selected_provider` | Currently selected provider name |
| `current_model` | Currently selected model ID |
| `shell` | Default shell. Windows defaults to `powershell`, other platforms to `bash` |
| `tool_timeout_sec` | Tool execution timeout in seconds |

### `context` fields

| Field | Description |
|-------|-------------|
| `context.compact.manual_strategy` | `/compact` strategy: `keep_recent` / `full_replace` |
| `context.compact.manual_keep_recent_messages` | Number of recent messages to keep under `keep_recent` |
| `context.compact.micro_compact_retained_tool_spans` | Number of recent compressible tool blocks to retain original content for, default `6` |
| `context.compact.read_time_max_message_spans` | Upper limit of message spans retained at context read time |
| `context.compact.max_summary_chars` | Maximum characters for compact summary |
| `context.compact.micro_compact_disabled` | Whether to disable the default micro compact |
| `context.budget.prompt_budget` | Explicit input budget; `> 0` uses directly, `0` means auto-derive |
| `context.budget.reserve_tokens` | Buffer reserved from model window for output, tool calls, and system prompt when auto-deriving |
| `context.budget.fallback_prompt_budget` | Fallback input budget when model window is unavailable or derivation fails |
| `context.budget.max_reactive_compacts` | Maximum reactive compact count allowed within a single Run |

### `runtime` fields

| Field | Description |
|-------|-------------|
| `runtime.max_no_progress_streak` | Threshold for consecutive "no progress" turn warnings, default `5` |
| `runtime.max_repeat_cycle_streak` | Threshold for consecutive "same tool same args" warnings, default `3` |
| `runtime.max_turns` | Maximum reasoning turns per Run, default `40` |
| `runtime.assets.max_session_asset_bytes` | Maximum bytes for a single session asset, default 20 MiB |
| `runtime.assets.max_session_assets_total_bytes` | Total byte limit for session assets per request, default 20 MiB |

### `verification` fields

| Field | Description |
|-------|-------------|
| `verification.enabled` | Whether to enable the verification engine, default `true` |
| `verification.final_intercept` | Whether to intercept and trigger verification before task completion, default `true` |
| `verification.max_no_progress` | Maximum retries when verification shows no progress, default `3` |
| `verification.max_retries` | Maximum retries after verification failure, default `2` |
| `verification.verifiers.<name>.enabled` | Whether to enable this verifier |
| `verification.verifiers.<name>.required` | Whether this verifier is a hard requirement |
| `verification.verifiers.<name>.timeout_sec` | Execution timeout for this verifier |
| `verification.verifiers.<name>.fail_closed` | Whether to treat verifier errors as failures |
| `verification.execution_policy.allowed_commands` | Command whitelist for verifiers |
| `verification.execution_policy.denied_commands` | Command blacklist for verifiers |

### `tools` fields

| Field | Description |
|-------|-------------|
| `tools.webfetch.max_response_bytes` | Maximum response bytes for WebFetch |
| `tools.webfetch.supported_content_types` | Allowed content types for WebFetch |
| `tools.mcp.servers` | MCP server list, see MCP configuration below |

## Environment variables

API keys are read from environment variables only, never written to config files.

| Provider | Environment variable |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenLL | `AI_API_KEY` |
| Qiniu | `QINIU_API_KEY` |
| ModelScope | `MODELSCOPE_API_KEY` |

```bash
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="AI..."
export MODELSCOPE_API_KEY="ms-..."
```

## Custom providers

If your model service isn't in the built-in list, you can add it via a config file.

Config file location: `~/.neocode/providers/<name>/provider.yaml`

Example (OpenAI-compatible endpoint):

```yaml
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
model_source: discover
base_url: https://llm.example.com/v1
chat_api_mode: chat_completions
chat_endpoint_path: /chat/completions
discovery_endpoint_path: /models
```

You can also use `/provider add` in the TUI to add interactively.

### Manual model list

If the provider does not support model discovery, use `model_source: manual`:

```yaml
name: company-gateway
driver: openaicompat
api_key_env: COMPANY_GATEWAY_API_KEY
model_source: manual
base_url: https://llm.example.com/v1
chat_endpoint_path: /chat/completions
models:
  - id: gpt-4o-mini
    name: GPT-4o Mini
    context_window: 128000
```

### Custom provider field reference

| Field | Description |
|-------|-------------|
| `name` | Provider identifier, used in `selected_provider` |
| `driver` | Driver type. Currently supports `openaicompat` |
| `api_key_env` | Environment variable name for the API key |
| `model_source` | `discover` (auto) or `manual` (explicit list) |
| `base_url` | Service base URL |
| `chat_api_mode` | `chat_completions` or `responses` |
| `chat_endpoint_path` | Chat endpoint path |
| `discovery_endpoint_path` | Model discovery path (`discover` mode only) |

## MCP tools

If you have an MCP server, register it in `config.yaml` through `tools.mcp.servers`. NeoCode currently supports `stdio` servers, and registered tools are named as `mcp.<server-id>.<tool>`.

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

For the full field reference, exposure policy, verification prompts, and troubleshooting steps, see [MCP Tools](./mcp).

## Fields not allowed in config.yaml

These fields cause a startup error if present in the main config file:

`providers`, `provider_overrides`, `workdir`, `default_workdir`, `base_url`, `api_key_env`, `models`

## Common errors

### Legacy fields rejected

If `config.yaml` contains `workdir`, `providers`, etc., the current version will report an unknown field error. Remove these fields manually.

### Legacy `context.auto_compact` field

If only `context.auto_compact` exists in config, preflight will auto-migrate it to `context.budget` and write a `config.yaml.bak` backup. If both `context.auto_compact` and `context.budget` are present, startup will error — merge them manually before restarting.

### API key not set

```text
config: environment variable OPENAI_API_KEY is empty
```

Set the corresponding environment variable in your current shell, then launch NeoCode.

## CLI argument overrides

Working directory is not written to `config.yaml` — override it via the launch argument:

```bash
neocode --workdir /path/to/workspace
```

## Next steps

- Daily operations: [Daily use](./daily-use)
- What the agent can do: [Tools & permissions](./tools-permissions)
- Something wrong: [Troubleshooting](./troubleshooting)
- Check for updates: [Update & Version](./update)
