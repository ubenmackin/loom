# Loom — Architecture

> Source of truth for Loom's design, protocols, and component responsibilities.
> Update via append entries under **Update History** when significant architectural
> changes are made.

## 1. Project Soul

Loom is an **agent-first work execution system**. Instead of tracking work for humans
to do manually, Loom orchestrates AI agents (planners, executors, builders, reviewers)
to plan stories, write code, verify builds, and review code — with humans acting primarily
as supervisors who create stories, answer planning questions, and intervene on failures.

The system is built around two distinct, complementary protocols:

- **MCP (Model Context Protocol)** — the *toolbox*. Agents connect to Loom as a tool
  server and decide which tools to call (`create_task`, `request_work`,
  `complete_work`, `report_blocked`, `add_comment`, …). This is the
  agent-driven, **pull** channel.
- **ACP (Agent Communication Protocol v1)** — the *conduit*. Loom spawns `opencode acp`
  as a subprocess and pushes prompts to agent sessions on demand (plan this story,
  execute this task, run build gate, etc.). This is the Loom-driven, **push** channel.

The deliberate design decision is that **ACP does only one thing**: push a prompt to an
agent session. ACP does not transport task assignments, completions, or state — those
flow through MCP tools invoked by the agent itself, and through the Dispatcher's internal
event bus. This separation prevents the double-completion bugs and protocol confusion
that plagued earlier iterations.

## 2. Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP router | `go-chi/chi/v5` |
| WebSocket | `gorilla/websocket` (UI hub only) |
| Storage | `modernc.org/sqlite` (pure-Go, **no CGO**) |
| Auth | `golang.org/x/crypto` (bcrypt) |
| IDs | `google/uuid` |
| Frontend | React + TypeScript, Vite build |
| Agent subprocess | `opencode acp` (stdio JSON-RPC 2.0) |
| Package | `github.com/ubenmackin/loom` |

## 3. Repository Layout

```
cmd/server/main.go          # Single binary, dual-mode (HTTP or --mcp)
internal/
  acp/                      # ACP v1 client: subprocess + JSON-RPC 2.0
  api/                      # REST API handlers + middleware (chi router)
  config/                   # Config helpers
  db/                       # Open/Migrate/Seed; migrations/*.sql; default-templates/
  dispatcher/               # Event bus, gate injection, staleness, dependency resolution
  gateway/                  # ACP session orchestration, queue, prompts, context builder
  mcp/                      # MCP tool server (stdio JSON-RPC, exposed to agents)
  models/                   # Domain types (Story, Task, Session, AgentProfile, enums)
  store/                    # 11 store types + generics helpers + cycle detection
  ws/                       # WebSocket hub (UI broadcast only)
opencode_config/
  opencode.json             # opencode client config (MCP server, agent definitions, permissions)
  prompts/*.txt             # Agent system prompt templates (one per role)
web/                        # React frontend (Vite)
.opencode/skills/           # Custom opencode skills (build-project, lint-project, etc.)
Makefile                    # `make run` = `go run ./cmd/server` (HTTP mode)
```

## 4. High-Level Architecture

