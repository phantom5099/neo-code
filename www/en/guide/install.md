---
title: Install & First Run
description: From installation to your first conversation in 3 minutes.
---

# Install & First Run

## 1. Requirements

- At least one API key (OpenAI, Gemini, OpenLL, Qiniu, or ModelScope)
- Go 1.25+ if running from source

## 2. Install

### One-line install (recommended)

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/1024XEngineer/neo-code/main/scripts/install.sh | bash
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/1024XEngineer/neo-code/main/scripts/install.ps1 | iex
```

### Run from source

```bash
git clone https://github.com/1024XEngineer/neo-code.git
cd neo-code
go run ./cmd/neocode
```

## 3. Set API key

NeoCode reads API keys from environment variables — they are never written to config files.

macOS / Linux:

```bash
export OPENAI_API_KEY="your_key_here"
```

Windows PowerShell:

```powershell
$env:OPENAI_API_KEY = "your_key_here"
```

Other providers:

| Provider | Environment variable |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenLL | `AI_API_KEY` |
| Qiniu | `QINIU_API_KEY` |
| ModelScope | `MODELSCOPE_API_KEY` |

## 4. Launch

```bash
neocode
```

You'll see the TUI interface. Type at the bottom to start a conversation.

To specify a workspace:

```bash
neocode --workdir /path/to/your/project
```

## 5. First conversation

Not sure what to ask? Try these:

```text
Read the current project directory structure and give a module summary
```

```text
Find the tool result injection logic in internal/runtime
```

The agent will automatically use file reading and search tools. When it requests file writes or command execution, the TUI will prompt for your approval.

## 6. Command cheat sheet

Inside the TUI, type `/` commands to perform operations:

| Command | Action |
|---|---|
| `/help` | Show all commands |
| `/provider` | Switch provider |
| `/model` | Switch model |
| `/status` | Show current status |
| `/compact` | Compress long session context |
| `/cwd [path]` | View/switch workspace |
| `/session` | Switch session |
| `/memo` | View memory index |
| `/remember <text>` | Save memory |
| `/forget <keyword>` | Delete memory |
| `/skills` | View available skills |
| `/exit` | Exit NeoCode |

## Installation issues?

See [Troubleshooting](./troubleshooting)

## Next steps

- More usage scenarios: [Usage examples](./examples)
- Switch models or add custom providers: [Configuration](./configuration)
- Daily operations: [Daily use](./daily-use)
