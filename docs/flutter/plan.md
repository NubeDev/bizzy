# Nube Agent App — Flutter Plan

## Overview

A Flutter app for interacting with nube-server agents via REST + WebSocket.
Runs on desktop (Linux/macOS/Windows), Android, iOS, and browser.

Reuses the `rubix_ui` package from the rubix-app for theming and components.

## Architecture

### Platform strategy (same pattern as rubix-app)

```
Browser:   Flutter web → Dart backend (Shelf) → proxies to nube-server
Native:    Flutter app → connects to nube-server directly
```

- **Browser**: Dart backend serves the Flutter web build and proxies
  `/api/*` and `/ws/*` to the Go nube-server. Same-origin avoids CORS.
- **Native**: User enters server URL + token on first launch.
  Stored in OS secure storage (keychain/keystore).

### Platform detection

```dart
// Same pattern as rubix-app
if (AppConfig.usesBackendApi) {
  // Web: relative URLs, proxy through Dart backend
} else {
  // Native: direct connection to nube-server URL
}
```

## Dependencies

```yaml
dependencies:
  flutter:
    sdk: flutter
  rubix_ui:
    path: ../../flutter/rubix-app/packages/rubix_ui  # reuse existing
  flutter_riverpod: ^2.5.1
  go_router: ^17.1.0
  dio: ^5.4.3
  web_socket_channel: ^3.0.0
  flutter_secure_storage: ^9.2.2
  shared_preferences: ^2.2.3
  flutter_markdown: ^0.7.0   # render agent responses
  google_fonts: ^8.0.2
```

## Project Structure

```
lib/
├── main.dart
├── app.dart                          # MaterialApp with rubix_ui theme
│
├── core/
│   ├── config/
│   │   └── app_config.dart           # kIsWeb, server URL, platform flags
│   ├── auth/
│   │   ├── auth_repository.dart      # interface
│   │   ├── auth_native.dart          # secure storage (token + server URL)
│   │   └── auth_web.dart             # backend API (same as rubix-app pattern)
│   ├── api/
│   │   └── nube_client.dart          # Dio REST client (agents, sessions, apps)
│   └── ws/
│       └── agent_ws.dart             # WebSocket client for streaming agent runs
│
├── features/
│   ├── auth/
│   │   └── screens/
│   │       └── login_screen.dart     # Server URL + token entry (native)
│   │
│   ├── agents/
│   │   ├── models/
│   │   │   ├── agent.dart            # Agent model (from GET /api/agents)
│   │   │   └── session.dart          # Session model (from GET /api/agents/sessions)
│   │   ├── data/
│   │   │   └── agent_repository.dart # REST calls for agents + sessions
│   │   ├── application/
│   │   │   └── agent_provider.dart   # Riverpod providers
│   │   └── screens/
│   │       └── agents_screen.dart    # Agent list with cards
│   │
│   ├── chat/
│   │   ├── models/
│   │   │   └── chat_message.dart     # Local message model (text, tool_call, etc.)
│   │   ├── application/
│   │   │   └── chat_provider.dart    # Manages WS connection + message stream
│   │   ├── screens/
│   │   │   └── chat_screen.dart      # Streaming chat with agent
│   │   └── widgets/
│   │       ├── message_bubble.dart   # Renders markdown text or tool call chip
│   │       ├── tool_call_chip.dart   # Shows "⚙ calling device_summary"
│   │       └── chat_input.dart       # Text input + send button
│   │
│   ├── qa/                           # QA flow screens
│   │   ├── models/
│   │   │   └── qa_form.dart          # QaForm, QaField, QaOption models
│   │   ├── data/
│   │   │   └── qa_repository.dart    # Calls POST /api/agents/tools/:name
│   │   ├── application/
│   │   │   └── qa_provider.dart      # Form state, validation, submission
│   │   ├── screens/
│   │   │   └── qa_screen.dart        # Dynamic form → submit → stream result
│   │   └── widgets/
│   │       ├── qa_field_builder.dart  # Routes field type → widget
│   │       ├── qa_text_field.dart     # text/textarea → RubixInput
│   │       ├── qa_select_field.dart   # select → RubixSelect
│   │       └── qa_multi_select.dart   # multi_select → chip group
│   │
│   ├── sessions/
│   │   └── screens/
│   │       ├── sessions_screen.dart  # Session history list
│   │       └── session_detail.dart   # Full session with rendered result
│   │
│   └── devices/
│       └── screens/
│           └── devices_screen.dart   # Device list (via rubix-developer agent)
│
└── routing/
    └── app_router.dart               # go_router setup

bin/
└── app_server.dart                   # Dart Shelf backend (web only)
```

