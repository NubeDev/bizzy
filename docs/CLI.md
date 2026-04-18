# Nube CLI

The `nube` CLI is the command-line client for the NubeIO central server. Primary commands are hand-written for the best UX. Additional admin/store commands are auto-generated from the embedded OpenAPI spec (`cmd/nube/openapi.yaml`).

---

## Installation / Build

```bash
go build -o bin/nube ./cmd/nube/
# or
make build
```

The binary embeds the OpenAPI spec — no external files required.

---

## Configuration

Credentials are stored in `~/.nube/config.json` after `login`. Every command reads this file automatically.

Global flags (override per-command):

| Flag | Description |
|---|---|
| `--server <url>` | Override the server URL |
| `--token <token>` | Override the bearer token |
| `-o json` | Output raw JSON instead of a table |

---

## Quick reference

```
nube tools                              # list your callable tools
nube prompts                            # list your prompts (with options)
nube prompts <name>                     # detail view for a prompt
nube providers                          # list AI providers
nube providers <name>                   # test a provider
nube ask "question"                     # AI chat (streaming, default provider)
nube ask --provider ollama "question"   # AI chat with specific provider
nube ask --session ses-abc123 "follow up" # resume a previous conversation
nube run <tool> [--param k=v]           # call a tool directly         (TODO)
nube jobs submit "prompt"               # async job                    (TODO)
nube jobs poll <id>                     # stream job events            (TODO)
nube preferences                        # show default provider/model  (TODO)
nube preferences set --provider ollama   # set default                 (TODO)
```

---

## Authentication

### Bootstrap (first-time setup)

```bash
nube bootstrap \
  --workspaceName "Acme Corp" \
  --adminName "Alice" \
  --adminEmail "alice@acme.com"
```

### Login / Logout

```bash
nube login http://localhost:8090 <token>
nube logout
```

---

## Discovery — what do I have?

### Tools

Lists all callable tools from your installed apps. Prompt-mode tools are excluded (see `nube prompts`).

```bash
nube tools
nube tools --app rubix
nube tools -o json
```

Example output:

```
NAME                              APP              TYPE     DESCRIPTION
rubix.runtime_status              rubix            js       Get Rubix runtime status
rubix.query_nodes                 rubix            js       Query nodes using Haystack filter
rubix-developer.device_summary    rubix-developer  openapi  Get device summary
weather-checker.get_weather       weather-checker  js       Get weather for a city

20 tools. Run with: nube run <name> [--param key=val]
```

### Prompts

Lists all prompts from your installed apps, including prompt-mode tools. Shows available options.

```bash
nube prompts
nube prompts rubix.navigation
```

Example output:

```
NAME                            APP              DESCRIPTION                                         OPTIONS
nube-marketing.content_review   nube-marketing   Review marketing content for quality, tone, and...
rubix.navigation                rubix            View/create/edit navigation hierarchy               show, build, edit, delete
rubix.getting_started           rubix            Get started with Rubix

11 prompts. Use with: nube ask "/name [option]"
```

Detail view for a specific prompt:

```
$ nube prompts rubix.navigation
Name:        rubix.navigation
App:         rubix
Type:        prompt
Description: View, create, or edit the site/building/floor navigation hierarchy

Arguments:
  action       What to do: show, build, edit, or delete
               options: show | build | edit | delete

Usage:
  nube ask "/rubix.navigation show"
  nube ask "/rubix.navigation build"
  nube ask "/rubix.navigation edit"
  nube ask "/rubix.navigation delete"
```

### Providers

```bash
nube providers              # list all with availability
nube providers ollama       # test connectivity + show models
```

Example output:

```
PROVIDER  AVAILABLE  TYPE  MODELS
claude    yes        cli   -
ollama    yes        api   gemma3, llama3.1
openai    no         api   -

$ nube providers ollama
Provider:  ollama
Available: yes
Latency:   12ms
Models:    gemma3, llama3.1
```

---

## AI Chat — ask

The primary way to interact with AI. Connects via WebSocket and streams the response live. The AI has access to all your installed tools via MCP.

```bash
# Default provider (from your preferences, or claude)
nube ask "write a marketing plan for Rubix"

# Specific provider/model
nube ask --provider ollama --model gemma3 "check my devices"
nube ask --provider openai --model gpt-4.1 "summarize"

# Use a prompt (with options)
nube ask "/rubix.navigation show"
nube ask "/nube-marketing.content_review"

# Direct mode (bypass server, no MCP tools)
nube ask --direct "quick question"
```

