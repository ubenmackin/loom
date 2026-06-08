package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ubenmackin/loom/internal/acp"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// ---------------------------------------------------------------------------
// Store interfaces — minimal subsets of the store interfaces used by the
// gateway. These are intentionally narrower than the full-store interfaces
// in internal/api and internal/store so that the gateway only depends on
// what it actually needs.
// ---------------------------------------------------------------------------

// TaskStore defines the gateway's task storage requirements.
type TaskStore interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
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
}

// CommentStore defines the gateway's comment storage requirements.
type CommentStore interface {
	Create(ctx context.Context, c *models.Comment) error
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

// TriggerRuleStore defines the gateway's trigger rule storage requirements.
type TriggerRuleStore interface {
	List(ctx context.Context) ([]*models.TriggerRule, error)
}

// ---------------------------------------------------------------------------
// Gateway is the main orchestrator that manages ACP agent sessions and
// proactively pushes work to opencode serve sessions. It runs as a background
// goroutine with an event-driven loop.
// ---------------------------------------------------------------------------

// Gateway is the main orchestrator that manages ACP agent sessions and
// proactively pushes work to opencode serve sessions.
type Gateway struct {
	dispatcher *dispatcher.Dispatcher
	tracker    *SessionTracker
	rules      *RulesEngine
	queue      *JobQueue
	acpClients map[string]*acp.Client // key: "projectID:agentType"

	taskStore     TaskStore
	sessionStore  SessionStore
	projectStore  ProjectStore
	storyStore    StoryStore
	commentStore  CommentStore
	activityStore ActivityStore
	profileStore  AgentProfileStore
	ruleStore     TriggerRuleStore

	mu      sync.RWMutex
	eventCh chan dispatcher.Event
	done    chan struct{}
	wg      sync.WaitGroup
	started atomic.Bool
	stopped atomic.Bool

	eventsProcessed atomic.Int64
	startedAt       time.Time

	acpBaseURL string // e.g., "ws://localhost:8765"
}

// NewGateway creates a new Gateway with the given dependencies. The acpBaseURL
// configures where opencode serve is running (e.g. "ws://localhost:8765"). The
// gateway does not start processing events until Start() is called.
func NewGateway(
	d *dispatcher.Dispatcher,
	acpBaseURL string,
	taskStore TaskStore,
	sessionStore SessionStore,
	projectStore ProjectStore,
	storyStore StoryStore,
	commentStore CommentStore,
	activityStore ActivityStore,
	profileStore AgentProfileStore,
	ruleStore TriggerRuleStore,
) *Gateway {
	return &Gateway{
		dispatcher:    d,
		tracker:       NewSessionTracker(),
		rules:         NewRulesEngine(),
		queue:         NewJobQueue(),
		acpClients:    make(map[string]*acp.Client),
		taskStore:     taskStore,
		sessionStore:  sessionStore,
		projectStore:  projectStore,
		storyStore:    storyStore,
		commentStore:  commentStore,
		activityStore: activityStore,
		profileStore:  profileStore,
		ruleStore:     ruleStore,
		eventCh:       make(chan dispatcher.Event, 256),
		done:          make(chan struct{}),
		acpBaseURL:    acpBaseURL,
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
	if err := g.loadRules(ctx); err != nil {
		slog.Error("gateway: failed to load rules", "error", err)
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.run()
	}()
	slog.Info("gateway started", "acp_base_url", g.acpBaseURL)
}

// Stop signals the gateway to shut down gracefully. It is idempotent:
// subsequent calls are no-ops. Stop waits for the event loop to finish.
func (g *Gateway) Stop() {
	if g.stopped.Swap(true) {
		return
	}
	close(g.done)
	g.wg.Wait()

	// Close all ACP client connections.
	g.mu.Lock()
	for key, client := range g.acpClients {
		if err := client.Close(); err != nil {
			slog.Warn("gateway: error closing acp client",
				"key", key, "error", err)
		}
	}
	g.mu.Unlock()

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

// getOrCreateACPClient returns an existing ACP client for the given
// (projectID, agentType) pair, or creates a new one by dialing the
// ACP WebSocket endpoint. This is a best-effort operation — if the
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

	// Need to create a new client.
	url := fmt.Sprintf("%s/acp", g.acpBaseURL)
	newClient := acp.NewClient(url)

	if err := newClient.Connect(ctx); err != nil {
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
		"project_id", projectID, "agent_type", agentType, "url", url)

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
		g.handleACPMessage(ctx, msg, projectID, agentType)
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

// logActivity is a helper that logs an activity entry and logs any error.
func (g *Gateway) logActivity(ctx context.Context, workItemID, workItemType, action, details string) {
	entry := &models.ActivityLogEntry{
		WorkItemID:   workItemID,
		WorkItemType: models.WorkItemType(workItemType),
		Action:       action,
		Details:      details,
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

// ReloadRules reloads all trigger rules from the database and updates the
// rules engine. This allows rule changes made via the REST API to take
// effect without a server restart.
func (g *Gateway) ReloadRules(ctx context.Context) error {
	return g.loadRules(ctx)
}

// loadProfiles loads all agent profiles from the database and configures the
// gateway's concurrency limits accordingly.
func (g *Gateway) loadProfiles(ctx context.Context) error {
	profiles, err := g.profileStore.List(ctx)
	if err != nil {
		return fmt.Errorf("load agent profiles: %w", err)
	}

	for _, p := range profiles {
		g.queue.SetConcurrency(p.Name, p.MaxConcurrency)
		slog.Info("gateway: configured concurrency from profile",
			"agent_type", p.Name,
			"max_concurrency", p.MaxConcurrency)
	}
	return nil
}

// loadRules loads all trigger rules from the database and replaces the
// rules engine's ruleset with the database-backed rules.
func (g *Gateway) loadRules(ctx context.Context) error {
	rules, err := g.ruleStore.List(ctx)
	if err != nil {
		return fmt.Errorf("load trigger rules: %w", err)
	}

	// If no rules exist in DB, keep the default rules.
	if len(rules) == 0 {
		slog.Info("gateway: no trigger rules found in database, using defaults")
		return nil
	}

	var gatewayRules []Rule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		// Resolve the profile name to get the agent type.
		profile, err := g.profileStore.GetByID(ctx, r.AgentProfileID)
		if err != nil {
			slog.Warn("gateway: skipping rule for unknown profile",
				"rule_id", r.ID, "profile_id", r.AgentProfileID)
			continue
		}

		gatewayRules = append(gatewayRules, Rule{
			EventType: r.EventType,
			AgentType: profile.Name,
			Action:    ActionType(r.Action),
		})
	}

	g.rules.SetRules(gatewayRules)
	slog.Info("gateway: loaded trigger rules from database",
		"count", len(gatewayRules))
	return nil
}