```
                 ┌─────────────────────────────────────────────────┐
                 │                  opencode (external)             │
                 │                                                  │
                 │   ┌────────────────┐    ┌──────────────────┐    │
   ACP push      │   │  opencode acp  │    │   agent + Loom    │    │
   (stdin/stdout │   │  (subprocess)  │◄──►│   MCP tool calls  │    │
   JSON-RPC 2.0) │   └────────────────┘    └──────────────────┘    │
                 │           ▲                       ▲              │
                 └───────────┼───────────────────────┼─────────────┘
                             │ ACP                   │ MCP (stdio)
                             │                       │
┌────────────────────────────┼───────────────────────┼────────────────────┐
│                            │                       │                    │
│  Loom process (Go binary)                                                │
│                                                                          │
│  ┌─────────────┐    ┌────────────┐    ┌─────────────┐    ┌────────────┐ │
│  │  Gateway    │    │ Dispatcher │    │  MCP Server │    │  REST API  │ │
│  │  (spawn ACP,│───►│  (events,  │◄──►│  (tool      │◄──►│  (chi,     │ │
│  │   push      │    │   gates,   │    │   handlers) │    │   web UI)  │ │
│  │   prompts)  │    │   staleness)    └─────────────┘    └────────────┘ │
│  └─────────────┘    └────────────┘                                      │
│         │                  │                                             │
│         ▼                  ▼                                             │
│  ┌────────────────────────────────────────┐    ┌──────────────────────┐  │
│  │  Stores (11): Story, Task, Session,    │    │  WebSocket Hub       │  │
│  │  Project, Comment, Template, Activity, │    │  (UI broadcast only) │  │
│  │  User, Profile, Setting, ProfileTaskType │   └──────────────────────┘  │
│  └────────────────────────────────────────┘                              │
│         │                                                                │
│         ▼                                                                │
│  ┌────────────────────────────────────────┐                              │
│  │  SQLite (modernc.org/sqlite, no CGO)    │                              │
│  │  internal/db/migrations/*.sql           │                              │
│  └────────────────────────────────────────┘                              │
└──────────────────────────────────────────────────────────────────────────┘
```

## 5. Component Reference

### 5.1 `cmd/server/main.go` — Entry Point

A single binary with two modes, selected by `--mcp` flag:

- **MCP mode** (`loom-server --mcp`): Runs the MCP tool server on stdio. Started by
  the agent harness (via the ACP `mcpServers` config pushed by Gateway). Uses a
  `noopBroadcaster` (no UI to push events to). Gateway reference is `nil`.
- **HTTP mode** (default, `loom-server`): Starts the full stack — opens DB, runs
  migrations, seeds defaults, creates WebSocket hub, Dispatcher (with hub as
  broadcaster), Gateway (resolves `acp_command` setting, default `opencode acp`),
  REST router mounted at `/api`, and serves `web/dist` as SPA with symlink
  protection.

The `Stores` struct holds 10 `*store.*Store` pointers and is shared across MCP and
HTTP paths via dependency injection.

### 5.2 `internal/models/models.go` — Domain Types

| Enum | Values |
|---|---|
| `Status` | new, draft, planning, ready, in_progress, blocked, done, completed, canceled, archived |
| `TaskType` | code, build, review, planning |
| `SessionStatus` | active, stale, disconnected |
| `AssigneeType` | human, session |
| `WorkItemType` | story, task |
| `UserRole` | admin, normal |
| `AuthorType` | human, session |

Core entities: **User, Session, Project, Story, Task, TaskDependency, Comment,
ActivityLogEntry, PromptTemplate, UnreadComment, AgentProfile**.

Notable design: `Story` and `Task` both carry `agent_session_id` + `agent_type`
fields so the Gateway can re-target an existing ACP session for Q&A context updates
(`session/prompt` re-send) rather than spawning fresh sessions.

### 5.3 `internal/store/` — Persistence Layer

11 store types wrapping SQLite queries. Key patterns:

- **Generic helpers** (`scanutil.go`): `collectRows[T]`, `batchExecTx[T]` use Go
  generics for type-safe row collection and batch transaction execution.
- **Transaction support**: `TaskStore.Transact` enables atomic multi-step operations
  (e.g., dependency insertion with cycle detection).
- **Cycle detection**: `TaskStore` implements a dependency graph walk to reject
  dependency cycles at insert time.
- **Capability matching**: `SessionStore.GetByCapabilities` queries sessions by
  JSON capability array, used by the Dispatcher for session→task assignment.

### 5.4 `internal/dispatcher/` — Event Bus + Orchestration Logic

The Dispatcher is the **internal event-driven nervous system**. It does NOT talk to
agents directly — it processes events and forwards agent-spawning work to the Gateway.

**Event constants** (`events.go`):

