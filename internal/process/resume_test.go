package process

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) (*sql.DB, *db.Queries, func()) {
	// Create in-memory database
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := database.Migrate(sqlDB, "../../migrations"); err != nil {
		sqlDB.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	queries := db.New(sqlDB)

	return sqlDB, queries, func() {
		sqlDB.Close()
	}
}

func TestResumeManager_GetLatestSessionID(t *testing.T) {
	_, queries, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	rm := NewResumeManager(queries, time.Hour)

	// Create test thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C123456",
		ThreadTs:  "1234567890.123456",
	})
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// Create test session
	session, err := queries.CreateSessionWithInitialPrompt(ctx, db.CreateSessionWithInitialPromptParams{
		ThreadID:      thread.ID,
		SessionID:     "test-session-123",
		Model:         sql.NullString{String: "claude-3", Valid: true},
		InitialPrompt: sql.NullString{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update session to completed
	err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
		Status:    sql.NullString{String: "completed", Valid: true},
		SessionID: session.SessionID,
	})
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Test getting latest session
	sessionID, err := rm.GetLatestSessionID(ctx, "C123456", "1234567890.123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sessionID != "test-session-123" {
		t.Errorf("expected session ID %s, got %s", "test-session-123", sessionID)
	}
}

func TestResumeManager_GetLatestSessionID_NoThread(t *testing.T) {
	_, queries, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	rm := NewResumeManager(queries, time.Hour)

	// Test with non-existent thread
	_, err := rm.GetLatestSessionID(ctx, "C999999", "9999999999.999999")
	if err == nil {
		t.Error("expected error for non-existent thread")
	}
}

func TestResumeManager_ShouldResume(t *testing.T) {
	sqlDB, queries, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	rm := NewResumeManager(queries, 30*time.Minute)

	// Create test thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C123456",
		ThreadTs:  "1234567890.123456",
	})
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// Create test session that ended recently
	session, err := queries.CreateSessionWithInitialPrompt(ctx, db.CreateSessionWithInitialPromptParams{
		ThreadID:      thread.ID,
		SessionID:     "test-session-recent",
		Model:         sql.NullString{String: "claude-3", Valid: true},
		InitialPrompt: sql.NullString{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update with recent end time
	_, err = sqlDB.Exec(
		"UPDATE sessions SET status = 'completed', ended_at = datetime('now', '-10 minutes') WHERE session_id = ?",
		session.SessionID,
	)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Should resume (within window)
	shouldResume, sessionID, err := rm.ShouldResume(ctx, "C123456", "1234567890.123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldResume {
		t.Error("expected shouldResume to be true")
	}

	if sessionID != "test-session-recent" {
		t.Errorf("expected session ID %s, got %s", "test-session-recent", sessionID)
	}
}

func TestResumeManager_ShouldResume_OutsideWindow(t *testing.T) {
	sqlDB, queries, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	rm := NewResumeManager(queries, 30*time.Minute)

	// Create test thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C123456",
		ThreadTs:  "1234567890.123456",
	})
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// Create test session that ended long ago
	session, err := queries.CreateSessionWithInitialPrompt(ctx, db.CreateSessionWithInitialPromptParams{
		ThreadID:      thread.ID,
		SessionID:     "test-session-old",
		Model:         sql.NullString{String: "claude-3", Valid: true},
		InitialPrompt: sql.NullString{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update with old end time
	_, err = sqlDB.Exec(
		"UPDATE sessions SET status = 'completed', ended_at = datetime('now', '-2 hours') WHERE session_id = ?",
		session.SessionID,
	)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Should not resume (outside window)
	shouldResume, _, err := rm.ShouldResume(ctx, "C123456", "1234567890.123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shouldResume {
		t.Error("expected shouldResume to be false")
	}
}

func TestResumeManager_CheckActiveSession(t *testing.T) {
	_, queries, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	rm := NewResumeManager(queries, time.Hour)

	// Create test thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C123456",
		ThreadTs:  "1234567890.123456",
	})
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// Initially no active sessions
	hasActive, err := rm.CheckActiveSession(ctx, "C123456", "1234567890.123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasActive {
		t.Error("expected no active sessions")
	}

	// Create active session
	_, err = queries.CreateSessionWithInitialPrompt(ctx, db.CreateSessionWithInitialPromptParams{
		ThreadID:      thread.ID,
		SessionID:     "test-session-active",
		Model:         sql.NullString{String: "claude-3", Valid: true},
		InitialPrompt: sql.NullString{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Now should have active session
	hasActive, err = rm.CheckActiveSession(ctx, "C123456", "1234567890.123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasActive {
		t.Error("expected active session")
	}
}
