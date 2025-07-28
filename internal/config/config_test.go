package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	// Set required values
	os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
	os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
	defer os.Unsetenv("CC_SLACK_SLACK_BOT_TOKEN")
	defer os.Unsetenv("CC_SLACK_SLACK_SIGNING_SECRET")

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Test defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}

	if cfg.Server.BaseURL != "http://localhost:8080" {
		t.Errorf("expected default base URL http://localhost:8080, got %s", cfg.Server.BaseURL)
	}

	if cfg.Database.Path != "./data/cc-slack.db" {
		t.Errorf("expected default database path ./data/cc-slack.db, got %s", cfg.Database.Path)
	}

	if cfg.Session.Timeout != 30*time.Minute {
		t.Errorf("expected default timeout 30m, got %v", cfg.Session.Timeout)
	}

	if cfg.Session.ResumeWindow != time.Hour {
		t.Errorf("expected default resume window 1h, got %v", cfg.Session.ResumeWindow)
	}
}

func TestConfigEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
	os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
	os.Setenv("CC_SLACK_SERVER_PORT", "9090")
	os.Setenv("CC_SLACK_DATABASE_PATH", "/custom/path/db.sqlite")
	os.Setenv("CC_SLACK_SESSION_RESUME_WINDOW", "2h")

	defer func() {
		os.Unsetenv("CC_SLACK_SLACK_BOT_TOKEN")
		os.Unsetenv("CC_SLACK_SLACK_SIGNING_SECRET")
		os.Unsetenv("CC_SLACK_SERVER_PORT")
		os.Unsetenv("CC_SLACK_DATABASE_PATH")
		os.Unsetenv("CC_SLACK_SESSION_RESUME_WINDOW")
	}()

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Test environment overrides
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090 from env, got %d", cfg.Server.Port)
	}

	if cfg.Database.Path != "/custom/path/db.sqlite" {
		t.Errorf("expected database path from env, got %s", cfg.Database.Path)
	}

	if cfg.Session.ResumeWindow != 2*time.Hour {
		t.Errorf("expected resume window 2h from env, got %v", cfg.Session.ResumeWindow)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func()
		wantErr   bool
	}{
		{
			name: "missing bot token",
			setupFunc: func() {
				os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing signing secret",
			setupFunc: func() {
				os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			setupFunc: func() {
				os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
				os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
				os.Setenv("CC_SLACK_SERVER_PORT", "99999")
			},
			wantErr: true,
		},
		{
			name: "valid config",
			setupFunc: func() {
				os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
				os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear specific env vars
			os.Unsetenv("CC_SLACK_SLACK_BOT_TOKEN")
			os.Unsetenv("CC_SLACK_SLACK_SIGNING_SECRET")
			os.Unsetenv("CC_SLACK_SERVER_PORT")

			// Setup test
			tt.setupFunc()

			// Cleanup after test
			defer func() {
				os.Unsetenv("CC_SLACK_SLACK_BOT_TOKEN")
				os.Unsetenv("CC_SLACK_SLACK_SIGNING_SECRET")
				os.Unsetenv("CC_SLACK_SERVER_PORT")
			}()

			// Load config
			_, err := Load()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRepositories(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "valid repositories",
			config: Config{
				Repositories: []RepositoryConfig{
					{
						Name: "frontend",
						Path: "/path/to/frontend",
					},
					{
						Name: "backend",
						Path: "/path/to/backend",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "missing name",
			config: Config{
				Repositories: []RepositoryConfig{
					{
						Path: "/path/to/repo",
					},
				},
			},
			wantErr: "repositories[0].name is required",
		},
		{
			name: "missing path",
			config: Config{
				Repositories: []RepositoryConfig{
					{
						Name: "repo",
					},
				},
			},
			wantErr: "repositories[0].path is required",
		},
		{
			name: "relative path",
			config: Config{
				Repositories: []RepositoryConfig{
					{
						Name: "repo",
						Path: "relative/path",
					},
				},
			},
			wantErr: "repositories[0].path must be absolute path: relative/path",
		},
		{
			name: "duplicate names",
			config: Config{
				Repositories: []RepositoryConfig{
					{
						Name: "repo",
						Path: "/path1",
					},
					{
						Name: "repo",
						Path: "/path2",
					},
				},
			},
			wantErr: "duplicate repository name: repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set required fields to pass other validations
			tt.config.Slack.BotToken = "xoxb-test"
			tt.config.Slack.SigningSecret = "test-secret"
			tt.config.Server.Port = 8080
			tt.config.Session.Timeout = 30
			tt.config.Session.CleanupInterval = 5
			tt.config.Session.ResumeWindow = 60

			err := tt.config.validate()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadWithRepositories(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 9090

slack:
  bot_token: xoxb-test
  app_token: xapp-test
  signing_secret: test-secret

repositories:
  - name: test-repo
    path: /tmp/test-repo
    default_branch: develop
    channels:
      - C123456
    slack_override:
      username: Test Bot
      icon_emoji: ":test:"

working_directories:
  default: /tmp/default
  worktree_directory: .test-worktrees
  worktree_retention_period: 12h
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to temp directory to load config
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("expected 1 repository, got %d", len(cfg.Repositories))
	}

	repo := cfg.Repositories[0]
	if repo.Name != "test-repo" {
		t.Errorf("expected repo name 'test-repo', got %s", repo.Name)
	}
	if repo.Path != "/tmp/test-repo" {
		t.Errorf("expected repo path '/tmp/test-repo', got %s", repo.Path)
	}
	if repo.DefaultBranch != "develop" {
		t.Errorf("expected default branch 'develop', got %s", repo.DefaultBranch)
	}
	if len(repo.Channels) != 1 || repo.Channels[0] != "C123456" {
		t.Errorf("expected channels [C123456], got %v", repo.Channels)
	}
	if repo.SlackOverride == nil {
		t.Fatal("expected SlackOverride to be set")
	}
	if repo.SlackOverride.Username != "Test Bot" {
		t.Errorf("expected username 'Test Bot', got %s", repo.SlackOverride.Username)
	}
	if repo.SlackOverride.IconEmoji != ":test:" {
		t.Errorf("expected icon emoji ':test:', got %s", repo.SlackOverride.IconEmoji)
	}

	if cfg.WorkingDirectories.Default != "/tmp/default" {
		t.Errorf("expected default working directory '/tmp/default', got %s", cfg.WorkingDirectories.Default)
	}
	if cfg.WorkingDirectories.WorktreeDirectory != ".test-worktrees" {
		t.Errorf("expected worktree directory '.test-worktrees', got %s", cfg.WorkingDirectories.WorktreeDirectory)
	}
	if cfg.WorkingDirectories.WorktreeRetentionPeriod != "12h" {
		t.Errorf("expected retention period '12h', got %s", cfg.WorkingDirectories.WorktreeRetentionPeriod)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
