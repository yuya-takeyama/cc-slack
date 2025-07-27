package config

import (
	"log"
	"os"
	"time"
)

// GetEnv returns an environment variable value or a default
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDurationEnv returns a duration from environment variable or a default
func GetDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("Invalid duration for %s: %s, using default: %v", key, value, defaultValue)
		return defaultValue
	}

	return duration
}
