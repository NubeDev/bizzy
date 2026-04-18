# OpenCode — Free Offline Coding Agent with MCP

OpenCode is an open-source CLI coding agent (like Claude Code) that supports MCP, local models, file editing, shell execution, and agentic workflows. It can connect to bizzy's MCP server to use all your installed app tools with free local or cloud models.

- Site: https://opencode.ai
- GitHub: https://github.com/opencode-ai/opencode
- Version installed: 1.4.11
- Binary: `~/.opencode/bin/opencode`

---

## Why OpenCode for Bizzy

| Requirement | OpenCode | Claude Code |
|---|---|---|
| MCP client (connects to bizzy `/mcp`) | Yes | Yes |
| Local models via Ollama | Yes (via Ollama Cloud / custom provider) | Yes (via `ollama launch claude`) |
| Free cloud models | Yes (4 free models built-in) | No |
| File editing, shell, git | Yes | Yes |
| Subagents (explore, plan, build) | Yes | Yes |
| Sessions / continue | Yes | Yes |
| One-shot mode | Yes (`opencode run "prompt"`) | Yes (`claude -p "prompt"`) |
| Fully offline | Yes | No (needs Anthropic API) |
| Cost | Free | Paid API |

---

## Install

```bash
curl -fsSL https://opencode.ai/install | bash
```

Binary installs to `~/.opencode/bin/opencode`.

### Shell alias

Add to `~/.bashrc`:

```bash
export PATH="$HOME/.opencode/bin:$PATH"
alias oc="opencode"
```

---

## Quick Start

### Interactive mode (like `claude`)

```bash
opencode
```

### One-shot mode (like `claude -p`)

```bash
opencode run "list all Go files in pkg/"
```

### Continue last session

```bash
opencode --continue
```

### With a specific model

```bash
opencode -m opencode/big-pickle
opencode -m ollama-cloud/kimi-k2.5
```

---

## Models

### Free built-in models (no API key needed)

```bash
opencode models opencode
```

| Model | Notes |
|---|---|
| `opencode/big-pickle` | Strong coding, 200k context |
| `opencode/gpt-5-nano` | 400k context, reasoning |
| `opencode/minimax-m2.5-free` | Free tier |
| `opencode/nemotron-3-super-free` | Free tier |

### Ollama Cloud models (free, remote Ollama hosting)

```bash
opencode models ollama-cloud
```

Requires `OLLAMA_API_KEY` — get one from https://ollama.com/cloud.

Good models for coding:

| Model | Context | Tool calling | Notes |
|---|---|---|---|
| `ollama-cloud/kimi-k2.5` | 262k | Yes | Strong coding, multimodal |
| `ollama-cloud/qwen3.5:397b` | 262k | Yes | Large, reasoning |
| `ollama-cloud/qwen3-coder-next` | 262k | Yes | Code-specialised |
| `ollama-cloud/deepseek-v3.2` | 164k | Yes | Reasoning |
| `ollama-cloud/devstral-2:123b` | 262k | Yes | Code-specialised |
| `ollama-cloud/glm-5.1` | 203k | Yes | Reasoning |
| `ollama-cloud/gemma4:31b` | 262k | Yes | Multimodal |

### Local Ollama models

OpenCode can connect to a local Ollama instance via the custom provider login or by using the LMStudio-compatible endpoint:

```bash
# Login to a local Ollama provider (OpenAI-compatible API)
opencode providers login http://localhost:11434/v1 --provider ollama
```

Or set the environment variable to point ollama-cloud at your local instance:

```bash
OLLAMA_BASE_URL=http://localhost:11434/v1 opencode -m ollama-cloud/gemma4:e4b
```

### Other providers

OpenCode supports 100+ providers. List all:

```bash
opencode models
opencode models --verbose
```

Configure cloud providers:

```bash
opencode providers login --provider anthropic
opencode providers login --provider openai
opencode providers login --provider google
```

---

## MCP — Connecting to Bizzy

This is the key integration: OpenCode acts as an MCP client and connects to bizzy's MCP server, giving it access to all your installed app tools (rubix, marketing, etc.).

### Add bizzy as an MCP server

```bash
opencode mcp add
```

Follow the interactive prompts, or create the config manually. The MCP config is stored per-project or globally.

### Manual MCP config

Create/edit the project-level `.opencode/mcp.json` or use the CLI:

```json
{
  "nube": {
    "type": "streamable-http",
    "url": "http://localhost:8090/mcp",
    "headers": {
      "Authorization": "Bearer <your-bizzy-token>"
    }
  }
}
```

### Verify MCP connection

```bash
opencode mcp list
```

This should show the `nube` server and all discovered tools from your installed apps.

### What this gives you

Once connected, OpenCode can call any tool from your bizzy app installs:

```
> query all offline devices

[calls rubix.query_nodes with filter for offline status]
Found 3 offline devices:
- Device A (floor 3)
- Device B (floor 5)  
- Device C (floor 8)
```

The AI sees the same tools that Claude Code sees when connected to bizzy — `rubix.query_nodes`, `rubix.create_node`, `rubix-developer.device_summary`, etc.

---

## CLI Reference

### Core commands

| Command | Description |
|---|---|
| `opencode` | Start interactive TUI (like `claude`) |
| `opencode run "message"` | One-shot mode (like `claude -p`) |
| `opencode -c` | Continue last session |
| `opencode -s <id>` | Resume specific session |
| `opencode -m provider/model` | Use specific model |

