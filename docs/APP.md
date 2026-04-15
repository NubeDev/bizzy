# Flutter App: nube_agent

Desktop/mobile/web client for interacting with nube-server agents and the community app store.

---

## Quick start

```bash
cd app/nube_agent

# Linux desktop
flutter run -d linux

# Web (served behind Dart Shelf proxy)
flutter run -d chrome
```

On first launch, enter the nube-server URL (default `http://localhost:8090`) and your bearer token from `/bootstrap`.

---

## Architecture

```
┌─────────────────────────────────────────────┐
│               Flutter App                    │
│                                              │
│  NubeClient ──── nube-server (:8090)         │
│    agents, sessions, installs, MCP           │
│                                              │
│  StoreClient ── store-server (:8091)         │
│    browse, create, review, publish           │
│                                              │
│  AgentWS ────── ws://nube-server             │
│    streaming chat + QA flows                 │
└─────────────────────────────────────────────┘
```

Two separate API clients, two separate servers.

---

## Dependencies

| Package | Version | Purpose |
|---|---|---|
| `flutter_riverpod` | ^2.5.1 | State management |
| `go_router` | ^17.1.0 | Navigation |
| `dio` | ^5.4.3 | HTTP client |
| `web_socket_channel` | ^3.0.0 | WebSocket |
| `flutter_secure_storage` | ^9.2.2 | Native credential storage |
| `shared_preferences` | ^2.2.3 | Web credential storage |
| `flutter_markdown` | ^0.7.0 | Markdown rendering |
| `lucide_icons` | ^0.257.0 | Icons |
| `google_fonts` | ^8.0.2 | Typography |
| `rubix_ui` | local | Custom UI component library |

---

## Configuration

| Setting | Value | Notes |
|---|---|---|
| Default nube-server URL | `http://localhost:8090` | Entered on login screen |
| Default store-server URL | `http://localhost:8091` | Set via `storeServerUrlProvider` |
| Web mode | Same-origin proxy | Uses empty base URL |

Credentials stored in:
- **Native** (Linux/macOS/iOS/Android): `flutter_secure_storage` keys `nube_server_url`, `nube_token`
- **Web**: `shared_preferences` key `nube_token`

---

## Navigation

Four tabs via `StatefulShellRoute.indexedStack`:

| Index | Tab | Icon | Root route |
|---|---|---|---|
| 0 | Agents | `messageSquare` | `/agents` |
| 1 | History | `clock` | `/sessions` |
| 2 | Store | `store` | `/store` |
| 3 | Admin | `settings` | `/admin` |

Wide layout (>800px): sidebar (260px) + content. Narrow: bottom bar + content.

---

## Routes

| Path | Screen | Description |
|---|---|---|
| `/login` | LoginScreen | Server URL + token entry |
| `/agents` | AgentsScreen | List agents with tool/flow counts |
| `/agent/:name` | AgentDetailScreen | Agent detail, launch chat or QA |
| `/chat/:agent` | ChatScreen | Streaming conversation |
| `/qa/:flow` | QaScreen | Interactive Q&A form flow |
| `/sessions` | SessionsScreen | Session history list |
| `/sessions/:id` | SessionDetailScreen | Session result + metadata |
| `/store` | StoreScreen | Browse public apps (search, filter, sort) |
| `/store/:id` | StoreAppDetailScreen | App detail, tools, prompts, reviews |
| `/my-apps` | MyAppsScreen | Author's apps, create new |
| `/my-apps/:id` | StoreAppEditorScreen | Edit metadata, tools, prompts, publish |
| `/admin` | AdminScreen | System app management |

Auth redirect: unauthenticated users go to `/login`, authenticated users skip it.

---

## State management

Riverpod providers. Pattern: `FutureProvider` for data fetching, `StateNotifierProvider.family` for stateful flows (chat, QA).

### Auth providers

| Provider | Type | Description |
|---|---|---|
| `authRepositoryProvider` | `Provider<AuthRepository>` | Platform-specific storage |
| `authProvider` | `AsyncNotifierProvider<AuthNotifier, AuthCredentials?>` | Current auth state |
| `nubeClientProvider` | `Provider<NubeClient?>` | HTTP client (null if not logged in) |

### Store providers