## Screens

### 1. Login (native only)

Simple form: server URL + bearer token. Uses `RubixInput` and `RubixButton`.
Validates by calling `GET /health` then `GET /users/me`.
Stores credentials in flutter_secure_storage.

### 2. Agents list (home screen)

Grid/list of agent cards using `RubixCard`. Each card shows:
- Agent name and description
- Tool count badge
- Tools with `mode: "qa"` show a "Builder" badge
- "Chat" button → opens chat screen
- QA tools show a "Start" button → opens QA form screen

Data: `GET /api/agents`

### 3. Chat screen (main feature)

Split layout on desktop, full-screen on mobile:

```
┌──────────────────────────────────────────────┐
│  ← rubix-developer                     ···   │  ← RubixBreadcrumbBar
├──────────────────────────────────────────────┤
│                                              │
│  ⚙ calling ToolSearch                        │  ← tool_call_chip
│  ⚙ calling device_summary                   │  ← tool_call_chip
│                                              │
│  ┌──────────────────────────────────────┐    │
│  │ There are **3 devices** total —      │    │  ← message_bubble (markdown)
│  │ 2 online and 1 offline.              │    │
│  └──────────────────────────────────────┘    │
│                                              │
│  ┌──────────────────────────────────────┐    │
│  │ ses-bcc6c534 · 7.4s · $0.064        │    │  ← done summary
│  └──────────────────────────────────────┘    │
│                                              │
├──────────────────────────────────────────────┤
│  ┌────────────────────────────────┐  [Send]  │  ← chat_input
│  │ Ask the agent...               │          │
│  └────────────────────────────────┘          │
└──────────────────────────────────────────────┘
```

WebSocket flow:
1. Connect: `ws://server/api/agents/run?token=xxx`
2. Receive: `{"type":"session","session_id":"ses-..."}`
3. Send: `{"prompt":"...","agent":"rubix-developer"}`
4. Stream events → append to message list in real-time
5. Connection closes after "done" event

### 4. QA flow screen (guided builder)

Backend-driven dynamic form. The server defines the fields, validation
rules, and options — the UI renders them automatically.

```
┌──────────────────────────────────────────────┐
│  ← Marketing Plan Builder              ···   │
├──────────────────────────────────────────────┤
│                                              │
│  Answer a few questions to generate a        │
│  targeted marketing plan.                    │
│                                              │
│  What product or feature is this plan for? * │
│  ┌──────────────────────────────────────┐    │
│  │ Rubix Edge Controller                │    │  ← qa_text_field (RubixInput)
│  └──────────────────────────────────────┘    │
│                                              │
│  Target audience                             │
│  ┌──────────────────────────────────────┐    │
│  │ B2B IoT and building automation...   │    │  ← qa_text_field (default filled)
│  └──────────────────────────────────────┘    │
│                                              │
│  Budget range *                              │
│  ┌──────────────────────────────────────┐    │
│  │ $10k - $50k                        ▼ │    │  ← qa_select_field (RubixSelect)
│  └──────────────────────────────────────┘    │
│                                              │
│  Campaign timeline *                         │
│  ┌──────────────────────────────────────┐    │
│  │ 1 quarter                          ▼ │    │
│  └──────────────────────────────────────┘    │
│                                              │
│  Focus channels                              │
│  [LinkedIn] [Email] [Events] [ Content ]     │  ← qa_multi_select (chips)
│  [  Paid  ] [Partners] [Webinars]            │
│                                              │
│  Anything else we should know?               │
│  ┌──────────────────────────────────────┐    │
│  │                                      │    │  ← qa_text_field (textarea)
│  │                                      │    │
│  └──────────────────────────────────────┘    │
│                                              │
├──────────────────────────────────────────────┤
│                        [Generate Plan]       │  ← RubixButton.primary
└──────────────────────────────────────────────┘
```