| Constant | String | Emitted by |
|---|---|---|
| `EventTaskCompleted` | `task_status_changed` | MCP `complete_work`, REST work/complete |
| `EventTaskBlocked` | `task_blocked` | MCP `report_blocked`, REST work/block |
| `EventWorkRequested` | `work_requested` | REST work/request, status transitions |
| `EventSessionRegistered` | `session_registered` | MCP `register_session`, REST sessions |
| `EventDependencyAdded` | `dependency_added` | MCP `add_dependency`, REST task deps |
| `EventGateCheck` | `gate_check` | Dispatcher (internal) |
| `EventGateTaskCreated` | `gate_task_created` | Dispatcher `createGateTask` |
| `EventSessionStale` | `session_stale` | Gateway staleness ticker |
| `EventTasksGenerated` | `tasks_generated` | MCP `create_task` (planner) |

**Key responsibilities**:

- **Gate injection** (`gates.go`): When a `code` task completes, automatically creates
  `build` and `review` gate tasks (if story requires them). Gate task creation
  forwards to Gateway via `submitToGateway` so the agent is spawned immediately.
- **Dependency resolution** (`assignment.go`, `events.go`): When dependencies are
  added, walks dependents to unblock tasks whose deps are now satisfied.
- **Staleness** (`staleness.go`): 30-minute idle threshold; marks tasks/sessions
  stale and emits `EventSessionStale`.
- **Session-to-task matching**: Tries to match incoming work to an idle session with
  compatible capabilities before spawning a new ACP session.

**Interfaces**:

- `EventBroadcaster` (`Broadcast(eventType, payload)`) — implemented by WebSocket
  hub (HTTP mode) and `noopBroadcaster` (MCP mode).
- `GatewaySubmitter` (`SubmitEvent(ctx, event)`) — implemented by Gateway; injected
  via `Dispatcher.SetGateway()`.

### 5.5 `internal/gateway/` — ACP Session Orchestrator

The Gateway **spawns and manages ACP subprocesses** and **pushes prompts** to agent
sessions. It is the sole bridge between Loom's internal event bus and external agents
(via ACP).

**Files**:

- **`gateway.go`**: `Gateway` struct. Holds `acpClients map[string]*acp.Client`
  keyed by `acpClientKey(projectID, agentType)`, plus `sessionIDtoClient` for
  reverse lookup (re-targeting existing sessions for context updates). Implements
  `GatewaySubmitter.SubmitEvent`, enqueueing events onto `eventCh` consumed by the
  loop goroutine.
- **`loop.go`** (~750 lines): The main event loop. Reads from `eventCh`, dispatches
  via `processEvent`. Creates ACP sessions (`createACPSession`) for planner/executor/
  builder/reviewer. Builds the prompt context via `buildACPContext` (assembles
  `SystemPrompt(agentType)` + MCP server hint + story/tasks/comments + task details,
  optional `[CONTEXT UPDATE]` prefix). Maintains a staleness ticker (30s check,
  5min threshold). Drains the per-(project,agent) job queue respecting concurrency
  limits.
- **`prompts.go`**: `SystemPrompt(agentType)` returns Loom-owned system prompts for
  `planner`, `executor`, `builder`, `reviewer`, `default`. Each prompt describes
  the role, the MCP tools available, and the expected workflow.
- **`session_tracker.go`**: In-memory map of ACP sessions (`GatewaySession` with
  ProjectID, AgentType, SessionID, Status, AssignedTaskID, StoryID).
- **`queue.go`**: `JobQueue` with per-(project,agent) queues and concurrency limits
  enforced via `AgentProfile.max_concurrency` plus a global `maxTotal` cap from
  settings.
- **`types.go`**: `GatewaySession`, `GatewayStatus`, `GatewayEvent`.

**ACP session lifecycle**:

1. Gateway receives event → resolves `agentType` via `profileTaskTypes` map
   (capability matching), falling back to `task.AgentType`, payload, then
   `session.HarnessType`.
