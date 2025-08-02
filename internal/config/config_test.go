package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	// Change to testdata directory to use test config
	oldDir, _ := os.Getwd()
	os.Chdir("testdata")
	defer os.Chdir(oldDir)

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

	if cfg.Slack.SlashCommandName != "/cc" {
		t.Errorf("expected default slash command name /cc, got %s", cfg.Slack.SlashCommandName)
	}
}

func TestConfigEnvironment(t *testing.T) {
	// Change to testdata directory to use test config
	oldDir, _ := os.Getwd()
	os.Chdir("testdata")
	defer os.Chdir(oldDir)

	// Set environment variables
	os.Setenv("CC_SLACK_SLACK_BOT_TOKEN", "xoxb-test")
	os.Setenv("CC_SLACK_SLACK_SIGNING_SECRET", "test-secret")
	os.Setenv("CC_SLACK_SERVER_PORT", "9090")
	os.Setenv("CC_SLACK_DATABASE_PATH", "/custom/path/db.sqlite")
	os.Setenv("CC_SLACK_SESSION_RESUME_WINDOW", "2h")
	os.Setenv("CC_SLACK_SLACK_SLASH_COMMAND_NAME", "/cc")

	defer func() {
		os.Unsetenv("CC_SLACK_SLACK_BOT_TOKEN")
		os.Unsetenv("CC_SLACK_SLACK_SIGNING_SECRET")
		os.Unsetenv("CC_SLACK_SERVER_PORT")
		os.Unsetenv("CC_SLACK_DATABASE_PATH")
		os.Unsetenv("CC_SLACK_SESSION_RESUME_WINDOW")
		os.Unsetenv("CC_SLACK_SLACK_SLASH_COMMAND_NAME")
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

	if cfg.Slack.SlashCommandName != "/cc" {
		t.Errorf("expected slash command name /cc from env, got %s", cfg.Slack.SlashCommandName)
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
			// Change to testdata directory to use test config
			oldDir, _ := os.Getwd()
			os.Chdir("testdata")
			defer os.Chdir(oldDir)

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

func TestIsSingleDirectoryMode(t *testing.T) {
	tests := []struct {
		name             string
		config           Config
		expectedIsSingle bool
		expectedWorkDir  string
	}{
		{
			name: "CLI flag set single directory",
			config: Config{
				WorkingDirFlags: []string{"/path/from/cli"},
				WorkingDirs: []WorkingDirectoryConfig{
					{Name: "dir1", Path: "/path/1"},
					{Name: "dir2", Path: "/path/2"},
				},
			},
			expectedIsSingle: true,
			expectedWorkDir:  "/path/from/cli",
		},
		{
			name: "Single working directory configured",
			config: Config{
				WorkingDirFlags: []string{},
				WorkingDirs: []WorkingDirectoryConfig{
					{Name: "single-dir", Path: "/only/path"},
				},
			},
			expectedIsSingle: true,
			expectedWorkDir:  "/only/path",
		},
		{
			name: "Multiple working directories configured",
			config: Config{
				WorkingDirFlags: []string{},
				WorkingDirs: []WorkingDirectoryConfig{
					{Name: "dir1", Path: "/path/1"},
					{Name: "dir2", Path: "/path/2"},
				},
			},
			expectedIsSingle: false,
			expectedWorkDir:  "",
		},
		{
			name: "No configuration",
			config: Config{
				WorkingDirFlags: []string{},
				WorkingDirs:     []WorkingDirectoryConfig{},
			},
			expectedIsSingle: false,
			expectedWorkDir:  "",
		},
		{
			name: "CLI flag set multiple directories",
			config: Config{
				WorkingDirFlags: []string{"/path/1", "/path/2"},
				WorkingDirs:     []WorkingDirectoryConfig{},
			},
			expectedIsSingle: false,
			expectedWorkDir:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSingle := tt.config.IsSingleDirectoryMode()
			if isSingle != tt.expectedIsSingle {
				t.Errorf("IsSingleDirectoryMode() = %v, want %v", isSingle, tt.expectedIsSingle)
			}

			workDir := tt.config.GetSingleWorkingDirectory()
			if workDir != tt.expectedWorkDir {
				t.Errorf("GetSingleWorkingDirectory() = %v, want %v", workDir, tt.expectedWorkDir)
			}
		})
	}
}
