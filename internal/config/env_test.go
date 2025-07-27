package config

import (
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "env var exists",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "env var empty",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR_UNSET",
			defaultValue: "fallback",
			envValue:     "",
			expected:     "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := GetEnv(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("GetEnv(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.expected)
			}
		})
	}
}

func TestGetDurationEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		envValue     string
		expected     time.Duration
	}{
		{
			name:         "valid duration",
			key:          "TEST_DURATION",
			defaultValue: 5 * time.Minute,
			envValue:     "10m",
			expected:     10 * time.Minute,
		},
		{
			name:         "invalid duration",
			key:          "TEST_DURATION",
			defaultValue: 5 * time.Minute,
			envValue:     "invalid",
			expected:     5 * time.Minute,
		},
		{
			name:         "empty env var",
			key:          "TEST_DURATION",
			defaultValue: 30 * time.Second,
			envValue:     "",
			expected:     30 * time.Second,
		},
		{
			name:         "complex duration",
			key:          "TEST_DURATION",
			defaultValue: time.Hour,
			envValue:     "1h30m45s",
			expected:     time.Hour + 30*time.Minute + 45*time.Second,
		},
		{
			name:         "milliseconds",
			key:          "TEST_DURATION",
			defaultValue: time.Second,
			envValue:     "500ms",
			expected:     500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := GetDurationEnv(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("GetDurationEnv(%q, %v) = %v, want %v", tt.key, tt.defaultValue, got, tt.expected)
			}
		})
	}
}
