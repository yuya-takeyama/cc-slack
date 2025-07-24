package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Save current env vars and restore after test
	origBotToken := os.Getenv("SLACK_BOT_TOKEN")
	origSigningSecret := os.Getenv("SLACK_SIGNING_SECRET")
	origWorkDir := os.Getenv("CC_SLACK_DEFAULT_WORKDIR")
	defer func() {
		os.Setenv("SLACK_BOT_TOKEN", origBotToken)
		os.Setenv("SLACK_SIGNING_SECRET", origSigningSecret)
		os.Setenv("CC_SLACK_DEFAULT_WORKDIR", origWorkDir)
	}()

	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			envVars: map[string]string{
				"SLACK_BOT_TOKEN":          "xoxb-test-token",
				"SLACK_SIGNING_SECRET":     "test-secret",
				"CC_SLACK_DEFAULT_WORKDIR": "/test/workdir",
			},
			wantErr: false,
		},
		{
			name: "missing bot token",
			envVars: map[string]string{
				"SLACK_SIGNING_SECRET":     "test-secret",
				"CC_SLACK_DEFAULT_WORKDIR": "/test/workdir",
			},
			wantErr: true,
		},
		{
			name: "missing signing secret",
			envVars: map[string]string{
				"SLACK_BOT_TOKEN":          "xoxb-test-token",
				"CC_SLACK_DEFAULT_WORKDIR": "/test/workdir",
			},
			wantErr: true,
		},
		{
			name: "missing default workdir",
			envVars: map[string]string{
				"SLACK_BOT_TOKEN":      "xoxb-test-token",
				"SLACK_SIGNING_SECRET": "test-secret",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			envVars: map[string]string{
				"SLACK_BOT_TOKEN":          "xoxb-test-token",
				"SLACK_SIGNING_SECRET":     "test-secret",
				"CC_SLACK_DEFAULT_WORKDIR": "/test/workdir",
				"CC_SLACK_PORT":            "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars
			os.Clearenv()

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg != nil {
				// Check defaults
				if cfg.ClaudeCodePath != "claude" {
					t.Errorf("Expected default ClaudeCodePath 'claude', got %s", cfg.ClaudeCodePath)
				}
				if cfg.MCPServerName != "cc-slack" {
					t.Errorf("Expected default MCPServerName 'cc-slack', got %s", cfg.MCPServerName)
				}
				if cfg.Port == 0 {
					t.Error("Port should not be 0")
				}
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var exists",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "actual",
			want:         "actual",
		},
		{
			name:         "env var empty",
			key:          "TEST_VAR_EMPTY",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}