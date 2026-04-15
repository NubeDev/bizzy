# QA Flow — How It Works

Conversational Q&A system driven by JS tools on the backend,
rendered dynamically by the Flutter app. Same pattern as Claude Code's
Plan mode — one question at a time, adaptive follow-ups, then AI generation.

## Architecture

```
┌─────────────┐         WS          ┌──────────────┐       JS        ┌──────────────┐
│  Flutter UI  │ ◄──────────────────► │  nube-server  │ ◄──────────── │  QA JS Tool  │
│  (qa_screen) │   questions/answers │  /api/agents/qa│   handle()    │  (per app)   │
└─────────────┘                      └──────────────┘                └──────────────┘
                                            │
                                            │ prompt
                                            ▼
                                     ┌──────────────┐
                                     │  Claude Code  │
                                     │  (streaming)  │
                                     └──────────────┘
```

## WebSocket Protocol

Endpoint: `ws://host/api/agents/qa?token=<bearer-token>`

### Full message sequence

```
  CLIENT                                SERVER
    │                                     │
    │◄──── {"type":"session",             │   1. Session created
    │       "session_id":"ses-abc123"}     │
    │                                     │
    │───── {"flow":"nube-marketing.       │   2. Client picks a QA flow
    │        marketing_plan_qa"}──────────►│
    │                                     │
    │◄──── {"type":"question",            │   3. Server asks first question
    │       "session_id":"ses-abc123",     │
    │       "field":"product",            │
    │       "label":"What product?",      │
    │       "input":"text",               │
    │       "required":true}              │
    │                                     │
    │───── {"answer":"Rubix"} ───────────►│   4. Client answers
    │                                     │
    │◄──── {"type":"question",            │   5. Next question (personalized)
    │       "field":"audience",           │
    │       "label":"Who is the target    │
    │        audience for Rubix?"}        │
    │                                     │
    │───── {"answer":"Integrators"} ─────►│   6. Client answers
    │                                     │
    │          ... more questions ...      │
    │                                     │
    │◄──── {"type":"question",            │   7. Conditional question
    │       "field":"agency",             │      (only asked if budget >= $50k)
    │       "label":"Agency or            │
    │        in-house?"}                  │
    │                                     │
    │───── {"answer":"hybrid"} ──────────►│   8. Client answers
    │                                     │
    │◄──── {"type":"generating",          │   9. All questions done
    │       "message":"Generating         │
    │        Marketing Plan..."}          │
    │                                     │
    │◄──── {"type":"connected",           │  10. Claude session started
    │       "model":"claude-opus-4-6"}    │
    │                                     │
    │◄──── {"type":"tool_call",           │  11. Claude calling tools
    │       "name":"marketing_plan"}      │
    │                                     │
    │◄──── {"type":"text",                │  12. Streaming response
    │       "content":"# Marketing..."}   │
    │                                     │
    │◄──── {"type":"done",                │  13. Complete
    │       "duration_ms":58000,          │
    │       "cost_usd":0.09}              │
    │                                     │
    │◄──── [WS close]                     │  14. Connection closed
```

### Event types

| Type | Direction | Purpose |
|------|-----------|---------|
| `session` | server → client | Session ID assigned |
| `question` | server → client | Ask user a question |
| `answer` | client → server | User's answer |
| `generating` | server → client | All questions done, starting AI |
| `connected` | server → client | Claude session started |
| `tool_call` | server → client | Claude calling an MCP tool |
| `text` | server → client | Streamed response text |
| `done` | server → client | Complete with duration/cost |
| `error` | server → client | Error occurred |

### Question event fields

```json
{
  "type": "question",
  "session_id": "ses-abc123",
  "field": "budget",
  "label": "What's the budget range?",
  "input": "select",
  "required": true,
  "options": [
    { "value": "< $10k", "label": "Under $10k" },
    { "value": "$10k - $50k", "label": "$10k – $50k" }
  ],
  "default": null,
  "placeholder": "",
  "min_length": 0,
  "max_length": 0
}
```

### Answer event format

```json
{"answer": "Rubix"}              // text
{"answer": "$10k - $50k"}        // select — send value, not label
{"answer": "linkedin,email"}     // multi_select — comma-separated values
{"answer": 5}                    // number
```

## Input types → Flutter widgets

| `input` value | Widget | Validation |
|---------------|--------|------------|
| `text` | `RubixInput` | `required`, `min_length`, `max_length` |
| `textarea` | `RubixInput(maxLines: 5)` | `required`, `min_length`, `max_length` |
| `select` | `RubixSelect` or radio list | `required`, `options` |
| `multi_select` | Chip toggle group | `options` |
| `number` | `RubixInput(keyboardType: number)` | `required`, `min`, `max` |

## Flutter Implementation

### New files to add in `apps/nube_agent/`

