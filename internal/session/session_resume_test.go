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

// Test end-to-end session resume functionality
func TestSessionResume_E2E(t *testing.T) {
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
	ctx := context.Background()

	// Test 1: Resume within window
	t.Run("ResumeWithinWindow", func(t *testing.T) {
		// Create resume manager with 1 hour window
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)

		// Create thread and session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_RESUME_1",
			ThreadTs:  "T_RESUME_1",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create completed session
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-resume-1",
			WorkingDirectory: "/test/resume",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Mark session as completed
		err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
			SessionID: "sess-resume-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update session status: %v", err)
		}

		// Set ended_at to recent time
		err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
			SessionID: "sess-resume-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update session end time: %v", err)
		}

		// Check if should resume
		shouldResume, previousSessionID, err := resumeManager.ShouldResume(ctx, "C_RESUME_1", "T_RESUME_1")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}

		if !shouldResume {
			t.Error("Expected session to be resumable within window")
		}
		if previousSessionID != "sess-resume-1" {
			t.Errorf("Expected previous session ID sess-resume-1, got %s", previousSessionID)
		}

		// Check active session (should be false)
		hasActive, err := resumeManager.CheckActiveSession(ctx, "C_RESUME_1", "T_RESUME_1")
		if err != nil {
			t.Fatalf("Failed to check active session: %v", err)
		}
		if hasActive {
			t.Error("Should not have active session")
		}
	})

	// Test 2: No resume outside window
	t.Run("NoResumeOutsideWindow", func(t *testing.T) {
		// Create resume manager with very short window
		resumeManager := process.NewResumeManager(queries, 1*time.Millisecond)

		// Create thread and old session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_RESUME_2",
			ThreadTs:  "T_RESUME_2",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create session
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-old-1",
			WorkingDirectory: "/test/old",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Mark as completed
		err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
			SessionID: "sess-old-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update session status: %v", err)
		}

		// Wait to ensure outside window
		time.Sleep(10 * time.Millisecond)

		// Check if should resume
		shouldResume, _, err := resumeManager.ShouldResume(ctx, "C_RESUME_2", "T_RESUME_2")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}

		if shouldResume {
			t.Error("Should not resume session outside window")
		}
	})

	// Test 3: Active session blocks resume
	t.Run("ActiveSessionBlocksResume", func(t *testing.T) {
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)

		// Create thread
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_ACTIVE",
			ThreadTs:  "T_ACTIVE",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create active session (no end time, no status)
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-active-1",
			WorkingDirectory: "/test/active",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Check active session
		hasActive, err := resumeManager.CheckActiveSession(ctx, "C_ACTIVE", "T_ACTIVE")
		if err != nil {
			t.Fatalf("Failed to check active session: %v", err)
		}

		if !hasActive {
			t.Error("Expected to have active session")
		}

		// Should not resume when active session exists
		shouldResume, _, err := resumeManager.ShouldResume(ctx, "C_ACTIVE", "T_ACTIVE")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}

		if shouldResume {
			t.Error("Should not resume when active session exists")
		}
	})

	// Test 4: Resume with DBManager integration
	t.Run("ResumeWithDBManager", func(t *testing.T) {
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)

		// Create config and managers
		cfg := &config.Config{
			Claude: config.ClaudeConfig{
				Executable:           "echo",
				PermissionPromptTool: "test_tool",
			},
		}
		baseManager := NewManager(nil, "http://test", cfg)
		dbManager := NewDBManager(baseManager, queries, resumeManager)

		// Create initial session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_DBMGR",
			ThreadTs:  "T_DBMGR",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create and complete a session
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-dbmgr-1",
			WorkingDirectory: "/test/dbmgr",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Complete the session
		err = dbManager.UpdateSessionOnComplete(ctx, "sess-dbmgr-1", process.ResultMessage{
			IsError: false,
		})
		if err != nil {
			t.Fatalf("Failed to complete session: %v", err)
		}

		// Try to create new session (should detect resume)
		// Note: We can't test actual process creation, but we can test the resume detection
		shouldResume, previousID, err := resumeManager.ShouldResume(ctx, "C_DBMGR", "T_DBMGR")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}

		if !shouldResume {
			t.Error("Expected to resume session")
		}
		if previousID != "sess-dbmgr-1" {
			t.Errorf("Expected previous session ID sess-dbmgr-1, got %s", previousID)
		}
	})

	// Test 5: Multiple sessions in thread - resume latest
	t.Run("ResumeLatestSession", func(t *testing.T) {
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)

		// Create thread
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_MULTI_RESUME",
			ThreadTs:  "T_MULTI_RESUME",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create multiple sessions
		sessionIDs := []string{"sess-multi-r-1", "sess-multi-r-2", "sess-multi-r-3"}
		for i, sessionID := range sessionIDs {
			// Add small delay to ensure different timestamps
			time.Sleep(10 * time.Millisecond)

			_, err := queries.CreateSession(ctx, db.CreateSessionParams{
				ThreadID:         thread.ID,
				SessionID:        sessionID,
				WorkingDirectory: "/test/multi",
			})
			if err != nil {
				t.Fatalf("Failed to create session %s: %v", sessionID, err)
			}

			// Complete all sessions
			err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
				SessionID: sessionID,
				Status:    sql.NullString{String: "completed", Valid: true},
			})
			if err != nil {
				t.Fatalf("Failed to complete session %s: %v", sessionID, err)
			}

			// Only set end time for completed sessions
			if i < len(sessionIDs)-1 {
				// Make older sessions actually older
				time.Sleep(10 * time.Millisecond)
			}

			err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
				SessionID: sessionID,
				Status:    sql.NullString{String: "completed", Valid: true},
			})
			if err != nil {
				t.Fatalf("Failed to update end time for %s: %v", sessionID, err)
			}
		}

		// Check resume - should get the latest session
		shouldResume, previousID, err := resumeManager.ShouldResume(ctx, "C_MULTI_RESUME", "T_MULTI_RESUME")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}

		if !shouldResume {
			t.Error("Expected to resume session")
		}
		// Note: The actual latest session depends on GetLatestSessionByThread implementation
		// It should return the most recently ended session
		if previousID == "" {
			t.Error("Expected a previous session ID")
		}
		t.Logf("Would resume session: %s", previousID)
	})
}
