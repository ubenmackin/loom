package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ubenmackin/loom/internal/acp"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// SettingKeyGlobalMaxConcurrency is the settings table key for the global
// max concurrency cap on the gateway's job queue.
const SettingKeyGlobalMaxConcurrency = "global_max_concurrency"

// SettingKeyGitWorktreeRoot is the settings table key for the git worktree
// root directory used for story isolation.
const SettingKeyGitWorktreeRoot = "git_worktree_root"

// ---------------------------------------------------------------------------
// Store interfaces — minimal subsets of the store interfaces used by the
// gateway. These are intentionally narrower than the full-store interfaces
// in internal/api and internal/store so that the gateway only depends on
// what it actually needs.
// ---------------------------------------------------------------------------

// TaskStore defines the gateway's task storage requirements.
type TaskStore interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	UpdateStatus(ctx context.Context, id string, status models.Status) error
	GetByStory(ctx context.Context, storyID string) ([]*models.Task, error)
}

// SessionStore defines the gateway's session storage requirements.
type SessionStore interface {
	Register(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id string) (*models.Session, error)
	UpdateLastSeen(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]*models.Session, error)
	Disconnect(ctx context.Context, id string) error
}

// ProjectStore defines the gateway's project storage requirements.
type ProjectStore interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
	List(ctx context.Context) ([]*models.Project, error)
}

// StoryStore defines the gateway's story storage requirements.
type StoryStore interface {
	GetByID(ctx context.Context, id string) (*models.Story, error)
	Update(ctx context.Context, story *models.Story) error
}

// CommentStore defines the gateway's comment storage requirements.
type CommentStore interface {
	Create(ctx context.Context, c *models.Comment) error
	GetByWorkItem(ctx context.Context, workItemID string, workItemType models.WorkItemType) ([]*models.Comment, error)
}

// ActivityStore defines the gateway's activity log storage requirements.
type ActivityStore interface {
	Log(ctx context.Context, entry *models.ActivityLogEntry) error
}

// AgentProfileStore defines the gateway's agent profile storage requirements.
type AgentProfileStore interface {
	List(ctx context.Context) ([]*models.AgentProfile, error)
	GetByID(ctx context.Context, id string) (*models.AgentProfile, error)
}

// SettingStore defines the gateway's settings storage requirements.
type SettingStore interface {
	Get(ctx context.Context, key string) (string, error)
}

// ---------------------------------------------------------------------------
// Gateway is the main orchestrator that manages ACP agent sessions and
// proactively pushes work to opencode serve sessions. It runs as a background
// goroutine with an event-driven loop.
// ---------------------------------------------------------------------------

// Gateway is the main orchestrator that manages ACP agent sessions (spawned as
// subprocesses) and proactively pushes work to opencode serve sessions.
type Gateway struct {
	dispatcher        *dispatcher.Dispatcher
	tracker           *SessionTracker
	queue             *JobQueue
	acpClients        map[string]*acp.Client // key: "projectID:agentType"
	sessionIDtoClient map[string]*acp.Client // key: ACP session ID → client

	taskStore     TaskStore
	sessionStore  SessionStore
	projectStore  ProjectStore
	storyStore    StoryStore
	commentStore  CommentStore
	activityStore ActivityStore
	profileStore  AgentProfileStore

	hub dispatcher.EventBroadcaster // WebSocket hub for status broadcasts

	mu      sync.RWMutex
	eventCh chan dispatcher.Event
	done    chan struct{}
	wg      sync.WaitGroup
	started atomic.Bool
	stopped atomic.Bool

	eventsProcessed atomic.Int64
	startedAt       time.Time

	acpCommand     string                          // e.g., "opencode acp"
	profilesByName map[string]*models.AgentProfile // profile name -> full profile (protected by mu); used to resolve Prompt for ProfilePrompt (TASK-006)
	filesInUse     map[string]string               // file path → taskID (protected by mu)

	// missingBlocks records per-project opencode configuration mismatches
	// (Decisions 4 / TASK-010): the routing layer (routeSessionMode in
	// loop.go) calls recordConfigMismatch when the requested agent block is
	// not in the modes the opencode session actually advertised, and the
	// auto-clear hook prunes entries once the agent advertises them again.
	// Both the outer and inner maps are guarded by g.mu. Outer key is
	// projectID, inner key is the block name (e.g. "missing_block:executor").
	// NOTE: no TTL is applied in v1; future work may swap the inner count
	// for a last-seen timestamp and prune stale entries.
	missingBlocks map[string]map[string]int // protected by mu

	worktreeManager *WorktreeManager // git worktree management for story isolation
	settingStore    SettingStore     // settings store for dynamic configuration
}