```
lib/features/qa/
├── models/
│   ├── qa_event.dart           # Typed event classes for all WS messages
│   └── qa_field.dart           # Question field model (input, options, validation)
├── data/
│   └── qa_ws_client.dart       # WebSocket client for /api/agents/qa
├── application/
│   └── qa_provider.dart        # Riverpod notifier: state machine + WS
├── screens/
│   └── qa_screen.dart          # Main screen: questions → generating → result
└── widgets/
    ├── qa_question_card.dart   # Renders one question with appropriate input
    ├── qa_text_input.dart      # text/textarea → RubixInput
    ├── qa_select_input.dart    # select → radio list or RubixSelect
    ├── qa_multi_select.dart    # multi_select → chip group
    ├── qa_answer_bubble.dart   # Shows answered Q&A pair (collapsed)
    ├── qa_generating.dart      # "Generating..." with animation
    └── qa_result_view.dart     # Streamed markdown result
```

### Integration with existing app

The app already has these relevant files:

```
lib/core/ws/agent_ws.dart              # Reuse WS connection pattern
lib/core/config/app_config.dart        # Server URL + token
lib/features/agents/models/agent.dart  # Agent model — tools already have "mode"
lib/features/agents/screens/agents_screen.dart  # Add QA flow launch button
lib/features/chat/widgets/message_bubble.dart   # Reuse for result rendering
lib/features/chat/widgets/tool_call_chip.dart   # Reuse for tool call display
lib/routing/app_router.dart            # Add /qa/:flow route
```

### Screen layout — conversational style

Questions and answers render as a chat-like thread. Only the current
question shows an active input. Previous Q&A pairs collapse.

```
┌──────────────────────────────────────────────┐
│  ← Marketing Plan Builder              ···   │
├──────────────────────────────────────────────┤
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ What product is this plan for?         │  │  ← answered (collapsed)
│  │ ✓ Rubix Edge Controller                │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ Target audience for Rubix?             │  │  ← answered (collapsed)
│  │ ✓ Systems integrators                  │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ What's the budget range?               │  │  ← active question
│  │                                        │  │
│  │  ○ Under $10k                          │  │
│  │  ● $10k – $50k                         │  │
│  │  ○ $50k – $100k                        │  │
│  │  ○ Over $100k                          │  │
│  │                                        │  │
│  │               [Continue]               │  │
│  └────────────────────────────────────────┘  │
│                                              │
└──────────────────────────────────────────────┘
```

After all questions:

```
├──────────────────────────────────────────────┤
│                                              │
│  ┌ answered questions (collapsed) ─────────┐ │
│  │ Product: Rubix Edge Controller           │ │
│  │ Audience: Systems integrators            │ │
│  │ Budget: $10k – $50k                      │ │
│  │ Timeline: 1 quarter                      │ │
│  │ Channels: LinkedIn, Email                │ │
│  └──────────────────────────────────────────┘ │
│                                              │
│  ┌──────────────────────────────────────────┐ │
│  │  ◐ Generating Marketing Plan...          │ │
│  └──────────────────────────────────────────┘ │
│                                              │
│  ⚙ calling marketing_plan                   │
│                                              │
│  # Marketing Plan: Rubix Edge Controller     │
│  **Target Audience:** Systems Integrators    │
│  **Budget:** $10k–$50k ...                   │
│                                              │
│  ┌──────────────────────────────────────────┐ │
│  │  ✓ Done · 58s · $0.09                    │ │
│  └──────────────────────────────────────────┘ │
│                                              │
├──────────────────────────────────────────────┤
│  [Copy] [Share] [New plan]                   │
└──────────────────────────────────────────────┘
```

### State machine

```dart
enum QaState {
  connecting,    // WS connecting
  waitingFlow,   // session received, sending flow name
  questioning,   // showing / answering questions
  generating,    // all questions done, waiting for Claude
  streaming,     // Claude response streaming in
  done,          // complete
  error,         // something failed
}
```

### Riverpod provider

```dart
class QaNotifier extends StateNotifier<QaSessionState> {
  final WebSocketChannel _channel;
  final List<QaExchange> exchanges = [];  // question + answer pairs
  String? currentField;
  String resultText = '';

  void start(String flow) {
    _channel.sink.add(jsonEncode({'flow': flow}));
  }

  void answer(dynamic value) {
    _channel.sink.add(jsonEncode({'answer': value}));
    // Collapse current question into exchanges list
  }

  void _handleEvent(Map<String, dynamic> event) {
    switch (event['type']) {
      case 'session':    // → waitingFlow, auto-send flow
      case 'question':   // → questioning, show input widget
      case 'generating': // → generating, show spinner
      case 'connected':  // → streaming
      case 'tool_call':  // → append tool chip
      case 'text':       // → append to resultText
      case 'done':       // → done
      case 'error':      // → error
    }
  }
}
```

### WebSocket client

```dart
class QaWsClient {
  final String serverUrl;
  final String token;

  Stream<Map<String, dynamic>> connect(String flow) async* {
    final uri = Uri.parse('$serverUrl/api/agents/qa?token=$token');
    final channel = WebSocketChannel.connect(uri);

    await for (final raw in channel.stream) {
      final event = jsonDecode(raw as String) as Map<String, dynamic>;
      yield event;

      // Auto-send flow after receiving session
      if (event['type'] == 'session') {
        channel.sink.add(jsonEncode({'flow': flow}));
      }

      if (event['type'] == 'done' || event['type'] == 'error') break;
    }
  }

  void answer(WebSocketChannel channel, dynamic value) {
    channel.sink.add(jsonEncode({'answer': value}));
  }
}
```

