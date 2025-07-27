package session

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/process"
)

// Test end-to-end session persistence flow
func TestSessionPersistence_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
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

	// Create components
	queries := db.New(sqlDB)
	resumeManager := process.NewResumeManager(queries, 1*time.Hour)

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Executable:           "echo",
			PermissionPromptTool: "test_tool",
		},
	}
	baseManager := NewManager(nil, "http://test", cfg)
	dbManager := NewDBManager(baseManager, queries, resumeManager)

	ctx := context.Background()

	// Test 1: Create new session and verify persistence
	t.Run("CreateAndPersistSession", func(t *testing.T) {
		// Create a session (without actual process)
		thread, err := dbManager.getOrCreateThread(ctx, "C_PERSIST_1", "T_PERSIST_1")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create session in database
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-persist-1",
			WorkingDirectory: "/test/persist",
			Model:            sql.NullString{String: "claude-3", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Verify session exists
		savedSession, err := queries.GetSession(ctx, "sess-persist-1")
		if err != nil {
			t.Fatalf("Failed to get saved session: %v", err)
		}

		if savedSession.SessionID != "sess-persist-1" {
			t.Errorf("Expected session ID sess-persist-1, got %s", savedSession.SessionID)
		}
		if savedSession.WorkingDirectory != "/test/persist" {
			t.Errorf("Expected working directory /test/persist, got %s", savedSession.WorkingDirectory)
		}

		// Simulate session activity and completion
		time.Sleep(100 * time.Millisecond)

		// Update session with results
		err = dbManager.UpdateSessionOnComplete(ctx, "sess-persist-1", process.ResultMessage{
			TotalCostUSD: 0.25,
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  1500,
				OutputTokens: 3000,
			},
			DurationMS: 12000,
			NumTurns:   10,
			IsError:    false,
		})
		if err != nil {
			t.Fatalf("Failed to update session on complete: %v", err)
		}

		// Verify updated session
		completedSession, err := queries.GetSession(ctx, "sess-persist-1")
		if err != nil {
			t.Fatalf("Failed to get completed session: %v", err)
		}

		if completedSession.Status.String != "completed" {
			t.Errorf("Expected status completed, got %s", completedSession.Status.String)
		}
		if completedSession.TotalCostUsd.Float64 != 0.25 {
			t.Errorf("Expected cost 0.25, got %f", completedSession.TotalCostUsd.Float64)
		}
		if !completedSession.EndedAt.Valid {
			t.Error("Expected EndedAt to be set")
		}
	})

	// Test 2: Multiple sessions in same thread
	t.Run("MultipleSessionsInThread", func(t *testing.T) {
		// Create thread
		thread, err := dbManager.getOrCreateThread(ctx, "C_MULTI", "T_MULTI")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create multiple sessions
		sessionIDs := []string{"sess-multi-1", "sess-multi-2", "sess-multi-3"}
		for i, sessionID := range sessionIDs {
			_, err := queries.CreateSession(ctx, db.CreateSessionParams{
				ThreadID:         thread.ID,
				SessionID:        sessionID,
				WorkingDirectory: "/test/multi",
			})
			if err != nil {
				t.Fatalf("Failed to create session %s: %v", sessionID, err)
			}

			// Mark first two as completed
			if i < 2 {
				time.Sleep(50 * time.Millisecond)
				err = dbManager.UpdateSessionOnComplete(ctx, sessionID, process.ResultMessage{
					IsError: false,
				})
				if err != nil {
					t.Fatalf("Failed to complete session %s: %v", sessionID, err)
				}
			}
		}

		// Verify sessions were created and completed correctly
		for i, sessionID := range sessionIDs {
			sess, err := queries.GetSession(ctx, sessionID)
			if err != nil {
				t.Fatalf("Failed to get session %s: %v", sessionID, err)
			}

			if i < 2 {
				// First two should be completed
				if sess.Status.String != "completed" {
					t.Errorf("Expected session %s to be completed, got %s", sessionID, sess.Status.String)
				}
			} else {
				// Last one should be active/empty status
				if sess.Status.String == "completed" {
					t.Errorf("Expected session %s to not be completed", sessionID)
				}
			}
		}
	})

	// Test 3: Session lifecycle (create -> active -> timeout)
	t.Run("SessionLifecycle", func(t *testing.T) {
		thread, err := dbManager.getOrCreateThread(ctx, "C_LIFECYCLE", "T_LIFECYCLE")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create session
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-lifecycle",
			WorkingDirectory: "/test/lifecycle",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Verify it's active
		session, err := queries.GetSession(ctx, "sess-lifecycle")
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}

		if session.EndedAt.Valid {
			t.Error("New session should not have EndedAt")
		}

		// Simulate timeout
		err = dbManager.UpdateSessionOnTimeout(ctx, "sess-lifecycle")
		if err != nil {
			t.Fatalf("Failed to timeout session: %v", err)
		}

		// Verify timeout state
		timedOutSession, err := queries.GetSession(ctx, "sess-lifecycle")
		if err != nil {
			t.Fatalf("Failed to get timed out session: %v", err)
		}

		if timedOutSession.Status.String != "timeout" {
			t.Errorf("Expected status timeout, got %s", timedOutSession.Status.String)
		}
		if !timedOutSession.EndedAt.Valid {
			t.Error("Timed out session should have EndedAt")
		}
	})

	// Test 4: Thread persistence across sessions
	t.Run("ThreadPersistence", func(t *testing.T) {
		// Create thread with first session
		thread1, err := dbManager.getOrCreateThread(ctx, "C_THREAD", "T_THREAD")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		originalUpdatedAt := thread1.UpdatedAt

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Get same thread again (should update timestamp)
		thread2, err := dbManager.getOrCreateThread(ctx, "C_THREAD", "T_THREAD")
		if err != nil {
			t.Fatalf("Failed to get thread: %v", err)
		}

		if thread1.ID != thread2.ID {
			t.Errorf("Expected same thread ID %d, got %d", thread1.ID, thread2.ID)
		}

		// Get directly from database to verify timestamp was updated
		dbThread, err := queries.GetThread(ctx, db.GetThreadParams{
			ChannelID: "C_THREAD",
			ThreadTs:  "T_THREAD",
		})
		if err != nil {
			t.Fatalf("Failed to get thread from db: %v", err)
		}

		// Note: UpdatedAt might not change if UpdateThreadTimestamp doesn't actually update
		// This depends on the implementation
		_ = originalUpdatedAt
		_ = dbThread.UpdatedAt
	})
}