2. `getOrCreateACPClient(projectID, agentType)` lazily spawns an `opencode acp`
   subprocess with `--port <random>` and `--cwd <RepoPath>` (if set).
3. `client.Initialize()` performs ACP capability negotiation.
4. `client.NewSession()` creates a session with `mcpServers` config pointing at
   `dist/loom-server --mcp` (the MCP tool server agents call back into).
5. `client.SendPrompt()` pushes the assembled context as the initial user message.
6. Agent processes, calls MCP tools (create_task, request_work, complete_work, ...).
7. On completion / blocker / comment, MCP handlers emit events via the Dispatcher.
8. For Q&A, when the user answers a planner question, `SendContextUpdate` re-prompts
   the existing session rather than spawning fresh.

### 5.6 `internal/acp/` — ACP v1 Client (Subprocess)

Implements the **Agent Communication Protocol v1** as a stdio JSON-RPC 2.0 client.
Not a WebSocket client — `opencode acp` is a subprocess.

- **`types.go`**: ACP spec types. `MCPServer` struct has `Env []EnvVar` with
  `json:"env"` (**no `omitempty`** — schema requires the field present, even if
  empty array). `JSONRPCRequest.ID` is `int64`; `JSONRPCResponse.ID` is `*int64`
  (nullable for notifications).
- **`client.go`**: `Client` spawns `opencode acp` via `exec.Command`, communicates
  over stdin/stdout with newline-delimited JSON-RPC 2.0. Uses id-based
  request/response correlation via a `pending map[int64]chan []byte`. Methods:
  `Connect`, `Initialize`, `NewSession`, `SendPrompt`, `Receive`, `Close`.
- **`client_test.go`**: Uses `testClientWithPipes` helper for in-process pipe-based
  testing.

### 5.7 `internal/mcp/` — MCP Tool Server

The MCP server is the **agent-facing tool surface**. It runs on stdio (started by
the agent harness as specified in the ACP `mcpServers` config pushed by the
Gateway).

**Registered tools** (`tools.go`):

| Tool | Purpose |
|---|---|
| `register_session` | Agent announces itself with harness_type + capabilities |
| `request_work` | Agent asks for the next available task |
| `start_work` | Agent begins a specific task |
| `complete_work` | Agent reports task done (with result) |
| `report_blocked` | Agent reports being blocked (with reason) |
| `keep_alive` | Agent heartbeats to avoid staleness |
| `get_task` | Fetch task details by id |
| `get_blockers` | List blockers on a task |
| `add_comment` | Post a comment on story/task (e.g., planner Q&A) |
| `get_comments` | List comments on a work item |
| `get_unread_comments` | List unread comments for current session |
| `get_my_tasks` | List tasks assigned to current session |
| `add_dependency` | Declare that one task depends on another |
| `create_task` | Planner creates a child task on a story |
| `get_story` | Fetch story + its tasks (used by planner) |

The MCP server has a reference to the Dispatcher (for event submission on
`complete_work`/`report_blocked`/`add_dependency`/`create_task`) and optionally to
the Gateway (via `GatewaySubmitter` interface, nil in pure MCP mode).

### 5.8 `internal/api/` — REST API

Chi-based REST router mounted at `/api`. Handler files are split by domain
(`handlers_stories.go`, `handlers_tasks.go`, `handlers_work.go`, `handlers_auth.go`,
...). Defines its own storage interfaces (decoupled from concrete `*store.*Store`
types) for testability.

**Key endpoints**:

- Stories: CRUD + status transitions (`POST /api/stories/:id/status`) + plan
  trigger (`POST /api/stories/:id/plan`)
- Tasks: CRUD + dependencies + reorder
- Work: `/api/work/request`, `/start`, `/complete`, `/block`, `/keepalive`
- Sessions: `/api/sessions` (register)
- Board: `/api/board` (unified view state + stats)
- Activity, Comments, Profiles, Users, Settings, Templates, Projects

**Middleware**: Logger, Recovery, CORS, SessionExtractor (pulls session ID /
user from headers for agent + human auth).