---

## Async Jobs

Submit a job for background processing (useful for scripts, cron, CI):

```bash
# Submit
nube agents submit "generate weekly report"
nube agents submit --provider ollama "check devices"

# Poll (streams events until done)
nube agents poll <job-id>

# One-shot check
nube agents poll <job-id> --once

# Scripting
JOB=$(nube agents submit -o json "generate report" | jq -r .job_id)
nube agents poll $JOB --once -o json | jq .result
```

---

## Sessions (history)

```bash
nube sessions list
nube sessions get --id <session-id>
```

---

## Server Status

```bash
nube status
nube status -o json
```

---

## App Store

```bash
# Browse
nube store browse
nube store browse --q "weather" --category automation

# Install
nube store get     --id <store-app-id>
nube store install --id <store-app-id>
nube store install --id <store-app-id> --settings '{"api_key":"xyz"}'

# Reviews
nube store reviews       --id <store-app-id>
nube store review        --id <store-app-id> --rating 5 --comment "Great"
nube store delete-review --id <store-app-id>
```

---

## Installs (managing installed apps)

```bash
nube installs list
nube installs update --id <install-id> --enabled true
nube installs update --id <install-id> --settings '{"key":"value"}'
nube installs delete --id <install-id>
```

---

## Admin

```bash
nube admin reload       # re-scan apps directory, rebuild MCP cache
nube admin providers    # view/manage global provider config
```

---

## My Apps (authoring)

Create and manage apps you publish to the store:

```bash
nube myapps list
nube myapps create --name my-app --displayName "My App" --description "Does stuff"
nube myapps publish    --id <app-id>
nube myapps visibility --id <app-id> --visibility public

# Tools & prompts
nube myapps add-tool    --id <app-id> --name my_tool --script '...'
nube myapps add-prompt  --id <app-id> --name my_prompt --body "Do {{thing}}"
```

---

## Output formats

All commands support `-o json` for scripting:

```bash
nube tools -o json | jq '.[].name'
nube providers -o json | jq '.[] | select(.available)'
nube prompts rubix.navigation -o json | jq .params
JOB=$(nube agents submit -o json "my prompt" | jq -r .job_id)
```

---

## Typical workflow

```bash
# 1. Setup
nube bootstrap --workspaceName "Acme" --adminName "Alice" --adminEmail "alice@acme.com"
nube login http://localhost:8090 <token>

# 2. Explore
nube status
nube store browse
nube store install --id <rubix-app-id>

# 3. Discover
nube tools                          # what can I run?
nube prompts                        # what prompts are available?
nube prompts rubix.navigation       # what options does this have?

# 4. Use
nube ask "summarise the devices on my network"
nube ask "/rubix.navigation show"
nube run rubix.runtime_status       # (coming soon)

# 5. Review
nube sessions list
```

---

## TODO — remaining CLI work

### Stage 2: `nube run <tool>` — direct tool execution

```bash
nube run rubix.runtime_status
nube run rubix.query_nodes --filter 'type is "ui.container"'
nube run rubix-developer.restart_device --id dev-001
nube run rubix.navigation    # -> error: "this is a prompt, use nube ask"
```

- Top-level `run` command that calls `POST /api/agents/tools/:name`
- Params passed as `--key value` flags (dynamic, from tool manifest)
- Rejects prompt-mode tools with a helpful redirect to `nube ask`
- Backend: guard `callTool` handler to return 400 for prompt-mode tools

### Stage 3: `nube preferences` — user default provider/model

```bash
nube preferences                                    # show current
nube preferences set --provider ollama --model gemma3  # set default
```

- Hand-written command wrapping `GET/PUT /users/me/preferences`
- Shows what will be used when no `--provider` is specified

### Stage 4: `nube jobs` — rename from `agents submit/poll`

```bash
nube jobs submit "generate report" --provider ollama
nube jobs poll <id>
nube jobs poll <id> --once
```

- Rename `agents submit` -> `jobs submit`, `agents poll` -> `jobs poll`
- Remove stale `agents` subcommands that are replaced by top-level commands
- Clean up auto-generated commands that overlap with hand-written ones

### Stage 5: cleanup

- Remove `agents list`, `agents providers`, `agents run`, `agents call` auto-generated commands (replaced by `tools`, `providers`, `ask`, `run`)
- Update `--help` text on root command to show the primary workflow
- Add `nube --version` flag
