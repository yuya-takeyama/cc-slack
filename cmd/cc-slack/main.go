package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	slackapi "github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/session"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
	"github.com/yuya-takeyama/cc-slack/internal/web"
)

// stringSliceFlag implements flag.Value for string slice flags
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	// Support both multiple calls and comma-separated values
	if strings.Contains(value, ",") {
		*s = append(*s, strings.Split(value, ",")...)
	} else {
		*s = append(*s, value)
	}
	return nil
}

func main() {
	// Parse command-line flags
	var workingDirs stringSliceFlag
	flag.Var(&workingDirs, "working-dirs", "Working directories (can be specified multiple times)")
	flag.Parse()

	// Load configuration from environment variables and config file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set working directories from command-line if provided
	if len(workingDirs) > 0 {
		cfg.WorkingDirFlags = []string(workingDirs)
	}

	// Validate working directories
	if err := cfg.ValidateWorkingDirectories(); err != nil {
		log.Fatalf("Invalid working directory configuration: %v", err)
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

	// Authenticate with Slack API to get bot user ID
	slackClient := slackapi.New(cfg.Slack.BotToken)
	auth, err := slackClient.AuthTest()
	if err != nil {
		log.Fatalf("Failed to authenticate with Slack API: %v", err)
	}
	botUserID := auth.UserID
	log.Printf("Authenticated as bot user: %s", botUserID)

	// We need to create the slack handler and session manager in two steps
	// First create a placeholder handler
	slackHandler := &slack.Handler{}

	// Create session manager with database support
	sessionMgr := session.NewManager(sqlDB, cfg, slackHandler, cfg.Server.BaseURL, cfg.Slack.FileUpload.ImagesDir)

	// Now create the actual Slack handler with the session manager
	handler := slack.NewHandler(cfg, sessionMgr, botUserID)
	*slackHandler = *handler

	// Create channel cache for web API
	channelCache := slack.NewChannelCache(slackHandler.GetClient(), 1*time.Hour)

	// Set Slack integration in MCP server
	mcpServer.SetSlackIntegration(slackHandler, sessionMgr)

	// Set MCP server as approval responder in Slack handler
	slackHandler.SetApprovalResponder(mcpServer)

	// Check if Socket Mode is enabled
	useSocketMode := cfg.Slack.AppToken != ""

	// Create HTTP router
	router := mux.NewRouter()

	if useSocketMode {
		// Create Socket Mode handler
		socketHandler, err := slack.NewSocketModeHandler(cfg, slackHandler)
		if err != nil {
			log.Fatalf("Failed to create Socket Mode handler: %v", err)
		}

		// Start Socket Mode handler in background
		go func() {
			if err := socketHandler.Run(context.Background()); err != nil {
				log.Fatalf("Socket Mode handler error: %v", err)
			}
		}()

		log.Println("Socket Mode enabled - HTTP endpoints will not be used for Slack events")
	} else {
		// Slack endpoints (with 30-second timeout)
		router.HandleFunc("/slack/events", http.TimeoutHandler(
			http.HandlerFunc(slackHandler.HandleEvent), 30*time.Second, "Request timeout").ServeHTTP).Methods(http.MethodPost)
		router.HandleFunc("/slack/interactive", http.TimeoutHandler(
			http.HandlerFunc(slackHandler.HandleInteraction), 30*time.Second, "Request timeout").ServeHTTP).Methods(http.MethodPost)
		router.HandleFunc("/slack/commands", http.TimeoutHandler(
			http.HandlerFunc(slackHandler.HandleSlashCommand), 30*time.Second, "Request timeout").ServeHTTP).Methods(http.MethodPost)

		log.Println("Socket Mode disabled - using HTTP webhooks")
	}

	// MCP endpoints (no additional timeout, uses server timeout)
	router.PathPrefix("/mcp").HandlerFunc(mcpServer.Handle)

	// Health check (with 5-second timeout)
	router.HandleFunc("/health", http.TimeoutHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		}), 5*time.Second, "Request timeout").ServeHTTP).Methods(http.MethodGet)

	// Manager proxy endpoints (with 30-second timeout)
	managerProxyHandler := func(w http.ResponseWriter, r *http.Request) {
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
	}
	router.HandleFunc("/api/manager/{path:.*}", http.TimeoutHandler(
		http.HandlerFunc(managerProxyHandler), 30*time.Second, "Request timeout").ServeHTTP).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)

	// Web console endpoints (must be last due to catch-all route)
	webHandler, err := web.NewHandler()
	if err != nil {
		// Ignore error if web/dist doesn't exist yet
		log.Printf("Web console not available: %v", err)
	} else {
		// Set database connection and channel cache for web package
		web.SetDatabase(sqlDB)
		web.SetChannelCache(channelCache)
		// Web console with 30-second timeout
		router.PathPrefix("/").Handler(http.TimeoutHandler(webHandler, 30*time.Second, "Request timeout"))
	}

	// Create HTTP server
	// Set long timeout (1 hour) for MCP endpoints that handle approval prompts
	// Other endpoints have their own shorter timeouts via http.TimeoutHandler
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		WriteTimeout: 1 * time.Hour,
		ReadTimeout:  1 * time.Hour,
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
	if useSocketMode {
		log.Printf("Slack mode: Socket Mode (app_token configured)")
	} else {
		log.Printf("Slack mode: HTTP webhooks")
		log.Printf("Slack webhook endpoint: %s/slack/events", cfg.Server.BaseURL)
	}
	log.Printf("Session timeout: %v", cfg.Session.Timeout)
	log.Printf("Cleanup interval: %v", cfg.Session.CleanupInterval)
	log.Printf("Database path: %s", cfg.Database.Path)

	// Log working directory mode
	if len(cfg.WorkingDirFlags) > 0 {
		if len(cfg.WorkingDirFlags) == 1 {
			log.Printf("Working directory mode: Single (from command-line: %s)", cfg.WorkingDirFlags[0])
		} else {
			log.Printf("Working directory mode: Multi (from command-line: %d directories)", len(cfg.WorkingDirFlags))
			for i, dir := range cfg.WorkingDirFlags {
				log.Printf("  - [%d]: %s", i+1, dir)
			}
		}
	} else if len(cfg.WorkingDirs) > 0 {
		log.Printf("Working directory mode: Multi (%d configured)", len(cfg.WorkingDirs))
		for _, wd := range cfg.WorkingDirs {
			log.Printf("  - %s: %s", wd.Name, wd.Path)
		}
	} else {
		log.Printf("Working directory mode: None configured")
	}

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