**Helpers** (`helpers.go`): pagination, JSON decode, validators
(`validStatus`, `validTaskType` — includes `planning` per fix, `validAssigneeType`,
`validWorkItemType`).

### 5.9 `internal/ws/hub.go` — WebSocket Hub

Broadcasts Dispatcher events to connected UI clients. Used for live updates
(board refresh, activity feed). Not used in MCP mode (replaced by
`noopBroadcaster`). `gorilla/websocket` is still a dependency for this component,
NOT for ACP.

### 5.10 `web/` — React Frontend

Vite-built React/TypeScript SPA served by the HTTP server (with symlink
protection to prevent path traversal). Pages include the unified **OperationsPage**
(Events + Sessions tabs, merged from the old Dispatcher + Gateway pages),
StoryBoard, ProjectSettings, ProfilePage, Auth flows.

## 6. Story → Task Workflow (Agent-First Lifecycle)

```
[draft] ──user creates──► [draft]
   │
   │ user clicks "Plan Story"
   ▼
[planning] ──planner ACP session spawned──► planner calls get_story, create_task,
   │                                            add_dependency, add_comment (Q&A)
   │ user answers questions (comments)
   ▼
[ready] ──user marks ready──► Gateway spawns executor ACP sessions
   │                            for unblocked tasks (deps satisfied)
   │ executor calls request_work, start_work, complete_work
   ▼
[in_progress / done per task]
   │ on last code task completion
   ▼
Dispatcher creates [build] + [review] gate tasks (if story requires)
   │
   │ Gateway spawns builder ACP session → runs build_command
   │   ├─ PASS → mark build task done
   │   └─ FAIL → builder calls create_task (fix) → loops back to executor
   │
   │ Gateway spawns reviewer ACP session → reads diff, checks patterns
   │   ├─ APPROVED → mark review task done
   │   └─ CHANGES_REQUESTED → reviewer calls create_task (fix) → back to executor
   ▼
All gates pass → Dispatcher checks story completion → [completed]
```

**Concurrency**: Per-(project, agentType) concurrency is limited by
`AgentProfile.max_concurrency`. A global `maxTotal` cap (from settings, 0 = unlimited)
applies across all agent types. Surplus events enqueue on the `JobQueue` and drain
as capacity frees up.

## 7. Agent Profiles (Seed Data)

Seeded via `internal/db/seed.go`:

| Profile | TaskTypes | max_concurrency |
|---|---|---|
| planner | planning | (default) |
| executor | code | (default) |
| builder | build | 2 |
| reviewer | review | (default) |

`AgentProfileStore.Create()` automatically inserts `profile_task_types` rows when
`TaskTypes` is populated. The Gateway loads these at startup via `loadProfiles()`
and maintains a `profileTaskTypes map[string][]string` for capability-based agent
type resolution.

## 8. Key Architectural Decisions & Constraints

1. **ACP only pushes prompts.** ACP never carries task assignments, completions, or
   state mutations. All such mutations flow through MCP tools invoked by agents.
   This eliminated earlier double-completion bugs and protocol confusion.

2. **MCP is the toolbox; agents pull.** Agents decide which tools to call. Loom
   does not push tool calls — it pushes prompts that *tell* agents which tools exist
   and what workflow to follow (see `prompts.go`).

3. **`opencode acp` is a subprocess, not a server.** The ACP client communicates
   via stdin/stdout newline-delimited JSON-RPC 2.0. No WebSocket, no HTTP. Each
   (project, agentType) pair may have its own subprocess, lazily spawned.

4. **Pure-Go SQLite.** Uses `modernc.org/sqlite` — no CGO required. Simplifies
   cross-compilation and CI.

5. **Single binary, dual mode.** `loom-server` (HTTP) and `loom-server --mcp`
   (stdio) are the same binary, branching on a flag. This lets the same compiled
   artifact serve both as the orchestrator and as the agent-facing tool server.

