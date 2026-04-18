
# PI AI (Pi Coding Agent)

Pi is an open-source interactive coding agent CLI (similar to Claude Code) that can run with local models via Ollama.

- GitHub: https://github.com/badlogic/pi-mono

## Install

```bash
pnpm install -g @mariozechner/pi-coding-agent
```

## Configuration

Pi uses `~/.pi/agent/models.json` to connect to Ollama. Create the file:

```json
{
  "providers": {
    "ollama": {
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "compat": {
        "supportsDeveloperRole": false,
        "supportsReasoningEffort": false
      },
      "models": [
        {
          "id": "gemma4:e4b",
          "name": "Gemma 4 E4B (Ollama)",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 8192,
          "cost": { "input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0 }
        },
        {
          "id": "gemma4:e4b-small",
          "name": "Gemma 4 E4B Small (4k ctx, low memory)",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 4096,
          "maxTokens": 4096,
          "cost": { "input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0 }
        }
      ]
    }
  }
}
```

## Usage

### Interactive mode (like Claude Code)

```bash
pi --provider ollama --model gemma4:e4b-small
```

### One-shot mode (run and exit)

```bash
pi --provider ollama --model gemma4:e4b-small -p "list all Go files"
```

### With file context

```bash
pi --provider ollama --model gemma4:e4b-small @main.go "explain this code"
```

### Continue last session

```bash
pi --provider ollama --model gemma4:e4b-small -c
```

### Shell alias (optional)

Add to `~/.bashrc` for quick access:

```bash
alias pia="pi --provider ollama --model gemma4:e4b-small"
```

Then just run `pia` to start.

## Useful Commands (inside pi)

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/model` | Switch models |
| `/compact` | Compress context when it gets long |
| `Ctrl+C` | Cancel current operation |
| `Ctrl+D` | Exit |

## Reducing Memory Usage

Create a smaller context model to save RAM:

```bash
cat > /tmp/Modelfile <<'EOF'
FROM gemma4:e4b
PARAMETER num_ctx 4096
PARAMETER num_batch 256
EOF
ollama create gemma4:e4b-small -f /tmp/Modelfile
```

This creates `gemma4:e4b-small` which uses significantly less memory than the full model.



# Ollama Guide

A practical reference for installing, managing, and using Ollama — including running models, using the REST API, and integrating with Claude Code.

---

## Table of Contents

- [Installation](#installation)
- [Start / Stop the Server](#start--stop-the-server)
- [Managing Models](#managing-models)
- [Running Models](#running-models)
- [REST API](#rest-api)
- [Claude Code Integration](#claude-code-integration)
- [Environment Variables](#environment-variables)

---

## Installation

### Linux / macOS

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### Windows

```powershell
irm https://ollama.com/install.ps1 | iex
```

### Docker

```bash
docker run -d -v ollama:/root/.ollama -p 11434:11434 --name ollama ollama/ollama
```

---

## Start / Stop the Server

Ollama runs as a background service. After installation it usually starts automatically. You can also control it manually.

### Start the server

```bash
ollama serve
```

By default the API is available at `http://localhost:11434`.

### Stop the server

#### systemd (Linux)

```bash
sudo systemctl stop ollama
sudo systemctl start ollama
sudo systemctl restart ollama
sudo systemctl status ollama
```

#### Manual process (macOS / Linux)

```bash
# Find the PID and kill it
pkill ollama
```

### View available environment variables

```bash
ollama serve --help
```

---

## Managing Models

### Pull (download) a model

```bash
ollama pull gemma3
ollama pull qwen3.5
ollama pull llama3.2
```

