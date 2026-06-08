package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/gateway"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// HubInterface defines the minimal interface the API needs from the WebSocket hub.
type HubInterface interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// StoryStore defines the interface for interacting with the stories storage.
type StoryStore interface {
	Create(ctx context.Context, story *models.Story) error
	GetByID(ctx context.Context, id string) (*models.Story, error)
	GetByNumericID(ctx context.Context, numID int) (*models.Story, error)
	List(ctx context.Context, filter store.StoryFilter) ([]*models.Story, error)
	Update(ctx context.Context, story *models.Story) error
	BatchUpdate(ctx context.Context, stories []*models.Story) error
	UpdateStatus(ctx context.Context, id string, status models.Status) error
	Delete(ctx context.Context, id string) error
	GetWithTasks(ctx context.Context, id string) (*models.Story, []*models.Task, error)
}

// ProjectStore defines the interface for interacting with the projects storage.
type ProjectStore interface {
	Create(ctx context.Context, project *models.Project) error
	GetByID(ctx context.Context, id string) (*models.Project, error)
	List(ctx context.Context) ([]*models.Project, error)
	Update(ctx context.Context, project *models.Project) error
	Delete(ctx context.Context, id string) error
}

// TaskStore defines the interface for interacting with the tasks storage.
type TaskStore interface {
	Create(ctx context.Context, t *models.Task) error
	GetByID(ctx context.Context, id string) (*models.Task, error)
	GetByNumericID(ctx context.Context, numID int) (*models.Task, error)
	List(ctx context.Context, filter store.TaskFilter) ([]*models.Task, error)
	Update(ctx context.Context, t *models.Task) error
	BatchUpdate(ctx context.Context, tasks []*models.Task) error
	UpdateStatus(ctx context.Context, id string, status models.Status) error
	AddDependency(ctx context.Context, taskID, dependsOnID string) error
	RemoveDependency(ctx context.Context, taskID, dependsOnID string) error
	GetDependencies(ctx context.Context, taskID string) ([]string, error)
	GetBlockers(ctx context.Context, taskID string) ([]*models.Task, error)
	GetBlockersForTasks(ctx context.Context, taskIDs []string) (map[string][]string, error)
	GetByStory(ctx context.Context, storyID string) ([]*models.Task, error)
	DetectCycle(ctx context.Context, taskID, dependsOnID string) (bool, error)
	GetDependents(ctx context.Context, taskID string) ([]*models.Task, error)
	Transact(ctx context.Context, fn func(context.Context) error) error
	Delete(ctx context.Context, id string) error
}

// SessionStore defines the interface for interacting with the agent sessions storage.
type SessionStore interface {
	Register(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id string) (*models.Session, error)
	UpdateLastSeen(ctx context.Context, id string) error
	Disconnect(ctx context.Context, id string) error
	GetTasksForSession(ctx context.Context, sessionID string) ([]*models.Task, error)
	ListAll(ctx context.Context) ([]*models.Session, error)
	ListActive(ctx context.Context) ([]*models.Session, error)
	GetByCapabilitiesWithTaskCount(ctx context.Context, capability string) ([]store.SessionWithTaskCount, error)
}

// CommentStore defines the interface for interacting with the comments storage.
type CommentStore interface {
	Create(ctx context.Context, c *models.Comment) error
	GetByID(ctx context.Context, id string) (*models.Comment, error)
	GetByWorkItem(ctx context.Context, workItemID string, workItemType models.WorkItemType) ([]*models.Comment, error)
	Update(ctx context.Context, c *models.Comment) error
	Delete(ctx context.Context, id, authorID string) error
	MarkAsRead(ctx context.Context, sessionID, commentID string) error
	GetUnreadForSession(ctx context.Context, sessionID string) ([]*models.Comment, error)
	GetUnreadForSessionByWorkItem(ctx context.Context, sessionID, workItemID string, workItemType models.WorkItemType) ([]*models.Comment, error)
}

// TemplateStore defines the interface for interacting with the prompt templates storage.
type TemplateStore interface {
	GetByTaskType(ctx context.Context, taskType models.TaskType) (*models.PromptTemplate, error)
	Upsert(ctx context.Context, t *models.PromptTemplate) error
	List(ctx context.Context) ([]*models.PromptTemplate, error)
	Delete(ctx context.Context, id string) error
}

// ActivityStore defines the interface for interacting with the activity log storage.
type ActivityStore interface {
	Log(ctx context.Context, entry *models.ActivityLogEntry) error
	GetRecent(ctx context.Context, limit int) ([]*models.ActivityLogEntry, error)
	GetByWorkItem(ctx context.Context, workItemID string, workItemType models.WorkItemType, limit, offset int) ([]*models.ActivityLogEntry, error)
	GetByAction(ctx context.Context, limit, offset int, actionPrefixes ...string) ([]*models.ActivityLogEntry, error)
}

// UserStore defines the interface for interacting with the users and user sessions storage.
type UserStore interface {
	CreateUser(ctx context.Context, username, email, displayName, password string, role models.UserRole) (*models.User, error)
	AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (*models.User, error)
	CreateSession(ctx context.Context, userID string) (string, error)
	GetUserBySessionToken(ctx context.Context, token string) (*models.User, error)
	DeleteSession(ctx context.Context, token string) error
	CountUsers(ctx context.Context) (int, error)
	CleanupExpiredSessions(ctx context.Context) error
	ListAll(ctx context.Context) ([]*models.User, error)
	DeleteUser(ctx context.Context, id string) error
}