// NewGateway creates a new Gateway with the given dependencies. The acpCommand
// configures the ACP subprocess command (e.g., "opencode acp"). The
// maxTotal parameter sets a global cap on active sessions (0 = unlimited).
// The gateway does not start processing events until Start() is called.
func NewGateway(
	d *dispatcher.Dispatcher,
	acpCommand string,
	maxTotal int,
	taskStore TaskStore,
	sessionStore SessionStore,
	projectStore ProjectStore,
	storyStore StoryStore,
	commentStore CommentStore,
	activityStore ActivityStore,
	profileStore AgentProfileStore,
	settingStore SettingStore,
	hub dispatcher.EventBroadcaster,
) *Gateway {
	q := NewJobQueue()
	q.SetMaxTotal(maxTotal)

	// Load global_max_concurrency from setting store, overriding the
	// hardcoded maxTotal default if present.
	if settingStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		v, err := settingStore.Get(ctx, SettingKeyGlobalMaxConcurrency)
		if err != nil {
			slog.Warn("gateway: failed to load global_max_concurrency from settings store, using default",
				"error", err)
		} else if n, err := strconv.Atoi(v); err != nil {
			slog.Warn("gateway: global_max_concurrency has invalid value, using default",
				"value", v, "error", err)
		} else if n >= 0 {
			q.SetMaxTotal(n)
			slog.Info("gateway: loaded global_max_concurrency from settings store", "value", n)
		}
	}

	return &Gateway{
		dispatcher:        d,
		tracker:           NewSessionTracker(),
		queue:             q,
		acpClients:        make(map[string]*acp.Client),
		sessionIDtoClient: make(map[string]*acp.Client),
		taskStore:         taskStore,
		sessionStore:      sessionStore,
		projectStore:      projectStore,
		storyStore:        storyStore,
		commentStore:      commentStore,
		activityStore:     activityStore,
		profileStore:      profileStore,
		hub:               hub,
		eventCh:           make(chan dispatcher.Event, 256),
		done:              make(chan struct{}),
		acpCommand:        acpCommand,
		profilesByName:    make(map[string]*models.AgentProfile),
		filesInUse:        make(map[string]string),
		missingBlocks:     make(map[string]map[string]int),
		worktreeManager:   NewWorktreeManager(".loom/worktrees"),
		settingStore:      settingStore,
	}
}

// Start launches the background gateway event loop. It is safe to call
// multiple times — subsequent calls are no-ops.
func (g *Gateway) Start() {
	if g.started.Swap(true) {
		return
	}
	g.startedAt = time.Now()

	ctx := context.Background()
	if err := g.loadProfiles(ctx); err != nil {
		slog.Error("gateway: failed to load profiles", "error", err)
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.run()
	}()
	slog.Info("gateway started", "acp_command", g.acpCommand)
}