| Provider | Type | Description |
|---|---|---|
| `storeServerUrlProvider` | `StateProvider<String>` | Store server URL |
| `storeTokenProvider` | `StateProvider<String>` | Store auth token |
| `storeClientProvider` | `Provider<StoreClient>` | HTTP client for store-server |
| `storeAppsProvider` | `FutureProvider.family<StoreListResponse, StoreQuery>` | Browse catalog |
| `storeAppDetailProvider` | `FutureProvider.family<StoreApp, String>` | App detail by ID |
| `storeAppReviewsProvider` | `FutureProvider.family<List<AppReview>, String>` | Reviews by app ID |
| `myStoreAppsProvider` | `FutureProvider<List<StoreApp>>` | Author's apps |
| `categoriesProvider` | `FutureProvider<List<String>>` | Category list |

### Agent providers

| Provider | Type | Description |
|---|---|---|
| `agentsProvider` | `FutureProvider<List<Agent>>` | Agent list |
| `sessionsProvider` | `FutureProvider<List<Session>>` | Session history |
| `sessionDetailProvider` | `FutureProvider.family<Session, String>` | Session by ID |

### Chat provider

| Provider | Type | Description |
|---|---|---|
| `chatProvider` | `StateNotifierProvider.family<ChatNotifier, ChatState, String>` | Per-agent chat state |

`ChatState`: `{messages, isRunning, error}`

### QA provider

| Provider | Type | Description |
|---|---|---|
| `qaProvider` | `StateNotifierProvider.family<QaNotifier, QaSessionState, String>` | Per-flow QA state |

`QaSessionState`: `{phase, exchanges, currentQuestion, resultText, toolCalls, ...}`

`QaPhase`: `connecting` -> `questioning` -> `generating` -> `streaming` -> `done`

---

## API clients

### NubeClient (nube-server)

Talks to `nube-server` for workspace operations, agents, and installs.

| Group | Methods |
|---|---|
| Health | `checkHealth()` |
| Users | `getMe()` |
| Agents | `listAgents()` |
| Sessions | `listSessions()`, `getSession(id)` |
| Apps | `listApps()`, `getApp(name)`, `createApp(data)`, `updateApp(name, data)`, `deleteApp(name)` |
| Tools | `listAppTools(app)`, `createTool(app, data)`, `updateTool(app, name, data)`, `deleteTool(app, name)` |
| Prompts | `listAppPrompts(app)`, `createPrompt(app, data)`, `deletePrompt(app, name)` |
| Installs | `installApp(name)` |
| Tool exec | `callTool(name, params)` |

### StoreClient (store-server)

Talks to `store-server` for the community marketplace.

| Group | Methods |
|---|---|
| Health | `checkHealth()` |
| Register | `register(name, email)` |
| Browse | `listStoreApps(...)`, `getStoreApp(id)`, `listCategories()` |
| Reviews | `listStoreAppReviews(appId)`, `submitReview(appId, rating, comment)`, `deleteReview(appId)` |
| My Apps | `listMyStoreApps()`, `createStoreApp(data)`, `getMyStoreApp(id)`, `updateStoreApp(id, data)`, `deleteStoreApp(id)`, `publishStoreApp(id)`, `setStoreAppVisibility(id, vis)` |
| Tools | `addStoreTool(appId, data)`, `updateStoreTool(appId, name, data)`, `deleteStoreTool(appId, name)` |
| Prompts | `addStorePrompt(appId, data)`, `updateStorePrompt(appId, name, data)`, `deleteStorePrompt(appId, name)` |

---

## WebSocket protocol

### Agent chat (`/api/agents/run`)

```
connect: ws://server/api/agents/run?token=<token>
  <- {"type":"session","session_id":"ses-..."}
  -> {"prompt":"...", "agent":"..."}
  <- {"type":"connected","model":"...","session_id":"..."}
  <- {"type":"text","content":"..."}        (streamed, multiple)
  <- {"type":"tool_call","name":"..."}      (zero or more)
  <- {"type":"done","duration_ms":...,"cost_usd":...}
```

### QA flow (`/api/agents/qa`)

```
connect: ws://server/api/agents/qa?token=<token>
  <- {"type":"session","session_id":"ses-..."}
  <- {"type":"question","field":{...}}      (server asks)
  -> {"field":"...","value":"..."}          (client answers)
  ... repeat until all questions answered ...
  <- {"type":"generating","message":"..."}
  <- {"type":"text","content":"..."}        (streamed result)
  <- {"type":"done",...}
```

