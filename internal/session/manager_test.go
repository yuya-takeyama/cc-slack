package session

import (
	"testing"
	"time"
)

func TestManager_CreateSession(t *testing.T) {
	manager := NewManager()

	session, err := manager.CreateSession("C123456", "/test/workdir")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.ChannelID != "C123456" {
		t.Errorf("Expected ChannelID 'C123456', got %s", session.ChannelID)
	}

	if session.WorkDir != "/test/workdir" {
		t.Errorf("Expected WorkDir '/test/workdir', got %s", session.WorkDir)
	}

	if session.SessionID == "" {
		t.Error("SessionID should not be empty")
	}

	// Check that session is stored
	stored := manager.GetBySessionID(session.SessionID)
	if stored == nil {
		t.Error("Session should be stored in manager")
	}
}

func TestManager_UpdateSessionID(t *testing.T) {
	manager := NewManager()

	// Create session
	session, _ := manager.CreateSession("C123456", "/test/workdir")
	oldID := session.SessionID
	newID := "new-session-id"

	// Update session ID
	manager.UpdateSessionID(oldID, newID)

	// Old ID should not exist
	if manager.GetBySessionID(oldID) != nil {
		t.Error("Old session ID should not exist")
	}

	// New ID should exist
	updated := manager.GetBySessionID(newID)
	if updated == nil {
		t.Error("New session ID should exist")
	}

	if updated.SessionID != newID {
		t.Errorf("Expected SessionID '%s', got '%s'", newID, updated.SessionID)
	}
}

func TestManager_UpdateThreadTS(t *testing.T) {
	manager := NewManager()

	// Create session
	session, _ := manager.CreateSession("C123456", "/test/workdir")
	threadTS := "1234567890.123456"

	// Update thread TS
	manager.UpdateThreadTS(session.SessionID, threadTS)

	// Should be retrievable by thread TS
	byThread := manager.GetByThreadTS(threadTS)
	if byThread == nil {
		t.Error("Session should be retrievable by thread TS")
	}

	if byThread.ThreadTS != threadTS {
		t.Errorf("Expected ThreadTS '%s', got '%s'", threadTS, byThread.ThreadTS)
	}
}

func TestManager_RemoveSession(t *testing.T) {
	manager := NewManager()

	// Create session
	session, _ := manager.CreateSession("C123456", "/test/workdir")
	threadTS := "1234567890.123456"
	manager.UpdateThreadTS(session.SessionID, threadTS)

	// Remove session
	manager.RemoveSession(session.SessionID)

	// Should not be retrievable by session ID
	if manager.GetBySessionID(session.SessionID) != nil {
		t.Error("Session should be removed")
	}

	// Should not be retrievable by thread TS
	if manager.GetByThreadTS(threadTS) != nil {
		t.Error("Session should be removed from thread mapping")
	}
}

func TestManager_CleanupIdleSessions(t *testing.T) {
	manager := NewManager()

	// Create sessions
	session1, _ := manager.CreateSession("C123456", "/test/workdir")
	session2, _ := manager.CreateSession("C789012", "/test/workdir")

	// Store IDs before modification
	session1ID := session1.SessionID
	session2ID := session2.SessionID

	// Make session1 idle by directly modifying it in the map
	manager.mu.Lock()
	if s, exists := manager.sessions[session1ID]; exists {
		s.LastActive = time.Now().Add(-2 * time.Hour)
	}
	manager.mu.Unlock()

	// Debug: Print sessions before cleanup
	t.Logf("Sessions before cleanup: %d", len(manager.sessions))

	// Cleanup with 1 hour timeout
	manager.CleanupIdleSessions(1 * time.Hour)

	// Debug: Print sessions after cleanup
	t.Logf("Sessions after cleanup: %d", len(manager.sessions))

	// Session1 should be removed
	if manager.GetBySessionID(session1ID) != nil {
		t.Error("Idle session should be removed")
	}

	// Session2 should still exist
	if manager.GetBySessionID(session2ID) == nil {
		t.Errorf("Active session should not be removed. Session2 ID: %s", session2ID)
		// Debug: List all remaining sessions
		for id := range manager.sessions {
			t.Logf("Remaining session ID: %s", id)
		}
	}
}

func TestGenerateTempSessionID(t *testing.T) {
	id1 := generateTempSessionID()

	if len(id1) == 0 {
		t.Error("Generated ID should not be empty")
	}

	// Check format
	if len(id1) < 10 {
		t.Error("Generated ID seems too short")
	}
	
	// Check prefix
	if len(id1) < 5 || id1[:5] != "temp-" {
		t.Error("Generated ID should start with 'temp-'")
	}
	
	// Generate multiple IDs and check uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateTempSessionID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}