// Stop signals the gateway to shut down gracefully. It is idempotent:
// subsequent calls are no-ops. Stop waits for the event loop to finish.
func (g *Gateway) Stop() {
	if g.stopped.Swap(true) {
		return
	}
	close(g.done)

	// Collect client references under lock so we can close them outside
	// the lock. This avoids two deadlocks:
	//   1) g.wg.Wait() blocking forever because readACPResponses is
	//      ranging over receiveCh, which only closes when the subprocess
	//      exits (triggered by client.Close()).
	//   2) client.Close() triggering subprocess exit → receiveCh close →
	//      readACPResponses defer calls removeACPClient() which
	//      re-acquires g.mu.
	g.mu.Lock()
	clients := make([]*acp.Client, 0, len(g.acpClients))
	for _, client := range g.acpClients {
		clients = append(clients, client)
	}
	g.mu.Unlock()

	// Close all clients first (outside the lock) so their receiveCh
	// closes and readACPResponses goroutines can terminate.
	for _, client := range clients {
		if err := client.Close(); err != nil {
			slog.Warn("gateway: error closing acp client",
				"error", err)
		}
	}

	// Now wait for all goroutines (including readACPResponses) to finish.
	g.wg.Wait()

	slog.Info("gateway stopped")
}

// SubmitEvent provides a thread-safe way to submit events to the gateway's
// event loop. If the event channel is full, the call blocks until the event
// is delivered or the gateway is shut down.
func (g *Gateway) SubmitEvent(event dispatcher.Event) {
	select {
	case g.eventCh <- event:
	case <-g.done:
		// Gateway is shutting down; discard event.
	}
}

