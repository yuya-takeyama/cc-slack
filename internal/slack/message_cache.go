package slack

import (
	"fmt"
	"sync"
	"time"
)

// ProcessedMessageCache tracks processed messages to prevent duplicate processing
type ProcessedMessageCache struct {
	mu       sync.Mutex
	messages map[string]time.Time // key: "channelID:timestamp", value: processed time
}

// NewProcessedMessageCache creates a new message cache
func NewProcessedMessageCache() *ProcessedMessageCache {
	return &ProcessedMessageCache{
		messages: make(map[string]time.Time),
	}
}

// IsProcessed checks if a message has been recently processed
func (c *ProcessedMessageCache) IsProcessed(channelID, timestamp string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s:%s", channelID, timestamp)
	if processedAt, exists := c.messages[key]; exists {
		// Consider it a duplicate if processed within 5 seconds
		if time.Since(processedAt) < 5*time.Second {
			return true
		}
	}
	return false
}

// MarkProcessed marks a message as processed
func (c *ProcessedMessageCache) MarkProcessed(channelID, timestamp string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s:%s", channelID, timestamp)
	c.messages[key] = time.Now()

	// Clean up old entries
	c.cleanup()
}

// cleanup removes entries older than 1 minute
func (c *ProcessedMessageCache) cleanup() {
	cutoff := time.Now().Add(-1 * time.Minute)
	for key, processedAt := range c.messages {
		if processedAt.Before(cutoff) {
			delete(c.messages, key)
		}
	}
}
