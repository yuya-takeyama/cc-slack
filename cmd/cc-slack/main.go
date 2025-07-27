package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/session"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

func main() {
	// Load configuration from environment variables
	config := loadConfig()

	// Create MCP server
	mcpServer, err := mcp.NewServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	// Create session manager
	sessionMgr := session.NewManager(mcpServer, config.BaseURL)

	// Create Slack handler
	slackHandler := slack.NewHandler(config.SlackToken, config.SlackSigningSecret, sessionMgr)

	// Set assistant display options
	slackHandler.SetAssistantOptions(config.AssistantUsername, config.AssistantIconEmoji, config.AssistantIconURL)

	// Set Slack handler in session manager
	sessionMgr.SetSlackHandler(slackHandler)

	// Set Slack integration in MCP server
	mcpServer.SetSlackIntegration(slackHandler, sessionMgr)

	// Set MCP server as approval responder in Slack handler
	slackHandler.SetApprovalResponder(mcpServer)

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

	// Create HTTP server
	srv := &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf(":%s", config.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(config.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			sessionMgr.CleanupIdleSessions(config.SessionTimeout)
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

	log.Printf("Server starting on port %s", config.Port)
	log.Printf("MCP endpoint: %s/mcp", config.BaseURL)
	log.Printf("Slack webhook endpoint: %s/slack/events", config.BaseURL)
	log.Printf("Session timeout: %v", config.SessionTimeout)
	log.Printf("Cleanup interval: %v", config.CleanupInterval)
	if config.AssistantUsername != "" || config.AssistantIconEmoji != "" || config.AssistantIconURL != "" {
		log.Printf("Assistant display options: username=%s, icon_emoji=%s, icon_url=%s",
			config.AssistantUsername, config.AssistantIconEmoji, config.AssistantIconURL)
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", config.Port, err)
	}

	<-done
	log.Println("Server stopped")
}

// Config holds the application configuration
type Config struct {
	Port               string
	SlackToken         string
	SlackSigningSecret string
	BaseURL            string
	SessionTimeout     time.Duration
	CleanupInterval    time.Duration
	AssistantUsername  string
	AssistantIconEmoji string
	AssistantIconURL   string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	config := &Config{
		Port:               config.GetEnv("CC_SLACK_PORT", "8080"),
		SlackToken:         config.GetEnv("CC_SLACK_SLACK_BOT_TOKEN", ""),
		SlackSigningSecret: config.GetEnv("CC_SLACK_SLACK_SIGNING_SECRET", ""),
		BaseURL:            config.GetEnv("CC_SLACK_BASE_URL", "http://localhost:8080"),
		SessionTimeout:     config.GetDurationEnv("CC_SLACK_SESSION_TIMEOUT", 30*time.Minute),
		CleanupInterval:    config.GetDurationEnv("CC_SLACK_CLEANUP_INTERVAL", 5*time.Minute),
		AssistantUsername:  config.GetEnv("CC_SLACK_ASSISTANT_USERNAME", ""),
		AssistantIconEmoji: config.GetEnv("CC_SLACK_ASSISTANT_ICON_EMOJI", ""),
		AssistantIconURL:   config.GetEnv("CC_SLACK_ASSISTANT_ICON_URL", ""),
	}

	// Validate required fields
	if config.SlackToken == "" {
		log.Fatal("CC_SLACK_SLACK_BOT_TOKEN is required")
	}
	if config.SlackSigningSecret == "" {
		log.Fatal("CC_SLACK_SLACK_SIGNING_SECRET is required")
	}

	return config
}
