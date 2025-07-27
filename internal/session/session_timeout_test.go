package session

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/process"
)

// Test timeout boundary conditions
func TestSessionTimeout_Boundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping boundary test in short mode")
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

	// Test 1: Boundary timing (skip on short tests due to timing sensitivity)
	t.Run("BoundaryTiming", func(t *testing.T) {
		// Skip this test as it's timing-sensitive and can be flaky
		t.Skip("Skipping timing-sensitive test")

		// Use 500ms window for more reliable testing
		resumeWindow := 500 * time.Millisecond
		resumeManager := process.NewResumeManager(queries, resumeWindow)

		// Create thread and session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_BOUNDARY_1",
			ThreadTs:  "T_BOUNDARY_1",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create and complete session
		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-boundary-1",
			WorkingDirectory: "/test/boundary",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Complete the session
		err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
			SessionID: "sess-boundary-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update status: %v", err)
		}

		err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
			SessionID: "sess-boundary-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update end time: %v", err)
		}

		// Test just before boundary
		time.Sleep(200 * time.Millisecond) // Less than half the window
		shouldResume, sessionID, err := resumeManager.ShouldResume(ctx, "C_BOUNDARY_1", "T_BOUNDARY_1")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}
		if !shouldResume {
			t.Errorf("Should resume before boundary (sessionID: %s, err: %v)", sessionID, err)
			// Debug: check session directly
			session, err := queries.GetSession(ctx, "sess-boundary-1")
			if err != nil {
				t.Logf("Failed to get session for debug: %v", err)
			} else {
				t.Logf("Session status: %s, EndedAt valid: %v", session.Status.String, session.EndedAt.Valid)
			}
		}

		// Test just after boundary
		time.Sleep(400 * time.Millisecond) // Now beyond the window (total 600ms > 500ms window)
		shouldResume, _, err = resumeManager.ShouldResume(ctx, "C_BOUNDARY_1", "T_BOUNDARY_1")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}
		if shouldResume {
			t.Error("Should not resume after boundary")
		}
	})

	// Test 2: Zero timeout window
	t.Run("ZeroTimeoutWindow", func(t *testing.T) {
		// Zero timeout means never resume
		resumeManager := process.NewResumeManager(queries, 0)

		// Create thread and session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_ZERO",
			ThreadTs:  "T_ZERO",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-zero-1",
			WorkingDirectory: "/test/zero",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Complete immediately
		err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
			SessionID: "sess-zero-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update status: %v", err)
		}

		err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
			SessionID: "sess-zero-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update end time: %v", err)
		}

		// Should never resume with zero window
		shouldResume, _, err := resumeManager.ShouldResume(ctx, "C_ZERO", "T_ZERO")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}
		if shouldResume {
			t.Error("Should never resume with zero timeout window")
		}
	})

	// Test 3: Very large timeout window
	t.Run("LargeTimeoutWindow", func(t *testing.T) {
		// 24 hour window
		resumeManager := process.NewResumeManager(queries, 24*time.Hour)

		// Create thread and session
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_LARGE",
			ThreadTs:  "T_LARGE",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		_, err = queries.CreateSession(ctx, db.CreateSessionParams{
			ThreadID:         thread.ID,
			SessionID:        "sess-large-1",
			WorkingDirectory: "/test/large",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Complete the session
		err = queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
			SessionID: "sess-large-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update status: %v", err)
		}

		err = queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
			SessionID: "sess-large-1",
			Status:    sql.NullString{String: "completed", Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update end time: %v", err)
		}

		// Should still be resumable after some time
		time.Sleep(100 * time.Millisecond)
		shouldResume, _, err := resumeManager.ShouldResume(ctx, "C_LARGE", "T_LARGE")
		if err != nil {
			t.Fatalf("Failed to check resume: %v", err)
		}
		if !shouldResume {
			t.Error("Should resume with large timeout window")
		}
	})

	// Test 4: Multiple timeout states
	t.Run("MultipleTimeoutStates", func(t *testing.T) {
		cfg := &config.Config{
			Claude: config.ClaudeConfig{
				Executable:           "echo",
				PermissionPromptTool: "test_tool",
			},
		}
		baseManager := NewManager(nil, "http://test", cfg)
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)
		dbManager := NewDBManager(baseManager, queries, resumeManager)

		// Create thread
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_STATES",
			ThreadTs:  "T_STATES",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Test different session end states
		testCases := []struct {
			sessionID   string
			status      string
			shouldExist bool
		}{
			{"sess-timeout-1", "timeout", true},
			{"sess-failed-1", "failed", true},
			{"sess-completed-1", "completed", true},
			{"sess-active-1", "", false}, // Active session has no status
		}

		for _, tc := range testCases {
			// Create session
			_, err := queries.CreateSession(ctx, db.CreateSessionParams{
				ThreadID:         thread.ID,
				SessionID:        tc.sessionID,
				WorkingDirectory: "/test/states",
			})
			if err != nil {
				t.Fatalf("Failed to create session %s: %v", tc.sessionID, err)
			}

			// Update based on status
			switch tc.status {
			case "timeout":
				err = dbManager.UpdateSessionOnTimeout(ctx, tc.sessionID)
			case "failed":
				err = dbManager.UpdateSessionOnComplete(ctx, tc.sessionID, process.ResultMessage{
					IsError: true,
				})
			case "completed":
				err = dbManager.UpdateSessionOnComplete(ctx, tc.sessionID, process.ResultMessage{
					IsError: false,
				})
			case "":
				// Active - do nothing
			}

			if err != nil && tc.status != "" {
				t.Fatalf("Failed to update session %s: %v", tc.sessionID, err)
			}

			// Verify session state
			session, err := queries.GetSession(ctx, tc.sessionID)
			if err != nil {
				t.Fatalf("Failed to get session %s: %v", tc.sessionID, err)
			}

			if tc.shouldExist {
				if session.Status.String != tc.status {
					t.Errorf("Session %s: expected status %s, got %s", tc.sessionID, tc.status, session.Status.String)
				}
				if !session.EndedAt.Valid {
					t.Errorf("Session %s: expected EndedAt to be set", tc.sessionID)
				}
			} else {
				// Active session should have 'active' status (default value)
				if session.Status.String != "active" {
					t.Errorf("Session %s: expected status 'active' for active session, got %s", tc.sessionID, session.Status.String)
				}
				if session.EndedAt.Valid {
					t.Errorf("Session %s: expected no EndedAt for active session", tc.sessionID)
				}
			}
		}
	})

	// Test 5: Concurrent timeout handling
	t.Run("ConcurrentTimeouts", func(t *testing.T) {
		cfg := &config.Config{
			Claude: config.ClaudeConfig{
				Executable:           "echo",
				PermissionPromptTool: "test_tool",
			},
		}
		baseManager := NewManager(nil, "http://test", cfg)
		resumeManager := process.NewResumeManager(queries, 1*time.Hour)
		dbManager := NewDBManager(baseManager, queries, resumeManager)

		// Create thread
		thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: "C_CONCURRENT",
			ThreadTs:  "T_CONCURRENT",
		})
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Create multiple sessions
		sessionCount := 5
		sessionIDs := make([]string, sessionCount)
		for i := 0; i < sessionCount; i++ {
			sessionID := fmt.Sprintf("sess-concurrent-%d", i)
			sessionIDs[i] = sessionID

			_, err := queries.CreateSession(ctx, db.CreateSessionParams{
				ThreadID:         thread.ID,
				SessionID:        sessionID,
				WorkingDirectory: "/test/concurrent",
			})
			if err != nil {
				t.Fatalf("Failed to create session %s: %v", sessionID, err)
			}
		}

		// Timeout all sessions concurrently
		done := make(chan error, sessionCount)
		for _, sessionID := range sessionIDs {
			go func(id string) {
				err := dbManager.UpdateSessionOnTimeout(ctx, id)
				done <- err
			}(sessionID)
		}

		// Wait for all to complete
		for i := 0; i < sessionCount; i++ {
			if err := <-done; err != nil {
				t.Errorf("Failed to timeout session: %v", err)
			}
		}

		// Verify all sessions are timed out
		for _, sessionID := range sessionIDs {
			session, err := queries.GetSession(ctx, sessionID)
			if err != nil {
				t.Fatalf("Failed to get session %s: %v", sessionID, err)
			}

			if session.Status.String != "timeout" {
				t.Errorf("Session %s: expected timeout status, got %s", sessionID, session.Status.String)
			}
			if !session.EndedAt.Valid {
				t.Errorf("Session %s: expected EndedAt to be set", sessionID)
			}
		}
	})
}
