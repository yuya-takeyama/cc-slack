package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/repository"
	"github.com/yuya-takeyama/cc-slack/internal/router"
	"github.com/yuya-takeyama/cc-slack/internal/session"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
	"github.com/yuya-takeyama/cc-slack/internal/web"
	"github.com/yuya-takeyama/cc-slack/internal/worktree"
)

func main() {
	// Load configuration from environment variables and config file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Open database connection
	sqlDB, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer sqlDB.Close()

	// Run database migrations
	if err := database.Migrate(sqlDB, cfg.Database.MigrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create MCP server
	mcpServer, err := mcp.NewServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	// We need to create the slack handler and session manager in two steps
	// First create a placeholder handler
	slackHandler := &slack.Handler{}

	// Create session manager with database support
	sessionMgr := session.NewManager(sqlDB, cfg, slackHandler, cfg.Server.BaseURL)

	// Create repository manager
	repoManager := repository.NewManager(sqlDB)

	// Create logger for worktree manager
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create worktree manager
	worktreeMgr := worktree.NewManager(logger, cfg, sqlDB)

	// Set managers in session manager
	sessionMgr.SetRepositoryManager(repoManager)
	sessionMgr.SetWorktreeManager(worktreeMgr)

	// Start worktree cleanup routine
	ctx := context.Background()
	worktreeMgr.Start(ctx)

	// Now create the actual Slack handler with the session manager
	*slackHandler = *slack.NewHandler(cfg.Slack.BotToken, cfg.Slack.SigningSecret, sessionMgr)

	// Set assistant display options
	slackHandler.SetAssistantOptions(cfg.Slack.Assistant.Username, cfg.Slack.Assistant.IconEmoji, cfg.Slack.Assistant.IconURL)

	// Create channel cache for web API
	slackClient := slackHandler.GetClient()
	channelCache := slack.NewChannelCache(slackClient, 1*time.Hour)

	// Set Slack integration in MCP server
	mcpServer.SetSlackIntegration(slackHandler, sessionMgr)

	// Set MCP server as approval responder in Slack handler
	slackHandler.SetApprovalResponder(mcpServer)

	// Check if we need to set up repository router for Slack handler
	if len(cfg.Repositories) > 0 {
		// Convert config repositories to db repositories for router
		var dbRepos []db.Repository
		for _, repo := range cfg.Repositories {
			dbRepos = append(dbRepos, db.Repository{
				ID:   int64(len(dbRepos) + 1), // Temporary ID
				Name: repo.Name,
				Path: repo.Path,
			})
		}
		repoRouter := router.NewRepositoryRouter(logger, cfg, dbRepos)

		// Set managers in Slack handler
		slackHandler.SetRepositoryManager(repoManager)
		slackHandler.SetWorktreeManager(worktreeMgr)
		slackHandler.SetRepositoryRouter(repoRouter)
	}

	// Create HTTP router
	router := mux.NewRouter()

	// Slack endpoints
	router.HandleFunc("/slack/events", slackHandler.HandleEvent).Methods(http.MethodPost)
	router.HandleFunc("/slack/interactive", slackHandler.HandleInteraction).Methods(http.MethodPost)

	// MCP endpoints
	router.PathPrefix("/mcp").HandlerFunc(mcpServer.Handle)

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}).Methods(http.MethodGet)

	// Manager proxy endpoints
	router.HandleFunc("/api/manager/{path:.*}", func(w http.ResponseWriter, r *http.Request) {
		// Extract the path after /api/manager/
		vars := mux.Vars(r)
		path := vars["path"]

		// Create proxy request to cc-slack-manager
		proxyURL := fmt.Sprintf("http://localhost:10080/%s", path)
		proxyReq, err := http.NewRequest(r.Method, proxyURL, r.Body)
		if err != nil {
			http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// Make the request
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "Failed to proxy request", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Copy status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Printf("Error copying response body: %v", err)
		}
	}).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	// Web console endpoints (must be last due to catch-all route)
	webHandler, err := web.NewHandler()
	if err != nil {
		// Ignore error if web/dist doesn't exist yet
		log.Printf("Web console not available: %v", err)
	} else {
		// Set database connection and channel cache for web package
		web.SetDatabase(sqlDB)
		web.SetChannelCache(channelCache)
		router.PathPrefix("/").Handler(webHandler)
	}

	// Create HTTP server
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(cfg.Session.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			sessionMgr.CleanupIdleSessions(cfg.Session.Timeout)
		}
	}()

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	log.Printf("Server starting on port %d", cfg.Server.Port)
	log.Printf("MCP endpoint: %s/mcp", cfg.Server.BaseURL)
	log.Printf("Slack webhook endpoint: %s/slack/events", cfg.Server.BaseURL)
	log.Printf("Session timeout: %v", cfg.Session.Timeout)
	log.Printf("Cleanup interval: %v", cfg.Session.CleanupInterval)
	log.Printf("Resume window: %v", cfg.Session.ResumeWindow)
	log.Printf("Database path: %s", cfg.Database.Path)
	if cfg.Slack.Assistant.Username != "" || cfg.Slack.Assistant.IconEmoji != "" || cfg.Slack.Assistant.IconURL != "" {
		log.Printf("Assistant display options: username=%s, icon_emoji=%s, icon_url=%s",
			cfg.Slack.Assistant.Username, cfg.Slack.Assistant.IconEmoji, cfg.Slack.Assistant.IconURL)
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %d: %v\n", cfg.Server.Port, err)
	}

	<-done
	log.Println("Server stopped")
}