#### QA flow protocol

```
Phase 1 — Get form definition:
  POST /api/agents/tools/nube-marketing.marketing_plan_qa
  Body: {}
  Response: { type: "qa", title: "...", fields: [...] }

Phase 2 — Submit with validation:
  POST /api/agents/tools/nube-marketing.marketing_plan_qa
  Body: { _submit: true, product: "Rubix", budget: "$10k - $50k", ... }
  Response (error):   { type: "validation_error", errors: [{field, message}] }
  Response (success): { type: "prompt", prompt: "Create a marketing plan..." }

Phase 3 — Feed rendered prompt to Claude:
  WS /api/agents/run?token=xxx
  Send: { prompt: "<rendered prompt from phase 2>", agent: "nube-marketing" }
  Stream: connected → tool_call → text → done
```

#### Field type → widget mapping

| Server field type | Flutter widget         | Validation props        |
|-------------------|------------------------|-------------------------|
| `text`            | `RubixInput`           | `required`, `min_length`, `max_length` |
| `textarea`        | `RubixInput(maxLines)` | `required`, `min_length`, `max_length` |
| `select`          | `RubixSelect`          | `required`, `options`   |
| `multi_select`    | Chip group             | `options`               |
| `number`          | `RubixInput(number)`   | `required`, `min`, `max`|

#### Client-side validation

The field definitions from the server drive Flutter validation directly —
no duplicate logic needed:

```dart
String? validateField(QaField field, String? value) {
  if (field.required && (value == null || value.isEmpty)) {
    return '${field.label} is required';
  }
  if (field.minLength != null && value != null && value.length < field.minLength!) {
    return 'Minimum ${field.minLength} characters';
  }
  if (field.maxLength != null && value != null && value.length > field.maxLength!) {
    return 'Maximum ${field.maxLength} characters';
  }
  return null;
}
```

Server validates again on submit (security boundary) — same JS tool,
same rules, single source of truth.

#### QA tool authoring (for app developers)

QA tools are regular JS tools with a convention:

```
apps/<app-name>/tools/
├── my_flow_qa.json     ← manifest with "mode": "qa"
└── my_flow_qa.js       ← handle(params) returns {type:"qa"|"validation_error"|"prompt"}
```

The JS tool has two phases in one function:

```javascript
function handle(params) {
  if (!params._submit) {
    // Phase 1: return form definition
    return {
      type: "qa",
      title: "My Flow",
      description: "...",
      fields: [
        { name: "x", label: "...", type: "text", required: true },
        // ...
      ]
    };
  }

  // Phase 2: validate and render prompt
  var errors = [];
  if (!params.x) errors.push({ field: "x", message: "Required" });
  if (errors.length > 0) return { type: "validation_error", errors: errors };

  return { type: "prompt", prompt: "..." };
}
```

No YAML schemas, no separate validation engine — the JS tool IS the
schema, validator, and prompt renderer.

### 5. Sessions history

List of past sessions using `RubixListPage`. Each row shows:
- Session ID, agent name, prompt preview, timestamp
- Tap → session detail with full markdown result

Data: `GET /api/agents/sessions`, `GET /api/agents/sessions/:id`

### 6. Devices (optional, later)

Could use the agent to query devices, or call REST directly.
Reuses `RubixCard`, dashboard gauge widgets for device status.

## Desktop vs Mobile Layout

