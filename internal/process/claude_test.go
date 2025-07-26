package process

import (
	"testing"
	"time"
)

func TestGenerateLogFileName(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "normal timestamp",
			time:     time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: "claude-20240115-143045.log",
		},
		{
			name:     "midnight",
			time:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			expected: "claude-20241231-000000.log",
		},
		{
			name:     "with nanoseconds",
			time:     time.Date(2024, 3, 10, 9, 8, 7, 123456789, time.UTC),
			expected: "claude-20240310-090807.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateLogFileName(tt.time)
			if got != tt.expected {
				t.Errorf("generateLogFileName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildMCPConfig(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "http URL",
			baseURL: "http://localhost:8080",
		},
		{
			name:    "https URL",
			baseURL: "https://example.com",
		},
		{
			name:    "URL with path",
			baseURL: "http://localhost:3000/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildMCPConfig(tt.baseURL)

			// Check structure
			mcpServers, ok := config["mcpServers"].(map[string]interface{})
			if !ok {
				t.Fatal("mcpServers should be a map")
			}

			ccSlack, ok := mcpServers["cc-slack"].(map[string]interface{})
			if !ok {
				t.Fatal("cc-slack should be a map")
			}

			// Check type
			if ccSlack["type"] != "http" {
				t.Errorf("type = %v, want 'http'", ccSlack["type"])
			}

			// Check URL
			expectedURL := tt.baseURL + "/mcp"
			if ccSlack["url"] != expectedURL {
				t.Errorf("url = %v, want %v", ccSlack["url"], expectedURL)
			}
		})
	}
}
