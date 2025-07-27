package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/process"
)

// Integration tests using real database
func TestDBManager_Integration(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsPath := filepath.Join("..", "..", "migrations")

	sqlDB, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer sqlDB.Close()

	// Run migrations
	err = database.Migrate(sqlDB, migrationsPath)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create queries instance
	queries := db.New(sqlDB)

	// Create resume manager
	resumeManager := process.NewResumeManager(queries, 1*time.Hour)

	// Create base manager
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Executable:           "echo", // Use echo as dummy
			PermissionPromptTool: "test_tool",
		},
	}
	baseManager := NewManager(nil, "http://test", cfg)

	// Create DBManager
	dbManager := NewDBManager(baseManager, queries, resumeManager)

	// Run tests
	t.Run("getOrCreateThread", func(t *testing.T) {
		testGetOrCreateThread(t, dbManager, queries)
	})

	t.Run("UpdateSessionOnComplete", func(t *testing.T) {
		testUpdateSessionOnComplete(t, dbManager, queries)
	})

	t.Run("UpdateSessionOnTimeout", func(t *testing.T) {
		testUpdateSessionOnTimeout(t, dbManager, queries)
	})
}

func testGetOrCreateThread(t *testing.T, dbManager *DBManager, queries *db.Queries) {
	ctx := context.Background()

	// Test creating new thread
	thread1, err := dbManager.getOrCreateThread(ctx, "C123", "T456")
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

	if thread1.ChannelID != "C123" {
		t.Errorf("Expected channel ID C123, got %s", thread1.ChannelID)
	}
	if thread1.ThreadTs != "T456" {
		t.Errorf("Expected thread TS T456, got %s", thread1.ThreadTs)
	}

	// Test getting existing thread
	thread2, err := dbManager.getOrCreateThread(ctx, "C123", "T456")
	if err != nil {
		t.Fatalf("Failed to get existing thread: %v", err)
	}

	if thread2.ID != thread1.ID {
		t.Errorf("Expected same thread ID %d, got %d", thread1.ID, thread2.ID)
	}

	// Verify thread exists in database
	dbThread, err := queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: "C123",
		ThreadTs:  "T456",
	})
	if err != nil {
		t.Fatalf("Failed to get thread from database: %v", err)
	}

	if dbThread.ID != thread1.ID {
		t.Errorf("Thread ID mismatch: expected %d, got %d", thread1.ID, dbThread.ID)
	}
}

func testUpdateSessionOnComplete(t *testing.T, dbManager *DBManager, queries *db.Queries) {
	ctx := context.Background()

	// Create a thread first
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C789",
		ThreadTs:  "T012",
	})
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

	// Create a session
	_, err = queries.CreateSession(ctx, db.CreateSessionParams{
		ThreadID:         thread.ID,
		SessionID:        "sess-complete-test",
		WorkingDirectory: "/test/dir",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test successful completion
	result := process.ResultMessage{
		TotalCostUSD: 0.123,
		Usage: struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}{
			InputTokens:  1000,
			OutputTokens: 2000,
		},
		DurationMS: 5000,
		NumTurns:   5,
		IsError:    false,
	}

	err = dbManager.UpdateSessionOnComplete(ctx, "sess-complete-test", result)
	if err != nil {
		t.Fatalf("Failed to update session on complete: %v", err)
	}

	// Verify update in database
	updatedSession, err := queries.GetSession(ctx, "sess-complete-test")
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}

	if updatedSession.Status.String != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", updatedSession.Status.String)
	}
	if updatedSession.TotalCostUsd.Float64 != 0.123 {
		t.Errorf("Expected cost 0.123, got %f", updatedSession.TotalCostUsd.Float64)
	}
	if updatedSession.InputTokens.Int64 != 1000 {
		t.Errorf("Expected input tokens 1000, got %d", updatedSession.InputTokens.Int64)
	}

	// Test failed session
	_, err = queries.CreateSession(ctx, db.CreateSessionParams{
		ThreadID:         thread.ID,
		SessionID:        "sess-failed-test",
		WorkingDirectory: "/test/dir",
	})
	if err != nil {
		t.Fatalf("Failed to create session2: %v", err)
	}

	failedResult := process.ResultMessage{
		IsError: true,
	}

	err = dbManager.UpdateSessionOnComplete(ctx, "sess-failed-test", failedResult)
	if err != nil {
		t.Fatalf("Failed to update failed session: %v", err)
	}

	// Verify failed status
	failedSession, err := queries.GetSession(ctx, "sess-failed-test")
	if err != nil {
		t.Fatalf("Failed to get failed session: %v", err)
	}

	if failedSession.Status.String != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", failedSession.Status.String)
	}
}

func testUpdateSessionOnTimeout(t *testing.T, dbManager *DBManager, queries *db.Queries) {
	ctx := context.Background()

	// Create a thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: "C999",
		ThreadTs:  "T999",
	})
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

	// Create a session
	_, err = queries.CreateSession(ctx, db.CreateSessionParams{
		ThreadID:         thread.ID,
		SessionID:        "sess-timeout-test",
		WorkingDirectory: "/test/dir",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test timeout update
	err = dbManager.UpdateSessionOnTimeout(ctx, "sess-timeout-test")
	if err != nil {
		t.Fatalf("Failed to update session on timeout: %v", err)
	}

	// Verify update in database
	timedOutSession, err := queries.GetSession(ctx, "sess-timeout-test")
	if err != nil {
		t.Fatalf("Failed to get timed out session: %v", err)
	}

	if timedOutSession.Status.String != "timeout" {
		t.Errorf("Expected status 'timeout', got '%s'", timedOutSession.Status.String)
	}

	// EndedAt should be set
	if !timedOutSession.EndedAt.Valid {
		t.Error("Expected EndedAt to be set for timed out session")
	}
}