// AgentProfileStore defines the interface for interacting with agent profiles.
type AgentProfileStore interface {
	Create(ctx context.Context, profile *models.AgentProfile) error
	GetByID(ctx context.Context, id string) (*models.AgentProfile, error)
	List(ctx context.Context) ([]*models.AgentProfile, error)
	Update(ctx context.Context, profile *models.AgentProfile) error
	Delete(ctx context.Context, id string) error
}

// TriggerRuleStore defines the interface for interacting with trigger rules.
type TriggerRuleStore interface {
	Create(ctx context.Context, rule *models.TriggerRule) error
	GetByID(ctx context.Context, id string) (*models.TriggerRule, error)
	ListByProfile(ctx context.Context, profileID string) ([]*models.TriggerRule, error)
	List(ctx context.Context) ([]*models.TriggerRule, error)
	Update(ctx context.Context, rule *models.TriggerRule) error
	Delete(ctx context.Context, id string) error
}

// handlers holds all store references and dependencies for the API handlers.
type handlers struct {
	stories   StoryStore
	tasks     TaskStore
	projects  ProjectStore
	sessions  SessionStore
	comments  CommentStore
	templates TemplateStore
	activity  ActivityStore
	users     UserStore
	profiles  AgentProfileStore
	rules     TriggerRuleStore
	dispatch  *dispatcher.Dispatcher
	gateway   *gateway.Gateway
	hub       HubInterface
}

// NewRouter creates and configures the chi router with all API routes.
func NewRouter(
	storyStore StoryStore,
	taskStore TaskStore,
	projectStore ProjectStore,
	sessionStore SessionStore,
	commentStore CommentStore,
	templateStore TemplateStore,
	activityStore ActivityStore,
	userStore UserStore,
	profileStore AgentProfileStore,
	ruleStore TriggerRuleStore,
	d *dispatcher.Dispatcher,
	gw *gateway.Gateway,
	hub HubInterface,
) *chi.Mux {
	h := &handlers{
		stories:   storyStore,
		tasks:     taskStore,
		projects:  projectStore,
		sessions:  sessionStore,
		comments:  commentStore,
		templates: templateStore,
		activity:  activityStore,
		users:     userStore,
		profiles:  profileStore,
		rules:     ruleStore,
		dispatch:  d,
		gateway:   gw,
		hub:       hub,
	}

	r := chi.NewRouter()

	// Global middleware.
	r.Use(CORS)
	r.Use(Logger)
	r.Use(Recovery)
	r.Use(SessionExtractor)

	// WebSocket endpoint for real-time board events.
	r.Get("/ws", h.hub.ServeHTTP)

	// Auth routes (public & onboarding)
	r.Route("/auth", h.registerAuthRoutes)

	// Agent-specific endpoints (require session authentication or shared agent secret)
	r.Group(func(r chi.Router) {
		r.Use(h.SessionAuthenticator)

		r.Route("/sessions", h.registerSessionRoutes)
		r.Route("/work", h.registerWorkRoutes)
	})

	// Protected human-centric board management and query endpoints
	r.Group(func(r chi.Router) {
		r.Use(h.UserAuthenticator)

		r.Route("/stories", h.registerStoryRoutes)
		r.Route("/tasks", h.registerTaskRoutes)
		r.Route("/work-items", h.registerCommentRoutes)
		r.Route("/templates", h.registerTemplateRoutes)

		// Story sub-resource: tasks under a story.
		r.Post("/stories/{id}/tasks", h.createTaskUnderStory)

		// Board state endpoint.
		r.Get("/board", h.GetBoard)

		// Sessions list for human users (Agents page).
		r.Get("/sessions", h.listSessions)

		// Global activity log endpoint.
		r.Route("/activity", h.registerActivityRoutes)

		// Dispatcher status endpoint.
		r.Get("/dispatcher/status", h.handleDispatcherStatus)

		// Project list for project picker (all logged-in users).
		r.Get("/projects", h.listProjects)
	})

	// Admin-only user management and project management endpoints
	r.Group(func(r chi.Router) {
		r.Use(h.UserAuthenticator)
		r.Use(h.AdminOnly)
		r.Route("/users", h.registerUserRoutes)
		r.Post("/projects", h.createProject)
		r.Get("/projects/{id}", h.getProject)
		r.Put("/projects/{id}", h.updateProject)
		r.Delete("/projects/{id}", h.deleteProject)

		// Gateway admin endpoints.
		r.Get("/gateway/status", h.handleGatewayStatus)
		r.Get("/gateway/queue", h.handleGatewayQueue)
		r.Post("/gateway/trigger", h.handleGatewayTrigger)

		// Agent profile management.
		r.Route("/profiles", h.registerProfileRoutes)
	})

	return r
}

// registerProfileRoutes registers CRUD routes for agent profiles.
func (h *handlers) registerProfileRoutes(r chi.Router) {
	r.Get("/", h.listProfiles)
	r.Post("/", h.createProfile)
	r.Get("/{id}", h.getProfile)
	r.Put("/{id}", h.updateProfile)
	r.Delete("/{id}", h.deleteProfile)

	// Sub-resource: trigger rules for a profile.
	r.Route("/{id}/rules", func(r chi.Router) {
		r.Get("/", h.listRulesByProfile)
		r.Post("/", h.createRule)
		r.Put("/{ruleID}", h.updateRule) // ruleID path param
		r.Delete("/{ruleID}", h.deleteRule)
	})
}
