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
	// Load configuration from environment variables and config file
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create MCP server
	mcpServer, err := mcp.NewServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	// Create session manager
	sessionMgr := session.NewManager(mcpServer, cfg.Server.BaseURL, cfg)

	// Create Slack handler
	slackHandler := slack.NewHandler(cfg.Slack.BotToken, cfg.Slack.SigningSecret, sessionMgr)

	// Set assistant display options
	slackHandler.SetAssistantOptions(cfg.Slack.Assistant.Username, cfg.Slack.Assistant.IconEmoji, cfg.Slack.Assistant.IconURL)

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
