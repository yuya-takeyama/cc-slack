package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Slack workspace subdomain
// TODO: Migrate to DB when supporting multiple workspaces
const SLACK_WORKSPACE_SUBDOMAIN = "yuyat"

// Config represents the complete configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Slack    SlackConfig    `mapstructure:"slack"`
	Claude   ClaudeConfig   `mapstructure:"claude"`
	Database DatabaseConfig `mapstructure:"database"`
	Session  SessionConfig  `mapstructure:"session"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

// SlackConfig contains Slack-related settings
type SlackConfig struct {
	BotToken      string              `mapstructure:"bot_token"`
	AppToken      string              `mapstructure:"app_token"`
	SigningSecret string              `mapstructure:"signing_secret"`
	Assistant     AssistantConfig     `mapstructure:"assistant"`
	FileUpload    FileUploadConfig    `mapstructure:"file_upload"`
	MessageFilter MessageFilterConfig `mapstructure:"message_filter"`
}

// AssistantConfig contains assistant display settings
type AssistantConfig struct {
	Username  string `mapstructure:"username"`
	IconEmoji string `mapstructure:"icon_emoji"`
	IconURL   string `mapstructure:"icon_url"`
}

// ClaudeConfig contains Claude Code settings
type ClaudeConfig struct {
	Executable           string   `mapstructure:"executable"`
	DefaultOptions       []string `mapstructure:"default_options"`
	PermissionPromptTool string   `mapstructure:"permission_prompt_tool"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path           string `mapstructure:"path"`
	MigrationsPath string `mapstructure:"migrations_path"`
}

// SessionConfig contains session management settings
type SessionConfig struct {
	Timeout         time.Duration `mapstructure:"timeout"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	ResumeWindow    time.Duration `mapstructure:"resume_window"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// FileUploadConfig contains file upload settings
type FileUploadConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	ImagesDir string `mapstructure:"images_dir"`
}

// MessageFilterConfig contains message filtering settings
type MessageFilterConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	IncludePatterns []string `mapstructure:"include_patterns"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
	RequireMention  bool     `mapstructure:"require_mention"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/cc-slack/")

	// Environment variable settings
	v.SetEnvPrefix("CC_SLACK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind environment variables
	v.BindEnv("slack.bot_token")
	v.BindEnv("slack.signing_secret")
	v.BindEnv("slack.app_token")
	v.BindEnv("slack.assistant.username")
	v.BindEnv("slack.assistant.icon_emoji")
	v.BindEnv("slack.assistant.icon_url")
	v.BindEnv("slack.file_upload.enabled")
	v.BindEnv("slack.file_upload.images_dir")
	v.BindEnv("server.port")
	v.BindEnv("server.base_url")
	v.BindEnv("database.path")
	v.BindEnv("database.migrations_path")
	v.BindEnv("session.timeout")
	v.BindEnv("session.cleanup_interval")
	v.BindEnv("session.resume_window")

	// Set defaults with the new viper instance
	setDefaultsWithViper(v)

	// Read config file if exists
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal to struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaultsWithViper sets default values with a specific viper instance
func setDefaultsWithViper(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.base_url", "http://localhost:8080")

	// Claude defaults
	v.SetDefault("claude.executable", "claude")
	v.SetDefault("claude.permission_prompt_tool", "mcp__cc-slack__approval_prompt")

	// Database defaults
	v.SetDefault("database.path", "./data/cc-slack.db")
	v.SetDefault("database.migrations_path", "./migrations")

	// Session defaults
	v.SetDefault("session.timeout", "30m")
	v.SetDefault("session.cleanup_interval", "5m")
	v.SetDefault("session.resume_window", "1h")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "./logs")

	// File upload defaults
	v.SetDefault("slack.file_upload.enabled", true)
	v.SetDefault("slack.file_upload.images_dir", "./tmp/uploaded_images")

	// Message filter defaults
	v.SetDefault("slack.message_filter.enabled", true)
	v.SetDefault("slack.message_filter.require_mention", true)
	v.SetDefault("slack.message_filter.include_patterns", []string{})
	v.SetDefault("slack.message_filter.exclude_patterns", []string{})
}

// validate validates the configuration
func (c *Config) validate() error {
	// Validate required Slack settings
	if c.Slack.BotToken == "" {
		return fmt.Errorf("slack.bot_token is required")
	}
	if c.Slack.SigningSecret == "" {
		return fmt.Errorf("slack.signing_secret is required")
	}

	// Validate server settings
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server.port: %d", c.Server.Port)
	}

	// Validate time durations
	if c.Session.Timeout <= 0 {
		return fmt.Errorf("session.timeout must be positive")
	}
	if c.Session.CleanupInterval <= 0 {
		return fmt.Errorf("session.cleanup_interval must be positive")
	}
	if c.Session.ResumeWindow <= 0 {
		return fmt.Errorf("session.resume_window must be positive")
	}

	return nil
}
