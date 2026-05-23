# PROJECT SOUL

Loom is a dual-stack application: a Go backend serving as an AI agent orchestration platform (stories, tasks, sessions, dispatcher, MCP protocol), paired with a React/TypeScript frontend for board visualization and agent monitoring.

The Go backend follows a layered architecture: models → store (data access) → api (HTTP handlers) → dispatcher (task orchestration) → ws (WebSocket real-time). The frontend uses React with custom hooks, Zustand-like stores, and a centralized API client.

Key architectural concerns for audit:
- The dispatcher is the brain of the system — it assigns work to sessions, manages staleness, and gates task flow. This is the most complex domain.
- The store layer uses raw `*sql.DB` with inline SQL — no ORM, no repository abstraction beyond struct methods.
- The API layer has 13 files with handlers directly coupled to store interfaces defined in router.go.
- The MCP server implements JSON-RPC for tool-based agent interaction.
- The frontend has significant utility sprawl (5 small util files that could be consolidated).

# TECH STACK
LANGUAGE: Go, TypeScript/TSX
SKILLS: lint-project, verify-project

# ARCHITECTURAL DECISIONS

- Chunk boundaries follow bounded contexts: Go backend layers are separated from Node.js frontend domains.
- Go store layer is split into two chunks (core entities vs. supporting entities) to keep file counts manageable.
- Go API handlers are split into two chunks (core CRUD vs. work/auth/sessions) for deep line-by-line analysis.
- The dispatcher gets its own dedicated chunk — it is the most complex subsystem.
- Frontend is split into: components, hooks+stores, and API+types+utils.
- Each chunk targets 5-12 files maximum for thorough audit coverage.

---

## TASK-001: Go Models & Database Layer Audit
TARGET_FILES: internal/models/models.go, internal/db/sqlite.go, internal/db/seed.go, internal/config/config.go, cmd/server/main.go
DETAILS: Audit the foundational layer. Review struct definitions in models.go for proper Go conventions (json tags, time handling, pointer vs value semantics). Examine sqlite.go for connection management, migration application, and SQL injection safety. Check seed.go for template loading logic. Review config.go for environment variable handling and defaults. Examine main.go for initialization order, dependency wiring, and graceful shutdown patterns. Flag any God objects, missing context propagation, or improper resource lifecycle management.

## TASK-002: Go Store Layer — Core Entities (Stories & Tasks)
TARGET_FILES: internal/store/stories.go, internal/store/stories_test.go, internal/store/tasks.go, internal/store/tasks_test.go, internal/store/errors.go
DETAILS: Audit the data access layer for the two primary domain entities. Review SQL query construction (parameterized vs string concatenation), transaction handling, error wrapping patterns, and test coverage quality. Check for N+1 query patterns in GetWithTasks or dependency resolution. Evaluate whether StoryFilter and TaskFilter structs are extensible. Review the cycle detection logic in tasks.go for correctness and performance. Ensure all public methods accept context.Context as first parameter.

## TASK-003: Go Store Layer — Supporting Entities
TARGET_FILES: internal/store/sessions.go, internal/store/sessions_test.go, internal/store/comments.go, internal/store/activity.go, internal/store/templates.go, internal/store/users.go
DETAILS: Audit the supporting data access stores. Review session management (registration, disconnection, active listing), comment threading, activity logging, template CRUD, and user authentication storage. Check for consistent error handling patterns across all stores. Verify that session-related queries handle concurrent access safely. Review users.go for password hashing implementation and session token management. Ensure store interfaces in router.go match actual implementations.

## TASK-004: Go API Handlers — Core CRUD (Stories, Tasks, Board, Templates)
TARGET_FILES: internal/api/handlers_stories.go, internal/api/handlers_stories_test.go, internal/api/handlers_tasks.go, internal/api/handlers_board.go, internal/api/handlers_templates.go, internal/api/router.go
DETAILS: Audit the HTTP handler layer for core resource management. Review request/response struct definitions for proper JSON tagging and validation. Check for consistent HTTP status code usage, error response formatting, and input sanitization. Examine router.go for route organization, middleware chain order, and interface definitions. Review the BoardState aggregation logic for efficiency. Check handlers_stories_test.go for test patterns and coverage gaps. Verify that all handlers properly propagate context to store calls.

## TASK-005: Go API Handlers — Auth, Sessions, Work, Comments, Activity
TARGET_FILES: internal/api/handlers_auth.go, internal/api/handlers_sessions.go, internal/api/handlers_work.go, internal/api/handlers_work_test.go, internal/api/handlers_comments.go, internal/api/handlers_activity.go, internal/api/middleware.go
DETAILS: Audit authentication, session management, work assignment workflow, comments, and activity logging handlers. Review auth.go for secure password handling, token generation, and session management. Examine the work assignment state machine (request → start → complete/block → keepalive) for race conditions. Check middleware.go for authentication/authorization middleware patterns and context key usage. Review handlers_work_test.go for mock patterns and edge case coverage. Verify comment authorship validation and activity log consistency.