Use `LayoutBuilder` or `MediaQuery` to adapt:

```dart
final isWide = MediaQuery.of(context).size.width > 800;

if (isWide) {
  // Desktop: sidebar + content area
  return Row(children: [
    SizedBox(width: 280, child: RubixSidebarTree(...)),
    Expanded(child: content),
  ]);
} else {
  // Mobile: bottom nav or drawer
  return Scaffold(
    body: content,
    bottomNavigationBar: BottomNavigationBar(...),
  );
}
```

### Desktop (> 800px)
```
┌─────────┬──────────────────────────────┐
│ Sidebar  │                              │
│          │                              │
│ Agents   │     Chat / Content area      │
│ Sessions │                              │
│ Devices  │                              │
│          │                              │
└─────────┴──────────────────────────────┘
```

### Mobile (< 800px)
```
┌──────────────────────┐
│     Content area     │
│                      │
│                      │
│                      │
├──────────────────────┤
│ Agents│Chats│Sessions│  ← bottom nav
└──────────────────────┘
```

## WebSocket Client

```dart
class AgentWS {
  final String serverUrl;
  final String token;

  WebSocketChannel? _channel;
  String? sessionId;

  Stream<AgentEvent> run(String prompt, {String? agent}) async* {
    final uri = Uri.parse('$serverUrl/api/agents/run?token=$token');
    _channel = WebSocketChannel.connect(uri);

    // First message is the session event.
    await for (final raw in _channel!.stream) {
      final event = AgentEvent.fromJson(jsonDecode(raw));

      if (event.type == 'session') {
        sessionId = event.sessionId;
      }

      yield event;

      // Send the prompt after receiving the session.
      if (event.type == 'session') {
        _channel!.sink.add(jsonEncode({
          'prompt': prompt,
          if (agent != null) 'agent': agent,
        }));
      }

      if (event.type == 'done' || event.type == 'error') break;
    }
  }

  void close() => _channel?.sink.close();
}
```

## Theming

Reuse `rubix_ui` directly:

```dart
MaterialApp(
  title: 'Nube Agent',
  theme: RubixTokens.lightTheme,
  darkTheme: RubixTokens.darkTheme,
  themeMode: ThemeMode.dark,  // default to dark (matches rubix-app)
)
```

All components use `RubixTokens.of(context)` for colors, typography, spacing.

## Dart Backend (browser only)

Same pattern as rubix-app `bin/app_server.dart`:

```dart
void main() async {
  final nubeServer = Platform.environment['NUBE_SERVER'] ?? 'http://localhost:8090';

  final handler = Cascade()
    .add(proxyHandler(nubeServer))         // /api/*, /mcp, websocket proxy
    .add(createStaticHandler('frontend'))   // Flutter web build
    .add(spaFallback('frontend'))           // SPA fallback → index.html
    .handler;

  final server = await serve(
    corsMiddleware(logRequests().addHandler(handler)),
    InternetAddress.anyIPv4,
    8080,
  );
  print('Serving on http://localhost:${server.port}');
}
```

## API Summary

### Core

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/health` | GET | none | Server status |
| `/bootstrap` | POST | none | Create first admin (one-time) |

### Auth & Users

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/users/me` | GET | Bearer | Current user info |
| `/users/:id` | GET | Admin | Get user by ID |
| `/users/:id` | DELETE | Admin | Delete user |
| `/users/:id/token` | POST | Bearer | Rotate token |
| `/workspaces` | GET | Bearer | List workspaces |
| `/workspaces` | POST | Admin | Create workspace |
| `/workspaces/:id/users` | POST | Admin | Create user in workspace |
| `/workspaces/:id/users` | GET | Bearer | List workspace users |