QA can also return skill generation data: `createApp`, `createPrompt`, `createTool` fields in the `generating` event.

---

## Models

### Agent

| Field | Type | Notes |
|---|---|---|
| `name` | String | |
| `description` | String | |
| `tools` | List\<AgentTool\> | |

Computed: `qaTools` (tools where mode == "qa"), `hasQaFlows`

### AgentTool

| Field | Type | Notes |
|---|---|---|
| `name` | String | Namespaced: `appName.toolName` |
| `type` | String | `"openapi"` or `"js"` |
| `description` | String | |
| `mode` | String | `""` or `"qa"` |

Computed: `isQa`, `shortName`

### Session

| Field | Type | Notes |
|---|---|---|
| `id` | String | |
| `agent` | String | |
| `prompt` | String | |
| `result` | String | |
| `status` | String | |
| `durationMs` | int | |
| `costUsd` | double | |
| `userId` | String | |
| `createdAt` | DateTime | |

### ChatMessage

| Field | Type | Notes |
|---|---|---|
| `type` | ChatMessageType | `userMessage`, `text`, `toolCall`, `connected`, `done`, `error` |
| `content` | String | |
| `toolName` | String? | For `toolCall` |
| `model` | String? | For `connected` |
| `sessionId` | String? | |
| `durationMs` | int? | For `done` |
| `costUsd` | double? | For `done` |

### StoreApp

| Field | Type | Notes |
|---|---|---|
| `id` | String | |
| `name` | String | Slug |
| `displayName` | String | |
| `description` | String | Short |
| `longDescription` | String | Markdown |
| `version` | String | |
| `icon` | String | Lucide icon name |
| `color` | String | Hex |
| `category` | String | |
| `tags` | List\<String\> | |
| `authorId` | String | |
| `authorName` | String | |
| `visibility` | String | `private`, `shared`, `unlisted`, `public` |
| `installCount` | int | |
| `avgRating` | double | |
| `reviewCount` | int | |
| `toolCount` | int | |
| `promptCount` | int | |
| `tools` | List\<StoreTool\> | Full detail only |
| `prompts` | List\<StorePrompt\> | Full detail only |
| `createdAt` | DateTime | |
| `updatedAt` | DateTime | |
| `publishedAt` | DateTime? | |

Two factories: `fromSummary()` (listing cards), `fromJson()` (full detail with tools/prompts).

### QaField

| Field | Type | Notes |
|---|---|---|
| `field` | String | Field name |
| `label` | String | Display label |
| `input` | String | `text`, `textarea`, `select`, `multi_select`, `number` |
| `required` | bool | |
| `defaultValue` | dynamic | |
| `placeholder` | String | |
| `options` | List\<QaOption\> | For select types |
| `minLength` | int | |
| `maxLength` | int | |

---

## Project structure

```
lib/
  main.dart
  core/
    api/
      nube_client.dart          # nube-server HTTP client
      store_client.dart         # store-server HTTP client
    auth/
      auth_repository.dart      # Credential model + interface
      auth_native.dart          # FlutterSecureStorage impl
      auth_web.dart             # SharedPreferences impl
    config/
      app_config.dart           # Platform config
    ws/
      agent_ws.dart             # Agent WebSocket client
  routing/
    app_router.dart             # GoRouter with 4 branches
    app_shell.dart              # Responsive sidebar/bottom-bar
  features/
    auth/screens/
      login_screen.dart
    agents/
      screens/                  # AgentsScreen, AgentDetailScreen
      models/                   # Agent, AgentTool, Session
      application/              # authProvider, agentsProvider
    chat/
      screens/                  # ChatScreen
      models/                   # ChatMessage
      application/              # chatProvider
    qa/
      screens/                  # QaScreen
      models/                   # QaEvent, QaField
      application/              # qaProvider
      data/                     # QaWsClient
    sessions/screens/           # SessionsScreen, SessionDetailScreen
    store/
      screens/                  # StoreScreen, StoreAppDetailScreen,
                                # MyAppsScreen, StoreAppEditorScreen
      models/                   # StoreApp, StoreTool, StorePrompt, AppReview
      application/              # storeClientProvider, storeAppsProvider, etc.
    admin/screens/              # AdminScreen
```
