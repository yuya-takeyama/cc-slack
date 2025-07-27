package process

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/db"
)

// ResumeManager manages session resume functionality
type ResumeManager struct {
	queries      *db.Queries
	resumeWindow time.Duration
}

// NewResumeManager creates a new ResumeManager
func NewResumeManager(queries *db.Queries, resumeWindow time.Duration) *ResumeManager {
	return &ResumeManager{
		queries:      queries,
		resumeWindow: resumeWindow,
	}
}

// logResumeDebug logs debug information for resume functionality
func (rm *ResumeManager) logResumeDebug(component, message string, fields map[string]interface{}) {
	// Create or append to the shared resume debug log file
	logDir := "logs"
	logPath := filepath.Join(logDir, "resume-debug-shared.log")

	// Ensure log directory exists
	os.MkdirAll(logDir, 0755)

	// Open file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open resume debug log: %v\n", err)
		return
	}
	defer file.Close()

	// Create logger for this write
	logger := zerolog.New(file).With().
		Timestamp().
		Str("component", component).
		Logger()

	// Create log event
	event := logger.Info()
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			event = event.Str(k, val)
		case error:
			if val != nil {
				event = event.Err(val)
			}
		case bool:
			event = event.Bool(k, val)
		case time.Time:
			event = event.Time(k, val)
		case time.Duration:
			event = event.Dur(k, val)
		default:
			event = event.Interface(k, val)
		}
	}

	event.Msg(message)
}

// GetLatestSessionID returns the latest completed session ID for a given channel and thread
func (rm *ResumeManager) GetLatestSessionID(ctx context.Context, channelID, threadTS string) (string, error) {
	// 1. threads テーブルから thread_id を取得
	thread, err := rm.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("thread not found for channel %s, thread %s", channelID, threadTS)
		}
		return "", fmt.Errorf("failed to get thread: %w", err)
	}

	// 2. sessions テーブルから最新の completed セッションを取得
	session, err := rm.queries.GetLatestSessionByThread(ctx, thread.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no completed sessions found for thread")
		}
		return "", fmt.Errorf("failed to get latest session: %w", err)
	}

	return session.SessionID, nil
}

// ShouldResume checks if a session should be resumed
func (rm *ResumeManager) ShouldResume(ctx context.Context, channelID, threadTS string) (bool, string, error) {
	rm.logResumeDebug("resume_manager", "ShouldResume called", map[string]interface{}{
		"channel_id": channelID,
		"thread_ts":  threadTS,
	})

	sessionID, err := rm.GetLatestSessionID(ctx, channelID, threadTS)
	if err != nil {
		// No previous session found
		rm.logResumeDebug("resume_manager", "No previous session found", map[string]interface{}{
			"error": err,
		})
		return false, "", nil
	}

	rm.logResumeDebug("resume_manager", "Found previous session", map[string]interface{}{
		"session_id": sessionID,
	})

	// Get session details
	session, err := rm.queries.GetSession(ctx, sessionID)
	if err != nil {
		rm.logResumeDebug("resume_manager", "Failed to get session details", map[string]interface{}{
			"error": err,
		})
		return false, "", fmt.Errorf("failed to get session details: %w", err)
	}

	// Check if session ended within resume window
	if !session.EndedAt.Valid {
		// Session not properly ended
		rm.logResumeDebug("resume_manager", "Session not properly ended", map[string]interface{}{
			"session_id": sessionID,
		})
		return false, "", nil
	}

	timeSinceEnd := time.Since(session.EndedAt.Time)
	rm.logResumeDebug("resume_manager", "Checking resume window", map[string]interface{}{
		"session_id":     sessionID,
		"ended_at":       session.EndedAt.Time,
		"time_since_end": timeSinceEnd,
		"resume_window":  rm.resumeWindow,
		"within_window":  timeSinceEnd <= rm.resumeWindow,
	})

	if timeSinceEnd <= rm.resumeWindow {
		return true, sessionID, nil
	}

	return false, "", nil
}

// CheckActiveSession checks if there's already an active session for the thread
func (rm *ResumeManager) CheckActiveSession(ctx context.Context, channelID, threadTS string) (bool, error) {
	rm.logResumeDebug("resume_manager", "CheckActiveSession called", map[string]interface{}{
		"channel_id": channelID,
		"thread_ts":  threadTS,
	})

	// Get thread
	thread, err := rm.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			// No thread exists yet
			rm.logResumeDebug("resume_manager", "No thread exists", map[string]interface{}{
				"channel_id": channelID,
				"thread_ts":  threadTS,
			})
			return false, nil
		}
		return false, fmt.Errorf("failed to get thread: %w", err)
	}

	// Check for active sessions
	count, err := rm.queries.CountActiveSessionsByThread(ctx, thread.ID)
	if err != nil {
		return false, fmt.Errorf("failed to count active sessions: %w", err)
	}

	rm.logResumeDebug("resume_manager", "Active session count", map[string]interface{}{
		"thread_id":    thread.ID,
		"active_count": count,
		"has_active":   count > 0,
	})

	return count > 0, nil
}
