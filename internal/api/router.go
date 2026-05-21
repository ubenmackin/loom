package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/store"
)

// HubInterface defines the minimal interface the API needs from the WebSocket hub.
type HubInterface interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// handlers holds all store references and dependencies for the API handlers.
type handlers struct {
	stories   *store.StoryStore
	tasks     *store.TaskStore
	sessions  *store.SessionStore
	comments  *store.CommentStore
	templates *store.TemplateStore
	activity  *store.ActivityStore
	users     *store.UserStore
	dispatch  *dispatcher.Dispatcher
	hub       HubInterface
}

// NewRouter creates and configures the chi router with all API routes.
// The hub parameter may be nil (WebSocket support is TASK-006).
func NewRouter(
	storyStore *store.StoryStore,
	taskStore *store.TaskStore,
	sessionStore *store.SessionStore,
	commentStore *store.CommentStore,
	templateStore *store.TemplateStore,
	activityStore *store.ActivityStore,
	userStore *store.UserStore,
	d *dispatcher.Dispatcher,
	hub HubInterface,
) *chi.Mux {
	h := &handlers{
		stories:   storyStore,
		tasks:     taskStore,
		sessions:  sessionStore,
		comments:  commentStore,
		templates: templateStore,
		activity:  activityStore,
		users:     userStore,
		dispatch:  d,
		hub:       hub,
	}

	r := chi.NewRouter()

	// Global middleware.
	r.Use(CORS)
	r.Use(Logger)
	r.Use(Recovery)
	r.Use(SessionExtractor)

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
		r.Post("/stories/{storyId}/tasks", h.createTaskUnderStory)

		// Board state endpoint.
		r.Get("/board", h.GetBoard)

		// Global activity log endpoint.
		r.Route("/activity", h.registerActivityRoutes)
	})

	// WebSocket endpoint.
	if h.hub != nil {
		r.Get("/ws", h.hub.ServeHTTP)
	}

	return r
}
