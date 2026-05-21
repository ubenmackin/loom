// Package main is the entry point for the Loom server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/api"
	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/mcp"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/ws"
)

func main() {
	dbPath := flag.String("db-path", "loom.db", "path to SQLite database file")
	port := flag.Int("port", 8080, "HTTP server port")
	webDir := flag.String("web-dir", "web/dist", "path to frontend static files")
	runMCP := flag.Bool("mcp", false, "run as MCP server on stdio instead of HTTP")
	flag.Parse()

	// Open database.
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Run migrations.
	if err := db.Migrate(database); err != nil {
		_ = database.Close()
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default prompt templates if none exist.
	if err := db.SeedDefaults(database); err != nil {
		_ = database.Close()
		log.Fatalf("Failed to seed default templates: %v", err)
	}

	// Backfill numeric IDs for any legacy work items.
	if err := db.BackfillNumericIDs(database); err != nil {
		_ = database.Close()
		log.Fatalf("Failed to backfill numeric IDs: %v", err)
	}

	if *runMCP {
		// Initialize stores for MCP server.
		storyStore := store.NewStoryStore(database)
		taskStore := store.NewTaskStore(database)
		sessionStore := store.NewSessionStore(database)
		commentStore := store.NewCommentStore(database)
		templateStore := store.NewTemplateStore(database)
		activityStore := store.NewActivityStore(database)

		// Initialize dispatcher with a no-op broadcaster (MCP mode has no WebSocket clients).
		noOpHub := &noOpBroadcaster{}
		d := dispatcher.NewDispatcher(
			storyStore, taskStore, sessionStore, templateStore,
			commentStore, activityStore, noOpHub, 0,
		)
		d.Start()

		// Create and run the MCP server on stdio.
		mcpServer := mcp.NewServer(
			storyStore, taskStore, sessionStore, commentStore,
			templateStore, activityStore, d,
		)

		ctx, cancel := context.WithCancel(context.Background())

		// Set up signal handling for graceful shutdown.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			log.Println("MCP server shutting down...")
			cancel()
		}()

		if err := mcpServer.Run(ctx); err != nil {
			d.Stop()
			cancel()
			_ = database.Close()
			log.Printf("MCP server error: %v", err)
			os.Exit(1)
		}

		d.Stop()
		cancel()
		_ = database.Close()
		return
	}

	defer func() { _ = database.Close() }()

	// Initialize stores.
	storyStore := store.NewStoryStore(database)
	taskStore := store.NewTaskStore(database)
	sessionStore := store.NewSessionStore(database)
	commentStore := store.NewCommentStore(database)
	templateStore := store.NewTemplateStore(database)
	activityStore := store.NewActivityStore(database)
	userStore := store.NewUserStore(database)

	// Initialize dispatcher with the WebSocket hub as the event broadcaster.
	hub := ws.NewHub()
	go hub.Start()
	defer hub.Stop()

	d := dispatcher.NewDispatcher(
		storyStore, taskStore, sessionStore, templateStore,
		commentStore, activityStore, hub, 0,
	)
	d.Start()
	defer d.Stop()

	// Periodically clean up expired user sessions.
	// Run once at startup, then every hour.
	if err := userStore.CleanupExpiredSessions(context.Background()); err != nil {
		log.Printf("Failed to cleanup expired sessions at startup: %v", err)
	}
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := userStore.CleanupExpiredSessions(context.Background()); err != nil {
				log.Printf("Failed to cleanup expired sessions: %v", err)
			}
		}
	}()

	// Create the API router.
	apiRouter := api.NewRouter(
		storyStore, taskStore, sessionStore, commentStore,
		templateStore, activityStore, userStore, d, hub,
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
	r.Handle("/*", spaHandler(*webDir))

	// Start HTTP server.
	addr := fmt.Sprintf(":%d", *port)
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
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// spaHandler serves static files from webDir and falls back to index.html
// for any path that doesn't match a file (SPA client-side routing).
func spaHandler(webDir string) http.Handler {
	fs := http.FileServer(http.Dir(webDir))
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		log.Fatalf("failed to resolve web directory: %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sanitize the URL path to prevent directory traversal.
		cleaned := path.Clean(r.URL.Path)
		if cleaned == "." {
			cleaned = "/"
		}
		r.URL.Path = cleaned

		// Try to serve the file directly.
		f, err := http.Dir(webDir).Open(r.URL.Path)
		if err == nil {
			if cerr := f.Close(); cerr != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			// Verify the resolved path is still within webDir.
			if realPath, err := filepath.EvalSymlinks(filepath.Join(absWebDir, filepath.FromSlash(r.URL.Path))); err == nil {
				if !strings.HasPrefix(realPath, absWebDir) {
					http.NotFound(w, r)
					return
				}
			}
			fs.ServeHTTP(w, r)
			return
		}
		// File not found — serve index.html for SPA routing.
		http.ServeFile(w, r, webDir+"/index.html")
	})
}

// noOpBroadcaster is a no-op implementation of dispatcher.EventBroadcaster
// used until the WebSocket hub (TASK-006) is available.
type noOpBroadcaster struct{}

func (n *noOpBroadcaster) Broadcast(eventType string, payload any) {
	// No-op — WebSocket hub not yet implemented.
}