// Status returns a snapshot of the current gateway runtime state.
func (g *Gateway) Status() GatewayStatus {
	g.mu.RLock()
	sessionsByProject := make(map[string]int)
	sessionsByAgent := make(map[string]int)
	g.mu.RUnlock()

	for _, s := range g.tracker.ListAll() {
		sessionsByProject[s.ProjectID]++
		sessionsByAgent[s.AgentType]++
	}

	now := time.Now()
	uptime := int64(0)
	if !g.startedAt.IsZero() {
		uptime = int64(now.Sub(g.startedAt).Seconds())
	}

	return GatewayStatus{
		Running:           !g.stopped.Load(),
		ActiveSessions:    g.tracker.Count(),
		QueueDepth:        g.queue.TotalLen(),
		EventsProcessed:   g.eventsProcessed.Load(),
		UptimeSeconds:     uptime,
		SessionsByProject: sessionsByProject,
		SessionsByAgent:   sessionsByAgent,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// acpClientKey builds the composite key for the acpClients map.
func acpClientKey(projectID, agentType string) string {
	return fmt.Sprintf("%s:%s", projectID, agentType)
}

// RegisterSessionClient maps an ACP session ID to its client for future
// context updates. The sessionID is the actual ACP session ID received
// from the create_session response.
func (g *Gateway) RegisterSessionClient(sessionID string, client *acp.Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessionIDtoClient[sessionID] = client
}

// SendContextUpdate sends a context update to an existing ACP session.
// It looks up the client by session ID, builds the new context with
// isUpdate=true, and sends it as a prompt.
//
// NOTE (TASK-005): per the spec, the Q&A resume path (user answers a planner
// question — see handlers_comments.go) re-issues the session/set_config_option
// routing call before sending the new prompt, so the resumed prompt uses the
// intended opencode agent block. The session's availableModes are looked up
// from the tracker by ACP session ID; on an empty payload no-op degrade.
func (g *Gateway) SendContextUpdate(ctx context.Context, sessionID, storyID, taskID, agentType, newContext string) error {
	g.mu.RLock()
	client, ok := g.sessionIDtoClient[sessionID]
	g.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no client found for session %q", sessionID)
	}

	// Re-apply session mode routing so the resumed Q&A prompt is pinned to
	// the intended opencode agent block. We resolve the per-subprocess
	// GatewaySession by ACP session ID to retrieve the stashed
	// gs.AvailableModes (populated at the original NewSessionWithModes call
	// by TASK-004). Routing failure is non-fatal — log and continue with
	// the prompt send so a mode-routing glitch does not block the Q&A flow.
	if gs, ok := g.tracker.GetBySessionID(sessionID); ok {
		role := agentType
		if role == "" {
			role = gs.AgentType
		}
		// Read the stashed available modes through the tracker accessor
		// (Stage-4 audit fix). The tracker's internal lock only guards
		// map membership, not the GatewaySession struct fields, so a bare
		// gs.AvailableModes read races with concurrent SetAvailableModes
		// writes during the ACP session-create path.
		availableModes := g.tracker.GetAvailableModesBySessionID(sessionID)
		if rErr := g.routeSessionMode(ctx, client, sessionID, gs.ProjectID, role, agentType, availableModes); rErr != nil {
			slog.Warn("gateway: failed to re-route session mode on context update, continuing",
				"project_id", gs.ProjectID,
				"agent_type", gs.AgentType,
				"session_id", sessionID,
				"error", rErr)
		}
	}

	// We accept newContext as a parameter, OR we build it here.
	// The caller can pass empty string to have it built, or pass the pre-built context.
	contextToSend := newContext
	if contextToSend == "" {
		var err error
		contextToSend, err = g.buildACPContext(ctx, storyID, taskID, agentType, true)
		if err != nil {
			return fmt.Errorf("build context update: %w", err)
		}
	}

	_, err := client.SendPrompt(ctx, sessionID, contextToSend)
	return err
}

// randomPort allocates a random available TCP port by listening on :0.
func randomPort() (port int, err error) {
	addr, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	tcpAddr, ok := addr.Addr().(*net.TCPAddr)
	if !ok {
		_ = addr.Close()
		return 0, fmt.Errorf("expected TCPAddr, got %T", addr.Addr())
	}
	port = tcpAddr.Port

	return port, addr.Close()
}

// getOrCreateACPClient returns an existing ACP client for the given
// (projectID, agentType) pair, or creates a new one by spawning the
// ACP subprocess. This is a best-effort operation — if the
// connection fails, the error is returned and the caller should queue
// the work rather than fail.
func (g *Gateway) getOrCreateACPClient(ctx context.Context, projectID, agentType string) (*acp.Client, error) {
	key := acpClientKey(projectID, agentType)

	g.mu.RLock()
	client, exists := g.acpClients[key]
	g.mu.RUnlock()

	if exists && client.IsConnected() {
		return client, nil
	}

	port, err := randomPort()
	if err != nil {
		return nil, fmt.Errorf("allocate port for %s/%s: %w", projectID, agentType, err)
	}

	newClient := acp.NewClient(g.acpCommand)
	newClient.Args = []string{"--port", strconv.Itoa(port)}

	project, err := g.projectStore.GetByID(ctx, projectID)
	if err != nil {
		slog.Warn("gateway: failed to look up project for --cwd",
			"project_id", projectID, "error", err)
	} else if project.RepoPath != "" {
		newClient.Args = append(newClient.Args, "--cwd", project.RepoPath)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := newClient.Connect(connectCtx); err != nil {
		return nil, fmt.Errorf("connect acp client for %s/%s: %w", projectID, agentType, err)
	}

	g.mu.Lock()
	// Double-check — another goroutine may have created one while we
	// were connecting. If so, close ours and return the existing one.
	if existing, ok := g.acpClients[key]; ok && existing.IsConnected() {
		g.mu.Unlock()
		_ = newClient.Close()
		return existing, nil
	}
	g.acpClients[key] = newClient
	g.mu.Unlock()

	// Start a goroutine to read ACP responses from this client only after
	// the client has been registered in the map, so the deferred cleanup
	// in readACPResponses can reliably remove the correct entry.
	g.wg.Add(1)
	go g.readACPResponses(ctx, newClient, projectID, agentType)

	slog.Info("gateway: created acp client",
		"project_id", projectID, "agent_type", agentType,
		"command", newClient.Command, "args", newClient.Args, "port", port)

	return newClient, nil
}

// readACPResponses reads messages from an ACP client's receive channel and
// processes them. It runs in a background goroutine per client.
func (g *Gateway) readACPResponses(ctx context.Context, client *acp.Client, projectID, agentType string) {
	defer g.wg.Done()
	defer g.removeACPClient(projectID, agentType)

	receiveCh, err := client.Receive()
	if err != nil {
		slog.Warn("gateway: failed to get receive channel",
			"project_id", projectID, "agent_type", agentType, "error", err)
		return
	}

	for msg := range receiveCh {
		g.handleACPMessage(ctx, msg, projectID, agentType, client)
	}

	slog.Info("gateway: acp client receive channel closed",
		"project_id", projectID, "agent_type", agentType)
}

// removeACPClient removes an ACP client from the acpClients map and closes
// the underlying connection. It is safe for concurrent use.
func (g *Gateway) removeACPClient(projectID, agentType string) {
	key := acpClientKey(projectID, agentType)

	g.mu.Lock()
	client, ok := g.acpClients[key]
	if ok {
		delete(g.acpClients, key)
		// Also remove from sessionIDtoClient
		for sid, c := range g.sessionIDtoClient {
			if c == client {
				delete(g.sessionIDtoClient, sid)
			}
		}
	}
	g.mu.Unlock()

	if ok && client != nil {
		if err := client.Close(); err != nil {
			slog.Warn("gateway: error closing acp client during removal",
				"key", key, "error", err)
		}
		slog.Info("gateway: removed acp client",
			"project_id", projectID, "agent_type", agentType)
	}
}

// Queue returns a reference to the gateway's JobQueue, allowing external
// consumers (e.g., REST API handlers) to inspect queued jobs.
func (g *Gateway) Queue() *JobQueue {
	return g.queue
}

// logActivity is a helper that logs an activity entry and logs any error. The
// projectID argument scopes the entry to a project so activity can be
// filtered by project without joining through the work item.
func (g *Gateway) logActivity(ctx context.Context, workItemID, workItemType, action, details, projectID string) {
	entry := &models.ActivityLogEntry{
		WorkItemID:   workItemID,
		WorkItemType: models.WorkItemType(workItemType),
		Action:       action,
		Details:      details,
		ProjectID:    projectID,
	}
	if err := g.activityStore.Log(ctx, entry); err != nil {
		slog.Error("gateway: failed to log activity",
			"work_item_id", workItemID,
			"action", action,
			"error", err)
	}
}

// ReloadProfiles reloads all agent profiles from the database and updates
// concurrency limits in the job queue. This allows profile changes made
// via the REST API to take effect without a server restart.
func (g *Gateway) ReloadProfiles(ctx context.Context) error {
	return g.loadProfiles(ctx)
}

// loadProfiles loads all agent profiles from the database and configures the
// gateway's concurrency limits accordingly.
func (g *Gateway) loadProfiles(ctx context.Context) error {
	profiles, err := g.profileStore.List(ctx)
	if err != nil {
		return fmt.Errorf("load agent profiles: %w", err)
	}

	g.mu.Lock()
	// Rebuild the profile-name → full-profile lookup. The capability-search
	// map (profileTaskTypes) and the role-key map that predated TASK-005
	// were both removed; routing is now driven directly by task.AgentType
	// with a defaultRoleForTaskType fallback, and per-profile prompt text
	// is resolved through profilesByName + ProfilePrompt (TASK-006).
	// profilesByName retains the full loaded profile (including the
	// prompt column added in migration 016) so buildACPContext can resolve
	// the per-profile prompt text via ProfilePrompt (TASK-006).
	g.profilesByName = make(map[string]*models.AgentProfile, len(profiles))
	for _, p := range profiles {
		g.queue.SetConcurrency(p.Name, p.MaxConcurrency)
		g.profilesByName[p.Name] = p
		slog.Info("gateway: configured concurrency from profile",
			"agent_type", p.Name,
			"max_concurrency", p.MaxConcurrency,
			"agent_role", p.AgentRole)
	}
	g.mu.Unlock()

	return nil
}

// defaultRoleForTaskType returns the canonical opencode agent block name (the
// "mode" advertised by opencode serve) for a given models.TaskType string.
// It is used as a fallback when a task has no explicit AgentType. The table is
// static; unrecognized task types (including the empty string) fall back to
// "planner".
//
// NOTE (TASK-005): routing is now driven by task.AgentType (with this helper
// as the fallback) instead of the old capability-search profileTaskTypes map.
func defaultRoleForTaskType(taskType string) string {
	switch taskType {
	case "planning":
		return "planner"
	case "code":
		return "executor"
	case "build":
		return "builder"
	case "review":
		return "reviewer"
	case "security":
		return "security-auditor"
	case "release":
		return "release-manager"
	case "workspace_setup":
		return "workspace-setup"
	case "":
		return "planner"
	default:
		return "planner"
	}
}

// recordConfigMismatch records a per-project opencode configuration
// mismatch surfaced by the routing layer (see routeSessionMode in loop.go):
// the requested agent block was not in the modes the opencode session
// actually advertised. The mismatch is logged AND accumulated in
// g.missingBlocks (Decisions 4 / TASK-010) so the API surface
// (GET /api/projects/{id}/config-status via MissingOpencodeBlocks) can
// surface it to the user.
//
// The accumulated entry key is shaped as "missing_block:<requestedAgentType>"
// by the caller. Both the outer (projectID) and inner (blockName) maps are
// guarded by g.mu. The inner value is a monotonically increasing count — v1
// does not apply a TTL; see the missingBlocks field comment for future work.
func (g *Gateway) recordConfigMismatch(projectID, blockName string) {
	slog.Warn("gateway: opencode config mismatch recorded",
		"project_id", projectID,
		"block_name", blockName)

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.missingBlocks == nil {
		g.missingBlocks = make(map[string]map[string]int)
	}
	inner, ok := g.missingBlocks[projectID]
	if !ok {
		inner = make(map[string]int)
		g.missingBlocks[projectID] = inner
	}
	inner[blockName]++
}

// MissingOpencodeBlocks returns a deterministic, deduplicated snapshot of the
// opencode block names recorded as missing for the given project. The slice
// is sorted lexicographically so the API response is stable for caching and
// for the UI to render deterministically. Returns an empty (non-nil) slice
// when no entries are recorded for the project.
//
// This is the read surface consumed by GET /api/projects/{id}/config-status
// (internal/api/config_status.go, TASK-010). It is read-only and safe to
// call concurrently with recordConfigMismatch / ClearMissingBlocksForProject.
func (g *Gateway) MissingOpencodeBlocks(projectID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inner, ok := g.missingBlocks[projectID]
	if !ok || len(inner) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(inner))
	for name := range inner {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ClearMissingBlocksForProject prunes the per-project missing-blocks map
// down to entries NOT present in availableModes. It is the auto-clear hook
// (Decisions 4 / TASK-010): once the opencode session advertises a block
// (captured in gs.AvailableModes by loop.go after NewSessionWithModes),
// any prior "missing_block:<agentType>" record for that block becomes stale
// and is dropped so the UI stops warning the user.
//
// The match is on the recorded block-name suffix: recorded keys are shaped
// as "missing_block:<requestedAgentType>", so a recorded entry is cleared
// when availableModes contains the "<requestedAgentType>" suffix. Entries
// that do not correspond to any advertised mode are retained.
func (g *Gateway) ClearMissingBlocksForProject(projectID string, availableModes []string) {
	if len(availableModes) == 0 {
		return
	}
	available := make(map[string]struct{}, len(availableModes))
	for _, m := range availableModes {
		available[m] = struct{}{}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	inner, ok := g.missingBlocks[projectID]
	if !ok || len(inner) == 0 {
		return
	}
	const prefix = "missing_block:"
	for recorded := range inner {
		if !strings.HasPrefix(recorded, prefix) {
			continue
		}
		suffix := recorded[len(prefix):]
		if _, has := available[suffix]; has {
			delete(inner, recorded)
		}
	}
	if len(inner) == 0 {
		delete(g.missingBlocks, projectID)
	}
}

// routeSessionMode issues a session/set_config_option call (configId="mode")
// against a freshly-created or resumed ACP session so opencode uses the
// intended agent block for subsequent prompts. The intended role is validated
// against the modes the opencode session actually advertised
// (gs.AvailableModes, populated by TASK-004 from NewSessionWithModes).
//
// Behavior:
//   - If availableModes is nil/empty: skip the call entirely (older opencode
//     returns no modes payload; let opencode use its default_agent). A
//     one-line debug log records the absence.
//   - If role is in availableModes: send set_config_option(mode=role).
//   - If role is NOT in availableModes: log a warning, record
//     "missing_block:<requestedAgentType>" via recordConfigMismatch, and
//     degrade to availableModes[0] (or "" if the slice is somehow empty
//     after the non-nil check), then send set_config_option with the
//     degraded value.
//
// requestedAgentType is the role originally requested for the task (matching
// task.AgentType); it is recorded as the source of the mismatch to make the
// degradation traceable to which opencode block was expected.
//
// This helper is invoked by loop.go after every NewSessionWithModes call
// (createACPSession + assignTaskToSession !ok branch) and after the
// SendContextUpdate resume path re-spawns a session.
//
// The client parameter accepts a narrow interface rather than *acp.Client
// directly so tests (TASK-011) can substitute a mock that records
// SetSessionConfigOption calls without standing up a real ACP subprocess.
// The concrete *acp.Client satisfies this interface in production.
func (g *Gateway) routeSessionMode(
	ctx context.Context,
	client modeRoutingClient,
	sessionID, projectID, role, requestedAgentType string,
	availableModes []string,
) error {
	if len(availableModes) == 0 {
		// Older opencode did not advertise any modes payload. Skip the
		// set_config_option call so opencode uses its default_agent.
		slog.Debug("gateway: no available modes advertised, skipping session mode routing",
			"project_id", projectID,
			"session_id", sessionID,
			"requested_role", role)
		return nil
	}

	// Validate role against advertised modes; degrade to the first mode
	// (or empty string) when the requested role is not supported.
	resolvedRole := role
	if !modeListContains(availableModes, role) {
		slog.Warn("gateway: opencode agent block not found, degrading",
			"project_id", projectID,
			"session_id", sessionID,
			"role", role,
			"available_modes", availableModes)
		g.recordConfigMismatch(projectID, "missing_block:"+requestedAgentType)
		if len(availableModes) > 0 {
			resolvedRole = availableModes[0]
		} else {
			resolvedRole = ""
		}
	}

	if _, err := client.SetSessionConfigOption(ctx, sessionID, "mode", resolvedRole); err != nil {
		return fmt.Errorf("set session mode %q: %w", resolvedRole, err)
	}
	slog.Info("gateway: session mode set",
		"project_id", projectID,
		"session_id", sessionID,
		"mode", resolvedRole,
		"requested_role", role)
	return nil
}

// modeListContains reports whether the given mode id is present in the
// available modes slice.
func modeListContains(modes []string, want string) bool {
	for _, m := range modes {
		if m == want {
			return true
		}
	}
	return false
}

// modeRoutingClient is the narrow ACP client surface required by
// routeSessionMode. Extracting this minimal interface (instead of depending
// on the concrete *acp.Client) lets in-package tests (gateway_test.go,
// TASK-011) substitute a recording mock without standing up a real ACP
// subprocess via in-memory pipes. The concrete *acp.Client satisfies this
// interface in production.
type modeRoutingClient interface {
	SetSessionConfigOption(ctx context.Context, sessionID, configID, value string) (*acp.SetSessionConfigOptionResponse, error)
}
