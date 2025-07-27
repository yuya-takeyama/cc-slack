package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/process"
)

// DBManager extends Manager with database persistence
type DBManager struct {
	*Manager
	queries       *db.Queries
	resumeManager *process.ResumeManager
}

// NewDBManager creates a new session manager with database support
func NewDBManager(manager *Manager, queries *db.Queries, resumeManager *process.ResumeManager) *DBManager {
	return &DBManager{
		Manager:       manager,
		queries:       queries,
		resumeManager: resumeManager,
	}
}

// CreateSessionWithResume creates a new session or resumes an existing one
func (dm *DBManager) CreateSessionWithResume(ctx context.Context, channelID, threadTS, workDir string) (*Session, bool, error) {
	// Check if we should resume
	shouldResume, previousSessionID, err := dm.resumeManager.ShouldResume(ctx, channelID, threadTS)
	if err != nil {
		// Log error but continue with new session
		fmt.Printf("Failed to check resume status: %v\n", err)
	}

	// Check for active session
	hasActive, err := dm.resumeManager.CheckActiveSession(ctx, channelID, threadTS)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check active session: %w", err)
	}
	if hasActive {
		return nil, false, fmt.Errorf("already has an active session for this thread")
	}

	// Get or create thread
	thread, err := dm.getOrCreateThread(ctx, channelID, threadTS)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get or create thread: %w", err)
	}

	// Create new session with resume if applicable
	session, err := dm.createSessionInternal(ctx, channelID, threadTS, workDir, shouldResume, previousSessionID)
	if err != nil {
		return nil, false, err
	}

	// Save to database
	model := ""
	if session.Process != nil {
		model = "claude-opus-4-20250514" // Default model, will be updated from init message
	}

	dbSession, err := dm.queries.CreateSession(ctx, db.CreateSessionParams{
		ThreadID:         thread.ID,
		SessionID:        session.ID,
		WorkingDirectory: workDir,
		Model:            sql.NullString{String: model, Valid: model != ""},
	})
	if err != nil {
		// Clean up created session
		session.Process.Close()
		return nil, false, fmt.Errorf("failed to save session to database: %w", err)
	}

	// Store database ID in session (extend Session struct if needed)
	_ = dbSession

	return session, shouldResume, nil
}

// UpdateSessionOnInit updates session information when init message is received
func (dm *DBManager) UpdateSessionOnInit(ctx context.Context, sessionID string, model string) error {
	// This would be called from the system message handler
	// For now, we don't update the model after creation
	return nil
}

// UpdateSessionOnComplete updates session when it completes
func (dm *DBManager) UpdateSessionOnComplete(ctx context.Context, sessionID string, result process.ResultMessage) error {
	status := "completed"
	if result.IsError {
		status = "failed"
	}

	return dm.queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
		Status:       sql.NullString{String: status, Valid: true},
		TotalCostUsd: sql.NullFloat64{Float64: result.TotalCostUSD, Valid: true},
		InputTokens:  sql.NullInt64{Int64: int64(result.Usage.InputTokens), Valid: true},
		OutputTokens: sql.NullInt64{Int64: int64(result.Usage.OutputTokens), Valid: true},
		DurationMs:   sql.NullInt64{Int64: int64(result.DurationMS), Valid: true},
		NumTurns:     sql.NullInt64{Int64: int64(result.NumTurns), Valid: true},
		SessionID:    sessionID,
	})
}

// UpdateSessionOnTimeout updates session when it times out
func (dm *DBManager) UpdateSessionOnTimeout(ctx context.Context, sessionID string) error {
	return dm.queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
		Status:    sql.NullString{String: "timeout", Valid: true},
		SessionID: sessionID,
	})
}

// getOrCreateThread gets existing thread or creates new one
func (dm *DBManager) getOrCreateThread(ctx context.Context, channelID, threadTS string) (*db.Thread, error) {
	// Try to get existing thread
	thread, err := dm.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err == nil {
		// Update timestamp
		_ = dm.queries.UpdateThreadTimestamp(ctx, thread.ID)
		return &thread, nil
	}

	// Create new thread
	if err == sql.ErrNoRows {
		thread, err = dm.queries.CreateThread(ctx, db.CreateThreadParams{
			ChannelID: channelID,
			ThreadTs:  threadTS,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create thread: %w", err)
		}
		return &thread, nil
	}

	return nil, fmt.Errorf("failed to get thread: %w", err)
}

// createSessionInternal creates the actual session with optional resume
func (dm *DBManager) createSessionInternal(ctx context.Context, channelID, threadTS, workDir string, shouldResume bool, previousSessionID string) (*Session, error) {
	// Create process options
	opts := process.Options{
		WorkDir:    workDir,
		MCPBaseURL: dm.mcpBaseURL,
		Handlers: process.MessageHandlers{
			OnSystem:    dm.createSystemHandler(channelID, threadTS),
			OnAssistant: dm.createAssistantHandler(channelID, threadTS),
			OnUser:      dm.createUserHandler(channelID, threadTS),
			OnResult:    dm.createResultHandlerWithDB(channelID, threadTS),
			OnError:     dm.createErrorHandler(channelID, threadTS),
		},
	}

	// Add resume option if applicable
	if shouldResume && previousSessionID != "" {
		opts.ResumeSessionID = previousSessionID
	}

	// Create Claude process
	claude, err := process.NewClaudeProcess(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude process: %w", err)
	}

	// Wait a bit for session ID to be assigned
	time.Sleep(100 * time.Millisecond)
	sessionID := claude.SessionID()
	if sessionID == "" {
		// Generate a temporary session ID
		sessionID = fmt.Sprintf("temp_%d", time.Now().UnixNano())
	}

	// Create session
	session := &Session{
		ID:         sessionID,
		ChannelID:  channelID,
		ThreadTS:   threadTS,
		WorkDir:    workDir,
		Process:    claude,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	// Store session
	dm.mu.Lock()
	dm.sessions[sessionID] = session
	dm.threadToSession[fmt.Sprintf("%s:%s", channelID, threadTS)] = sessionID
	dm.lastActiveID = sessionID
	dm.mu.Unlock()

	return session, nil
}

// createResultHandlerWithDB creates result handler that updates database
func (dm *DBManager) createResultHandlerWithDB(channelID, threadTS string) func(process.ResultMessage) error {
	originalHandler := dm.createResultHandler(channelID, threadTS)
	return func(msg process.ResultMessage) error {
		// Update database
		if msg.SessionID != "" {
			ctx := context.Background()
			if err := dm.UpdateSessionOnComplete(ctx, msg.SessionID, msg); err != nil {
				fmt.Printf("Failed to update session in database: %v\n", err)
			}
		}

		// Call original handler
		return originalHandler(msg)
	}
}

// CleanupIdleSessionsWithDB extends cleanup to update database
func (dm *DBManager) CleanupIdleSessionsWithDB(maxIdleTime time.Duration) {
	dm.mu.Lock()
	sessions := make(map[string]*Session)
	for k, v := range dm.sessions {
		sessions[k] = v
	}
	dm.mu.Unlock()

	ctx := context.Background()
	now := time.Now()

	for sessionID, session := range sessions {
		if now.Sub(session.LastActive) > maxIdleTime {
			// Update database before cleanup
			if err := dm.UpdateSessionOnTimeout(ctx, sessionID); err != nil {
				fmt.Printf("Failed to update timeout in database: %v\n", err)
			}
		}
	}

	// Call original cleanup
	dm.CleanupIdleSessions(maxIdleTime)
}
