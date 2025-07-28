package slack

import (
	"fmt"
	"testing"
	"time"
)

func TestProcessedMessageCache_IsProcessed(t *testing.T) {
	cache := NewProcessedMessageCache()

	// Test unprocessed message
	if cache.IsProcessed("C123", "1234567890.123456") {
		t.Error("Expected unprocessed message to return false")
	}

	// Mark as processed
	cache.MarkProcessed("C123", "1234567890.123456")

	// Test processed message (within 5 seconds)
	if !cache.IsProcessed("C123", "1234567890.123456") {
		t.Error("Expected processed message to return true")
	}

	// Test different channel/timestamp
	if cache.IsProcessed("C456", "1234567890.123456") {
		t.Error("Expected different channel to return false")
	}
	if cache.IsProcessed("C123", "9876543210.654321") {
		t.Error("Expected different timestamp to return false")
	}
}

func TestProcessedMessageCache_IsProcessed_Expiry(t *testing.T) {
	cache := NewProcessedMessageCache()

	// Mark as processed
	cache.MarkProcessed("C123", "1234567890.123456")

	// Manually set the timestamp to more than 5 seconds ago
	cache.mu.Lock()
	key := "C123:1234567890.123456"
	cache.messages[key] = time.Now().Add(-6 * time.Second)
	cache.mu.Unlock()

	// Should no longer be considered processed
	if cache.IsProcessed("C123", "1234567890.123456") {
		t.Error("Expected expired message to return false")
	}
}

func TestProcessedMessageCache_Cleanup(t *testing.T) {
	cache := NewProcessedMessageCache()

	// Add messages with different timestamps
	now := time.Now()
	cache.mu.Lock()
	cache.messages["C123:old"] = now.Add(-2 * time.Minute)     // Should be cleaned up
	cache.messages["C123:recent"] = now.Add(-30 * time.Second) // Should remain
	cache.mu.Unlock()

	// Trigger cleanup by marking a new message
	cache.MarkProcessed("C123", "new")

	// Check that old message was cleaned up
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, exists := cache.messages["C123:old"]; exists {
		t.Error("Expected old message to be cleaned up")
	}
	if _, exists := cache.messages["C123:recent"]; !exists {
		t.Error("Expected recent message to remain")
	}
	if _, exists := cache.messages["C123:new"]; !exists {
		t.Error("Expected new message to exist")
	}
}

func TestProcessedMessageCache_ConcurrentAccess(t *testing.T) {
	cache := NewProcessedMessageCache()
	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			cache.MarkProcessed("C123", fmt.Sprintf("%d", i))
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			cache.IsProcessed("C123", fmt.Sprintf("%d", i))
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without deadlock or panic, the test passes
}
