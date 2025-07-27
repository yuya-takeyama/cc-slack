package process

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
	sessionID, err := rm.GetLatestSessionID(ctx, channelID, threadTS)
	if err != nil {
		return false, "", nil
	}

	// Get session details
	session, err := rm.queries.GetSession(ctx, sessionID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get session details: %w", err)
	}

	// Check if session ended within resume window
	if !session.EndedAt.Valid {
		return false, "", nil
	}

	timeSinceEnd := time.Since(session.EndedAt.Time)
	if timeSinceEnd <= rm.resumeWindow {
		return true, sessionID, nil
	}

	return false, "", nil
}

// CheckActiveSession checks if there's already an active session for the thread
func (rm *ResumeManager) CheckActiveSession(ctx context.Context, channelID, threadTS string) (bool, error) {
	// Get thread
	thread, err := rm.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			// No thread exists yet
			return false, nil
		}
		return false, fmt.Errorf("failed to get thread: %w", err)
	}

	// Check for active sessions
	count, err := rm.queries.CountActiveSessionsByThread(ctx, thread.ID)
	if err != nil {
		return false, fmt.Errorf("failed to count active sessions: %w", err)
	}

	return count > 0, nil
}
