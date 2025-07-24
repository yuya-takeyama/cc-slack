package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Slack configuration
	SlackBotToken     string
	SlackSigningSecret string
	
	// Server configuration
	Port           int
	DefaultWorkDir string
	
	// Claude Code configuration
	ClaudeCodePath string
	
	// MCP configuration
	MCPServerName string
	
	// Channel configuration
	Channels []ChannelConfig
}

type ChannelConfig struct {
	ChannelID string
	Name      string
	WorkDir   string
}

func Load() (*Config, error) {
	cfg := &Config{
		SlackBotToken:     os.Getenv("SLACK_BOT_TOKEN"),
		SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		DefaultWorkDir:    os.Getenv("CC_SLACK_DEFAULT_WORKDIR"),
		ClaudeCodePath:    getEnvOrDefault("CLAUDE_CODE_PATH", "claude"),
		MCPServerName:     getEnvOrDefault("MCP_SERVER_NAME", "cc-slack"),
	}
	
	// Parse port
	portStr := getEnvOrDefault("CC_SLACK_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}
	cfg.Port = port
	
	// Validate required fields
	if cfg.SlackBotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required")
	}
	if cfg.SlackSigningSecret == "" {
		return nil, fmt.Errorf("SLACK_SIGNING_SECRET is required")
	}
	if cfg.DefaultWorkDir == "" {
		return nil, fmt.Errorf("CC_SLACK_DEFAULT_WORKDIR is required")
	}
	
	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}