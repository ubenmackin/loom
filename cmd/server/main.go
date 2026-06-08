// Package main is the entry point for the Loom server.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/api"
	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/gateway"
	"github.com/ubenmackin/loom/internal/mcp"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/ws"
)

// serverConfig holds the parsed command-line configuration.
type serverConfig struct {
	dbPath string
	port   int
	webDir string
	runMCP bool
}

// Stores holds all database-backed store instances.
type Stores struct {
	Story    *store.StoryStore
	Task     *store.TaskStore
	Project  *store.ProjectStore
	Session  *store.SessionStore
	Comment  *store.CommentStore
	Template *store.TemplateStore
	Activity *store.ActivityStore
	User     *store.UserStore
	Profile  *store.AgentProfileStore
	Rule     *store.TriggerRuleStore
}

// NewStores creates all store instances from the given database connection.
func NewStores(db *sql.DB) *Stores {
	return &Stores{
		Story:    store.NewStoryStore(db),
		Task:     store.NewTaskStore(db),
		Project:  store.NewProjectStore(db),
		Session:  store.NewSessionStore(db),
		Comment:  store.NewCommentStore(db),
		Template: store.NewTemplateStore(db),
		Activity: store.NewActivityStore(db),
		User:     store.NewUserStore(db),
		Profile:  store.NewAgentProfileStore(db),
		Rule:     store.NewTriggerRuleStore(db),
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := parseFlags()

	database, err := db.Open(cfg.dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := db.Migrate(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	ctx := context.Background()
	stores := NewStores(database)

	if err := db.SeedDefaultProjects(ctx, stores.Project); err != nil {
		return fmt.Errorf("seed default projects: %w", err)
	}

	if err := db.SeedDefaults(ctx, stores.Template); err != nil {
		return fmt.Errorf("seed default templates: %w", err)
	}

	if err := db.SeedDefaultAgentProfiles(ctx, stores.Profile); err != nil {
		return fmt.Errorf("seed default agent profiles: %w", err)
	}

	if err := db.BackfillNumericIDs(database); err != nil {
		return fmt.Errorf("backfill numeric IDs: %w", err)
	}

	if cfg.runMCP {
		return runMCP(cfg, database, stores)
	}
	return runHTTP(cfg, database, stores)
}

// parseFlags reads and returns the command-line configuration.
func parseFlags() serverConfig {
	dbPath := flag.String("db-path", "loom.db", "path to SQLite database file")
	port := flag.Int("port", 8080, "HTTP server port")
	webDir := flag.String("web-dir", "web/dist", "path to frontend static files")
	runMCP := flag.Bool("mcp", false, "run as MCP server on stdio instead of HTTP")
	flag.Parse()

	return serverConfig{
		dbPath: *dbPath,
		port:   *port,
		webDir: *webDir,
		runMCP: *runMCP,
	}
}

// runMCP starts the MCP server on stdio.
func runMCP(cfg serverConfig, database *sql.DB, stores *Stores) error {
	// Initialize dispatcher with a no-op broadcaster (MCP mode has no WebSocket clients).
	noOpHub := &noopBroadcaster{}
	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{
		StoryStore:    stores.Story,
		TaskStore:     stores.Task,
		SessionStore:  stores.Session,
		TemplateStore: stores.Template,
		CommentStore:  stores.Comment,
		ActivityStore: stores.Activity,
		Broadcaster:   noOpHub,
	})
	d.Start()
	defer d.Stop()

	mcpServer := mcp.NewServer(
		stores.Story, stores.Task, stores.Session, stores.Comment,
		stores.Template, stores.Activity, d,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("MCP server shutting down...")
		cancel()
	}()

	if err := mcpServer.Run(ctx); err != nil {
		return fmt.Errorf("MCP server: %w", err)
	}

	log.Println("MCP server stopped")
	return nil
}

// runHTTP starts the HTTP API server with WebSocket support.
func runHTTP(cfg serverConfig, database *sql.DB, stores *Stores) error {
	// Initialize dispatcher with the WebSocket hub as the event broadcaster.
	hub := ws.NewHub()
	go hub.Start()
	defer hub.Stop()

	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{
		StoryStore:    stores.Story,
		TaskStore:     stores.Task,
		SessionStore:  stores.Session,
		TemplateStore: stores.Template,
		CommentStore:  stores.Comment,
		ActivityStore: stores.Activity,
		Broadcaster:   hub,
	})
	d.Start()
	defer d.Stop()

	// Create and start the gateway engine (ACP/WebSocket push orchestration).
	// The ACP base URL defaults to ws://localhost:8765. In Phase 3 this
	// will become configurable.
	gw := gateway.NewGateway(
		d,
		"ws://localhost:8765",
		stores.Task,
		stores.Session,
		stores.Project,
		stores.Story,
		stores.Comment,
		stores.Activity,
		stores.Profile,
		stores.Rule,
	)
	gw.Start()
	defer gw.Stop()

	// Create a lifecycle context for background tasks.
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	// Start periodic session cleanup.
	runSessionCleanup(serverCtx, stores.User)

	// Create the API router.
	apiRouter := api.NewRouter(
		stores.Story, stores.Task, stores.Project, stores.Session, stores.Comment,
		stores.Template, stores.Activity, stores.User, stores.Profile, stores.Rule,
		d, gw, hub,
	)

	// Set up the top-level HTTP router.
	r := chi.NewRouter()

	// Health check endpoints.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	// Mount API routes.
	r.Mount("/api", apiRouter)

	// Serve static frontend files with SPA fallback.
	r.Handle("/*", spaHandler(cfg.webDir))

	// Start HTTP server.
	addr := fmt.Sprintf(":%d", cfg.port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Loom server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	go func() {
		<-done
		log.Println("Shutting down server...")
		serverCancel()
	}()

	<-serverCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// runSessionCleanup runs an initial session cleanup and then periodically
// cleans up expired user sessions on the given interval.
func runSessionCleanup(ctx context.Context, userStore *store.UserStore) {
	// Run once at startup.
	if err := userStore.CleanupExpiredSessions(ctx); err != nil {
		log.Printf("Failed to cleanup expired sessions at startup: %v", err)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := userStore.CleanupExpiredSessions(ctx); err != nil {
					log.Printf("Failed to cleanup expired sessions: %v", err)
				}
			}
		}
	}()
}

// spaHandler serves static files from webDir and falls back to index.html
// for any path that doesn't match a file (SPA client-side routing).
// It uses os.Stat to check file existence (single open) before serving.
func spaHandler(webDir string) http.Handler {
	fs := http.FileServer(http.Dir(webDir))
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		log.Fatalf("failed to resolve web directory: %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash so the path is relative to webDir.
		relPath := strings.TrimPrefix(r.URL.Path, "/")

		// Use os.Stat to check file existence — single filesystem lookup.
		fullPath := filepath.Join(absWebDir, filepath.FromSlash(relPath))
		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() {
			// Verify the resolved path is still within webDir (symlink traversal protection).
			realPath, err := filepath.EvalSymlinks(fullPath)
			if err != nil || !strings.HasPrefix(realPath+string(os.PathSeparator), absWebDir+string(os.PathSeparator)) {
				http.NotFound(w, r)
				return
			}
			// Reset URL path so http.FileServer can serve correctly.
			r.URL.Path = "/" + relPath
			fs.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA client-side routing.
		http.ServeFile(w, r, filepath.Join(absWebDir, "index.html"))
	})
}

// noopBroadcaster is a no-op implementation of dispatcher.EventBroadcaster
// used in MCP mode where there are no WebSocket clients.
type noopBroadcaster struct{}

func (n *noopBroadcaster) Broadcast(eventType string, payload any) {
	// No-op — MCP mode has no WebSocket clients.
}