## TASK-006: Go Dispatcher — Task Orchestration Engine
TARGET_FILES: internal/dispatcher/dispatcher.go, internal/dispatcher/dispatcher_test.go, internal/dispatcher/assignment.go, internal/dispatcher/gates.go, internal/dispatcher/staleness.go, internal/dispatcher/prompt.go
DETAILS: Audit the core orchestration engine. This is the most critical subsystem. Review the event-driven architecture (eventCh, done channel, goroutine lifecycle). Examine assignment logic for fairness, capability matching, and deadlock prevention. Review gates.go for task readiness criteria. Check staleness.go for timeout handling and resource cleanup. Examine prompt.go for template rendering and prompt construction. Review dispatcher_test.go and mockBroadcaster for test completeness. Check for goroutine leaks, channel deadlocks, and proper shutdown sequencing. Evaluate the EventBroadcaster interface usage.

## TASK-007: Go MCP Server & WebSocket Hub
TARGET_FILES: internal/mcp/server.go, internal/mcp/tools.go, internal/ws/hub.go
DETAILS: Audit the MCP (Model Context Protocol) JSON-RPC server and WebSocket real-time communication layer. Review MCP server.go for proper JSON-RPC request/response handling, tool registration, and error formatting. Examine tools.go for tool handler implementations and their integration with stores/dispatcher. Review ws/hub.go for WebSocket connection management, broadcast fan-out efficiency, client lifecycle (register/unregister), and memory leak prevention. Check for proper use of sync.RWMutex, channel buffer sizing, and graceful shutdown. Verify the Hub implements HubInterface correctly.

## TASK-008: Go Test Helpers & Cross-Cutting Concerns
TARGET_FILES: internal/testhelpers/testhelpers.go
DETAILS: Audit the test helper utilities. Review database setup/teardown patterns, mock generation helpers, and test fixture management. Check for test isolation (no shared state between tests), proper use of t.Cleanup, and consistent naming conventions. Evaluate whether the test helpers promote good testing practices or enable anti-patterns. Cross-reference with all _test.go files to identify testing gaps or inconsistent patterns.

## TASK-009: Frontend — React Components Audit
TARGET_FILES: web/src/components/Board.tsx, web/src/components/Column.tsx, web/src/components/StoryCard.tsx, web/src/components/StoryDetail.tsx, web/src/components/TaskCard.tsx, web/src/components/TaskDetail.tsx, web/src/components/CommentThread.tsx, web/src/components/CreateStoryForm.tsx, web/src/components/DependencyGraph.tsx, web/src/components/Layout.tsx, web/src/components/TopNav.tsx, web/src/components/ConfirmModal.tsx, web/src/components/SharpTag.tsx, web/src/components/StatCard.tsx
DETAILS: Audit all React components for proper TypeScript typing, component composition patterns, prop drilling vs context usage, and rendering performance. Review Board.tsx and Column.tsx for state management and drag-and-drop patterns. Check StoryCard.tsx and TaskCard.tsx for consistent card patterns and stale status handling. Examine DependencyGraph.tsx for graph rendering complexity. Review ConfirmModal.tsx for accessibility. Check for duplicated styling logic, inline styles vs CSS classes, and proper event handler memoization.

## TASK-010: Frontend — Hooks, Stores & Pages
TARGET_FILES: web/src/hooks/useBoard.ts, web/src/hooks/useActivity.ts, web/src/hooks/useSessions.ts, web/src/hooks/useCreateStory.ts, web/src/hooks/useTheme.ts, web/src/hooks/useWebSocket.ts, web/src/stores/session.ts, web/src/stores/theme.ts, web/src/pages/ActivityPage.tsx, web/src/pages/AgentsPage.tsx
DETAILS: Audit custom React hooks for proper dependency arrays, cleanup functions, and error handling. Review useWebSocket.ts for connection lifecycle, reconnection logic, and message handling. Check useBoard.ts and useActivity.ts for data fetching patterns and loading states. Review stores (session.ts, theme.ts) for state management patterns and persistence. Examine page components for proper routing integration and data composition. Check for hook composition patterns and whether hooks are properly isolated from component concerns.

## TASK-011: Frontend — API Client, Types & Utilities
TARGET_FILES: web/src/api/client.ts, web/src/types/index.ts, web/src/utils/relativeTime.ts, web/src/utils/statusConstants.ts, web/src/utils/statusVariant.ts, web/src/utils/taskTypeLabel.ts, web/src/utils/taskTypeVariant.ts, web/src/App.tsx, web/src/main.tsx, web/public/darkMode.js
DETAILS: Audit the centralized API client for error handling, retry logic, request/response interceptors, and type safety. Review types/index.ts for comprehensive type coverage matching Go backend models. Examine utility files for potential consolidation (statusConstants + statusVariant + taskTypeVariant + taskTypeLabel could potentially be unified). Check App.tsx and main.tsx for proper React initialization, routing setup, and provider composition. Review darkMode.js for proper DOM manipulation and SSR compatibility.