6. **Loom owns the system prompts.** System prompts live in
   `internal/gateway/prompts.go` (Go constants), NOT in `opencode_config/prompts/`
   (which are templates for the opencode client itself). The Gateway prepends the
   Loom-owned system prompt to every ACP session.

7. **Schema compliance over convenience.** The `MCPServer.Env` field uses
   `json:"env"` (no `omitempty`) because the ACP v1 schema lists `env` as required
   for `McpServerStdio`. Construction sites pass `Env: []acp.EnvVar{}`
   explicitly. Lesson: match the wire schema, not Go idioms.

8. **Dependency-injected storage.** `api`, `mcp`, `dispatcher`, and `gateway` all
   consume stores via interfaces, not concrete types, enabling test substitution.

9. **Event-driven; not request-driven.** The Dispatcher processes an internal event
   bus. REST handlers and MCP tool handlers emit events; the Dispatcher reacts
   (gates, staleness, dependency unblock) and forwards agent-spawning work to the
   Gateway. This decouples user/agent actions from orchestration logic.

10. **Sessions persist `agent_session_id`.** Both stories (for planner) and tasks
    (for executor/builder/reviewer) store the ACP session ID. This enables Q&A
    context updates via `session/prompt` re-send to an existing session rather
    than spawning fresh sessions.

## 9. ACP v1 Protocol Conformance

Loom's ACP client implements the official Agent Communication Protocol v1
(https://agentclientprotocol.com). The standard flow:

1. **`initialize`** — capability negotiation. Loom declares its client info and
   supported protocol version (1). `opencode acp` responds with its capabilities.
2. **`session/new`** — create a session. Loom passes `cwd`, `mcpServers`
   (pointing at `dist/loom-server --mcp`), and optional `additionalDirectories`.
   The `mcpServers` schema requires `name`, `command`, `args`, `env` all present
   (env may be `[]`).
3. **`session/prompt`** — send a prompt to the session. Loom passes the assembled
   context as a single text `ContentBlock`. For Q&A follow-ups, the same session
   receives additional `session/prompt` calls with `[CONTEXT UPDATE]` prefix.

All requests use id-based correlation (`JSONRPCRequest.ID` int64 →
`JSONRPCResponse.ID` *int64). Notifications (server-initiated) have `id: null`.

## 10. Build, Test, & Run

| Command | What it does |
|---|---|
| `make run` | `go run ./cmd/server` (HTTP mode) |
| `go build ./...` | Build all packages |
| `go vet ./...` | Static analysis |
| `go test ./...` | Run all Go tests |
| `cd web && npm run build` | Build frontend |
| `cd web && npm test` | Run frontend tests |

## 11. Update History

### 2026-07-14 — Session Worktree Pinning Constraint

An executor ACP session is pinned to the **worktree of the first story** it works on.
The `--cwd` (repository path) is set at session birth during `session/new` negotiation
and does **not** change for subsequent tasks from different stories assigned to the
same session. This means a reused session always executes from the worktree it was
originally spawned for, regardless of which story's tasks it picks up.

**Implication**: Cross-story isolation on reused sessions would require the Gateway
to re-resolve the target worktree, tear down the existing session, and re-spawn with
a fresh `session/new` carrying the updated `--cwd`. A lighter-weight approach — not
currently implemented — would be to re-session/prompt with a `[CONTEXT UPDATE]` that
includes the new worktree, but the ACP `cwd` itself remains immutable once set.

### 2026-07-13 — SDLC Pipeline expansion

Added `TaskTypeSecurity` and `TaskTypeRelease` constants; migration 010; agent
profiles; 3-stage gate chain; worktree isolation; file-collision scheduling; circuit
breaker. See session `plan-d955fc`.

### 2026-07-13 — Initial architecture document created

Covered the post-refactor state: ACP v1 protocol alignment; MCP/ACP layer separation;
agent-first workflow; Loom-owned system prompts; `MCPServer.Env` schema compliance
fix.