### MCP management

| Command | Description |
|---|---|
| `opencode mcp add` | Add an MCP server |
| `opencode mcp list` | List MCP servers and status |
| `opencode mcp auth <name>` | Authenticate with OAuth MCP server |

### Provider management

| Command | Description |
|---|---|
| `opencode providers list` | Show configured providers |
| `opencode providers login` | Add/configure a provider |
| `opencode providers logout` | Remove provider credentials |

### Model management

| Command | Description |
|---|---|
| `opencode models` | List all available models |
| `opencode models <provider>` | List models for a provider |
| `opencode models --verbose` | Show model details (context, cost) |
| `opencode models --refresh` | Refresh models cache |

### Agent management

| Command | Description |
|---|---|
| `opencode agent list` | List all agents (build, plan, explore, etc.) |
| `opencode agent create` | Create a custom agent |

### Session management

| Command | Description |
|---|---|
| `opencode session list` | List all sessions |
| `opencode session delete <id>` | Delete a session |

### Other

| Command | Description |
|---|---|
| `opencode serve` | Start headless server |
| `opencode web` | Start server + web UI |
| `opencode attach <url>` | Attach to running server |
| `opencode pr <number>` | Fetch PR and start coding |
| `opencode stats` | Token usage and costs |
| `opencode export [sessionID]` | Export session as JSON |
| `opencode upgrade` | Upgrade to latest version |

### Run mode flags

| Flag | Description |
|---|---|
| `-m, --model` | Model in `provider/model` format |
| `-c, --continue` | Continue last session |
| `-s, --session` | Continue specific session |
| `--fork` | Fork session when continuing |
| `-f, --file` | Attach files to message |
| `--thinking` | Show reasoning blocks |
| `--format json` | Output raw JSON events |
| `--dangerously-skip-permissions` | Auto-approve all actions |
| `--agent` | Use specific agent |

---

## Built-in Agents

OpenCode has a multi-agent architecture similar to Claude Code:

| Agent | Role | Type |
|---|---|---|
| `build` | Primary coding agent — file editing, shell, tools | Primary |
| `plan` | Architectural planning, reads but doesn't edit | Primary |
| `explore` | Codebase search and exploration | Subagent |
| `general` | General-purpose subagent | Subagent |
| `compaction` | Context compression | Primary |
| `summary` | Session summarisation | Primary |
| `title` | Session title generation | Primary |

### Custom agents

```bash
opencode agent create \
  --description "review Go code for bizzy conventions" \
  --tools "read,grep,glob,bash" \
  --mode subagent
```

---

## Comparison with Other Tools

### vs Claude Code

OpenCode is the closest CLI alternative to Claude Code. Same interactive TUI, same file editing + shell tools, same MCP client support, same subagent architecture. The difference: OpenCode works with free/local models, Claude Code requires Anthropic API.

### vs Pi (pi-mono)

Pi is a simpler CLI agent that works with Ollama but **does not support MCP**. It can't connect to bizzy's tool server. Good for basic coding tasks where you don't need bizzy integration.

### vs Aider

Aider is git-centric pair programming. It has MCP support but is more focused on code review and git workflows than agentic tool use. Good complement to OpenCode but not a direct replacement for Claude Code's agentic style.

### vs mcp-client-for-ollama

A TUI for calling MCP tools interactively. Not a coding agent — no file editing, no shell commands. Useful for testing/debugging your bizzy MCP tools but not for writing code.

---

## Integration with Bizzy's AI Runner

Bizzy's server already supports multiple AI providers through its Runner interface. OpenCode is a **client-side** tool — it doesn't go through bizzy's server for AI inference. Instead:

```
┌─────────────────────────────────────────┐
│  OpenCode (CLI)                          │
│                                          │
│  AI Provider ──> opencode/big-pickle     │
│                  ollama-cloud/kimi-k2.5   │
│                  local ollama model       │
│                                          │
│  MCP Client ──> http://localhost:8090/mcp │
│                 (bizzy server)            │
│                                          │
│  Tools ──> file edit, shell, git         │
│            + bizzy app tools via MCP     │
└─────────────────────────────────────────┘
```

This is the same architecture as Claude Code with bizzy — the AI runs client-side, tools come from the MCP server.

---

## Typical Workflow

```bash
# 1. Start bizzy server
cd ~/code/bizzy && make run

# 2. Get your token
nube login http://localhost:8090 <token>

# 3. Add bizzy MCP server to opencode
opencode mcp add
# -> type: streamable-http
# -> url: http://localhost:8090/mcp
# -> auth: Bearer <token>

# 4. Start coding with free model + bizzy tools
opencode -m opencode/big-pickle

# 5. Inside the session, you have:
#    - All file editing / shell / git tools
#    - All your bizzy app tools via MCP
#    - Free AI model, no API costs
```

---

## Data / Config Locations

| What | Location |
|---|---|
| Binary | `~/.opencode/bin/opencode` |
| Auth/credentials | `~/.local/share/opencode/auth.json` |
| Models cache | `~/.cache/opencode/models.json` |
| Plugins | `~/.config/opencode/` |
| Project config | `.opencode/` in project root |
| Sessions | `~/.local/share/opencode/` |