## JS Tool Authoring

QA tools are regular JS tools with `"mode": "qa"` in the JSON manifest.
One `handle()` function supports both calling modes:

### Chat mode (WebSocket Q&A)

Server passes `{_answers: {...}}`. Tool returns one question at a time.
When all questions are answered, returns `{type: "prompt", prompt: "..."}`.

```javascript
function handle(params) {
  // Chat mode: WebSocket Q&A
  if (params._answers !== undefined) {
    return chatMode(params._answers);
  }
  // Form mode: REST API (optional, for form-based UI)
  if (!params._submit) return formDefinition();
  return formSubmit(params);
}

function chatMode(answers) {
  if (!answers.product) {
    return {
      type: "question",
      field: "product",
      label: "What product is this for?",
      input: "text",
      required: true
    };
  }

  // Conditional — ask follow-up based on previous answer
  if (answers.product.indexOf("Edge") >= 0 && !answers.deployment) {
    return {
      type: "question",
      field: "deployment",
      label: "How will " + answers.product + " be deployed?",
      input: "select",
      options: [
        { value: "cloud", label: "Cloud-managed" },
        { value: "on-prem", label: "On-premise" }
      ]
    };
  }

  // All done
  return { type: "prompt", prompt: "..." };
}
```

### Adding a new QA flow

1. Create `apps/<app>/tools/my_flow_qa.json` with `"mode": "qa"`
2. Create `apps/<app>/tools/my_flow_qa.js` with `handle()` supporting `_answers`
3. The flow auto-appears in `GET /api/agents` with `mode: "qa"`
4. The Flutter app discovers it and the user can launch it

## Skill Generator (nube-admin)

The `nube-admin.create_skill_qa` tool is a special QA flow that creates
new skills. It asks the user what the skill should do, what questions
it should ask, and what the output should look like — then generates
all the files (app.yaml, prompt .md, QA JS tool) automatically.

### How it works in the Flutter app

```
User taps "Create Skill" → QA screen opens with nube-admin.create_skill_qa

Questions:
  1. "What should this skill be called?"        → sales-proposal
  2. "What does it do?"                          → Generates a sales proposal
  3. "What questions should it ask?"             → Company name, product, deal size...
  4. "What should the output look like?"         → 1-page proposal with pricing
  5. "Any tags?"                                 → sales, proposals
  6. "Ready to create?"                          → Yes

Tool returns structured output:
  {
    type: "prompt",
    create_app: { name: "sales-proposal", ... },
    create_prompt: { name: "sales_proposal", body: "...", ... },
    create_tool: { name: "sales_proposal_qa", script: "...", ... }
  }

Flutter app automatically calls:
  1. POST /apps                          → create the app
  2. POST /apps/sales-proposal/prompts   → create the prompt template
  3. POST /apps/sales-proposal/tools     → create the QA JS tool
  4. POST /apps/sales-proposal/install   → install for the user

Result: "sales-proposal" is now available as an agent with a QA flow.
```

### Flutter implementation

The skill generator screen extends `qa_screen.dart` with a post-QA step
that parses the structured output and makes the CRUD API calls:

```dart
void _handleSkillCreation(Map<String, dynamic> result) async {
  final client = ref.read(nubeClientProvider);

  // 1. Create app
  await client.post('/apps', data: result['create_app']);

  // 2. Create prompt
  final appName = result['create_app']['name'];
  await client.post('/apps/$appName/prompts', data: result['create_prompt']);

  // 3. Create tool
  await client.post('/apps/$appName/tools', data: result['create_tool']);

  // 4. Install
  await client.post('/apps/$appName/install');
}
```

## API Reference

### Agent & QA

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /api/agents` | REST | List agents — QA tools have `mode: "qa"` |
| `POST /api/agents/tools/:name` | REST | Call tool directly (form mode) |
| `GET /api/agents/qa?token=` | WS | Conversational Q&A flow |
| `GET /api/agents/run?token=` | WS | Direct prompt → Claude (no Q&A) |
| `GET /api/agents/sessions` | REST | Session history |
| `GET /api/agents/sessions/:id` | REST | Full session with result |

### App CRUD (admin)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /apps` | REST | List all apps |
| `POST /apps` | REST | Create new app |
| `PUT /apps/:id` | REST | Update app metadata |
| `DELETE /apps/:id` | REST | Delete app + files |
| `GET /apps/:id/tools` | REST | List tools |
| `POST /apps/:id/tools` | REST | Create tool (.js + .json) |
| `PUT /apps/:id/tools/:name` | REST | Update tool |
| `DELETE /apps/:id/tools/:name` | REST | Delete tool |
| `GET /apps/:id/prompts` | REST | List prompts |
| `POST /apps/:id/prompts` | REST | Create prompt |
| `DELETE /apps/:id/prompts/:name` | REST | Delete prompt |
| `POST /apps/:id/install` | REST | Install app for user |
