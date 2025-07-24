package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
	"github.com/yuya-takeyama/cc-slack/pkg/config"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	// Create shared components
	slackBot := slack.NewBot(cfg)
	mcpServer := mcp.NewServer(cfg)

	// Set up cross-component dependencies
	mcpServer.SetApprovalManager(slackBot.GetApprovalManager())

	// Start Slack Bot HTTP Server (background goroutine)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := slackBot.Start(ctx); err != nil {
			slog.Error("Slack bot failed", "error", err)
		}
	}()

	// Start MCP Server (main goroutine)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := mcpServer.Run(ctx); err != nil {
			slog.Error("MCP server failed", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		slog.Info("Received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
		slog.Info("Context cancelled, shutning down")
	}

	wg.Wait()
	slog.Info("Shutdown complete")
}