Browse the full library at [ollama.com/library](https://ollama.com/library).

### List downloaded models

```bash
ollama ls
```

### List currently running models

```bash
ollama ps
```

### Stop a running model (unload from memory)

```bash
ollama stop gemma3
```

### Remove a model

```bash
ollama rm gemma3
```

### Create a custom model from a Modelfile

```bash
# 1. Create a Modelfile
cat > Modelfile <<'EOF'
FROM gemma3
SYSTEM """You are a helpful assistant."""
EOF

# 2. Build the model
ollama create my-model -f Modelfile

# 3. Run it
ollama run my-model
```

---

## Running Models

### Interactive chat

```bash
ollama run gemma3
```

### Pass a single prompt

```bash
ollama run gemma3 "Explain how TCP/IP works in one paragraph"
```

### Multimodal (image input)

```bash
ollama run gemma3 "What's in this image? /path/to/image.png"
```

### Generate embeddings

```bash
echo "Hello world" | ollama run nomic-embed-text
```

---

## REST API

The REST API is available at `http://localhost:11434/api` once the server is running.

### Generate a completion

```bash
curl http://localhost:11434/api/generate -d '{
  "model": "gemma3",
  "prompt": "Why is the sky blue?",
  "stream": false
}'
```

### Chat (multi-turn)

```bash
curl http://localhost:11434/api/chat -d '{
  "model": "gemma3",
  "messages": [
    { "role": "user", "content": "Hello!" }
  ],
  "stream": false
}'
```

### Streaming response

Remove `"stream": false` (or set it to `true`) to receive tokens as they are generated:

```bash
curl http://localhost:11434/api/chat -d '{
  "model": "gemma3",
  "messages": [{ "role": "user", "content": "Tell me a joke" }]
}'
```

### List models via API

```bash
curl http://localhost:11434/api/tags
```

### Show model info

```bash
curl http://localhost:11434/api/show -d '{"name": "gemma3"}'
```

### Pull a model via API

```bash
curl http://localhost:11434/api/pull -d '{"name": "gemma3"}'
```

### Delete a model via API

```bash
curl -X DELETE http://localhost:11434/api/delete -d '{"name": "gemma3"}'
```

### Generate embeddings via API

```bash
curl http://localhost:11434/api/embeddings -d '{
  "model": "nomic-embed-text",
  "prompt": "Here is an article about llamas..."
}'
```

### Python SDK

```bash
pip install ollama
```

```python
from ollama import chat

response = chat(model='gemma3', messages=[
  {'role': 'user', 'content': 'Why is the sky blue?'}
])
print(response.message.content)
```

### JavaScript SDK

```bash
npm i ollama
```

```js
import ollama from 'ollama';

const response = await ollama.chat({
  model: 'gemma3',
  messages: [{ role: 'user', content: 'Why is the sky blue?' }],
});
console.log(response.message.content);
```

---

## Claude Code Integration

[Claude Code](https://docs.ollama.com/integrations/claude-code) is Anthropic's agentic coding tool. Ollama exposes an Anthropic-compatible API so you can power Claude Code with local or cloud models.

### Install Claude Code

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

### Quick launch (interactive)

```bash
ollama launch claude
```

This walks you through model selection and configures the environment automatically.

### Launch with a specific model

```bash
ollama launch claude --model qwen3.5
ollama launch claude --model kimi-k2.5:cloud
```

### Recommended models for Claude Code

| Model | Notes |
|---|---|
| `kimi-k2.5:cloud` | Cloud model, strong coding ability |
| `qwen3.5` | Local model, good context window |
| `qwen3.5:cloud` | Cloud variant |
| `glm-5:cloud` | Cloud model |
| `glm-4.7-flash` | Fast local model |

> **Note:** Claude Code requires a large context window — at least 64k tokens recommended.

### Manual / headless setup

Set environment variables then invoke `claude` directly:

```bash
export ANTHROPIC_AUTH_TOKEN=ollama
export ANTHROPIC_API_KEY=""
export ANTHROPIC_BASE_URL=http://localhost:11434

claude --model qwen3.5
```

Or inline:

```bash
ANTHROPIC_AUTH_TOKEN=ollama \
ANTHROPIC_BASE_URL=http://localhost:11434 \
ANTHROPIC_API_KEY="" \
claude --model qwen3.5
```

### Non-interactive / CI mode

```bash
ollama launch claude --model kimi-k2.5:cloud --yes -- -p "how does this repository work?"
```

`--yes` auto-pulls the model and skips interactive prompts. Arguments after `--` are passed directly to Claude Code.

### Scheduled tasks with `/loop`

Inside a Claude Code session, use `/loop` to run a prompt on a recurring schedule:

```
/loop 30m Check my open PRs and summarize their status
/loop 1h  Research the latest AI news and summarize key developments
/loop 15m Check for new GitHub issues and triage by priority
```

---

## Environment Variables

Set these before running `ollama serve` to customise behaviour:

| Variable | Default | Description |
|---|---|---|
| `OLLAMA_HOST` | `0.0.0.0:11434` | Address and port the server listens on |
| `OLLAMA_MODELS` | `~/.ollama/models` | Directory where models are stored |
| `OLLAMA_NUM_PARALLEL` | `1` | Number of parallel requests |
| `OLLAMA_MAX_LOADED_MODELS` | `1` | Max models loaded into memory |
| `OLLAMA_KEEP_ALIVE` | `5m` | How long to keep a model loaded after last use |
| `OLLAMA_DEBUG` | `false` | Enable verbose debug logging |
| `OLLAMA_GPU_OVERHEAD` | `0` | Reserve VRAM (bytes) for the OS / other apps |

### Example: expose Ollama on the network

```bash
OLLAMA_HOST=0.0.0.0:11434 ollama serve
```

### Example: keep models loaded longer

```bash
OLLAMA_KEEP_ALIVE=30m ollama serve
```

### Example: allow more parallel requests

```bash
OLLAMA_NUM_PARALLEL=4 ollama serve
```

---

## Further Reading

- [Ollama CLI Reference](https://docs.ollama.com/cli)
- [Ollama REST API Docs](https://docs.ollama.com/api)
- [Model Library](https://ollama.com/library)
- [Claude Code Docs](https://docs.ollama.com/integrations/claude-code)
- [Modelfile Reference](https://docs.ollama.com/modelfile)
- [Context Length Settings](https://docs.ollama.com/context-length)