### App CRUD (admin)

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/apps` | GET | Bearer | List all apps |
| `/apps/:id` | GET | Bearer | Get app details |
| `/apps` | POST | Admin | Create new app |
| `/apps/:id` | PUT | Admin | Update app metadata |
| `/apps/:id` | DELETE | Admin | Delete app + all files |

### Tool CRUD (admin)

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/apps/:id/tools` | GET | Bearer | List tools in app |
| `/apps/:id/tools` | POST | Admin | Create tool (.js + .json) |
| `/apps/:id/tools/:name` | PUT | Admin | Update tool script/manifest |
| `/apps/:id/tools/:name` | DELETE | Admin | Delete tool |

### Prompt CRUD (admin)

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/apps/:id/prompts` | GET | Bearer | List prompts in app |
| `/apps/:id/prompts` | POST | Admin | Create prompt (.md) |
| `/apps/:id/prompts/:name` | DELETE | Admin | Delete prompt |

### App Installs

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/apps/:id/install` | POST | Bearer | Install app for user |
| `/app-installs` | GET | Bearer | List user's installs |
| `/app-installs/:id` | PATCH | Bearer | Enable/disable, update settings |
| `/app-installs/:id` | DELETE | Bearer | Uninstall app |

### Agent API

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/api/agents` | GET | Bearer | List agents + tools (mode=qa for QA tools) |
| `/api/agents/tools/:name` | POST | Bearer | Call tool directly (QA form/submit) |
| `/api/agents/run` | WS | `?token=` | Stream a Claude session |
| `/api/agents/qa` | WS | `?token=` | Conversational Q&A flow |
| `/api/agents/sessions` | GET | Bearer | List past sessions |
| `/api/agents/sessions/:id` | GET | Bearer | Get full session with result |

### Skill Generator (via nube-admin app)

The `nube-admin.create_skill_qa` tool is a QA flow that generates new
skills. It outputs structured `create_app`, `create_prompt`, and
`create_tool` data that maps directly to the App CRUD API above.

The Flutter app can automate the full pipeline:
1. Run the QA flow to gather requirements
2. Call `POST /apps` with `create_app` data
3. Call `POST /apps/:id/prompts` with `create_prompt` data
4. Call `POST /apps/:id/tools` with `create_tool` data
5. Call `POST /apps/:id/install` to enable it for the user

## Build Targets

| Platform | Command | Notes |
|----------|---------|-------|
| Linux desktop | `flutter build linux` | Direct nube-server connection |
| macOS desktop | `flutter build macos` | Direct nube-server connection |
| Android | `flutter build apk` | Direct nube-server connection |
| iOS | `flutter build ios` | Direct nube-server connection |
| Web | `flutter build web` + `dart run bin/app_server.dart` | Shelf proxy |

## Implementation Order

### Phase 1 — Core + Chat (MVP)
1. Scaffold project, add rubix_ui dependency
2. `AppConfig` + platform detection
3. Login screen (native) with secure storage
4. `NubeClient` (Dio REST client)
5. `AgentWS` (WebSocket streaming)
6. Chat screen with streaming markdown
7. Test on Linux desktop + Chrome

### Phase 2 — QA Flows
8. QA models (`QaForm`, `QaField`, `QaOption`)
9. `QaRepository` (calls `POST /api/agents/tools/:name`)
10. `qa_field_builder.dart` — dynamic field rendering
11. `qa_screen.dart` — form → validate → submit → stream result
12. Client-side validation from server field rules

### Phase 3 — Admin & Skill Builder
13. App management screen (list/create/delete apps)
14. Tool editor screen (view/create/edit JS tools)
15. Skill generator — run `nube-admin.create_skill_qa` QA flow,
    then auto-call CRUD API to create the app + prompt + tool + install
16. App marketplace / install flow

### Phase 4 — Polish
17. Agents list screen (cards with QA badges)
18. Sessions history screen
19. Desktop sidebar layout with `RubixSidebarTree`
20. Mobile bottom nav layout
21. Dart backend for web (Shelf proxy)

### Phase 5 — Extend
22. Device management screen
23. Session search/filter
24. Multiple concurrent chats
25. User management (admin)
26. Push notifications (mobile)
