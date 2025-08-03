package process

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yuya-takeyama/cc-slack/internal/db"
)

// ResumeManager manages session resume functionality
type ResumeManager struct {
	queries *db.Queries
}

// NewResumeManager creates a new ResumeManager
func NewResumeManager(queries *db.Queries) *ResumeManager {
	return &ResumeManager{
		queries: queries,
	}
}

// GetLatestSessionID returns the latest completed session ID for a given channel and thread
func (rm *ResumeManager) GetLatestSessionID(ctx context.Context, channelID, threadTS string) (string, error) {
	// 1. Get thread_id from threads table
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

	// 2. Get latest completed session from sessions table
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

	// Check if session has ended (active sessions should not be resumed)
	if !session.EndedAt.Valid {
		return false, "", nil
	}

	// Always resume if a previous session exists, regardless of time
	return true, sessionID, nil
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
