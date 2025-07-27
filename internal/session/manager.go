package session

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/messages"
	"github.com/yuya-takeyama/cc-slack/internal/process"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

// Manager manages Claude sessions with database persistence
type Manager struct {
	sessions        map[string]*Session
	threadToSession map[string]string
	lastActiveID    string
	mu              sync.Mutex

	db                *sql.DB
	queries           *db.Queries
	config            *config.Config
	slackHandler      *slack.Handler
	mcpBaseURL        string
	resumeDebugLogger *zerolog.Logger
}

// Session represents an active Claude session
type Session struct {
	ID         string
	Process    *process.ClaudeProcess
	ChannelID  string
	ThreadTS   string
	LastActive time.Time
}

// NewManager creates a new session manager
func NewManager(database *sql.DB, cfg *config.Config, slackHandler *slack.Handler, mcpBaseURL string) *Manager {
	queries := db.New(database)

	// Set up resume debug logger
	var resumeDebugLogger *zerolog.Logger
	if logFile := os.Getenv("RESUME_DEBUG_LOG"); logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logger := zerolog.New(file).With().Timestamp().Logger()
			resumeDebugLogger = &logger
			logger.Info().Msg("UnifiedManager resume debug logging initialized")
		} else {
			log.Printf("Failed to open resume debug log file: %v", err)
		}
	}

	return &Manager{
		sessions:          make(map[string]*Session),
		threadToSession:   make(map[string]string),
		db:                database,
		queries:           queries,
		config:            cfg,
		slackHandler:      slackHandler,
		mcpBaseURL:        mcpBaseURL,
		resumeDebugLogger: resumeDebugLogger,
	}
}

// CreateSessionWithResume creates a new session or resumes an existing one
// Returns: session, resumed, previousSessionID, error
func (m *Manager) CreateSessionWithResume(ctx context.Context, channelID, threadTS, workDir string) (*slack.Session, bool, string, error) {
	m.logResumeDebug("session_manager", "CreateSessionWithResume called", map[string]interface{}{
		"channel_id": channelID,
		"thread_ts":  threadTS,
		"work_dir":   workDir,
	})

	// Check if should resume
	shouldResume, previousSessionID, err := m.ShouldResume(ctx, channelID, threadTS)
	m.logResumeDebug("session_manager", "ShouldResume result", map[string]interface{}{
		"should_resume":       shouldResume,
		"previous_session_id": previousSessionID,
		"error":               err,
	})

	if err != nil {
		return nil, false, "", fmt.Errorf("failed to check resume status: %w", err)
	}

	// Check for active session
	hasActive, err := m.CheckActiveSession(ctx, channelID, threadTS)
	m.logResumeDebug("session_manager", "CheckActiveSession result", map[string]interface{}{
		"has_active": hasActive,
		"error":      err,
	})

	if err != nil {
		return nil, false, "", fmt.Errorf("failed to check active session: %w", err)
	}

	if hasActive {
		return nil, false, "", fmt.Errorf("already has an active session for this thread")
	}

	session, resumed, err := m.createSessionInternal(ctx, channelID, threadTS, workDir, shouldResume, previousSessionID)
	return session, resumed, previousSessionID, err
}

// createSessionInternal handles the actual session creation
func (m *Manager) createSessionInternal(ctx context.Context, channelID, threadTS, workDir string, shouldResume bool, previousSessionID string) (*slack.Session, bool, error) {
	// Get or create thread ID
	threadID, err := m.getOrCreateThread(ctx, channelID, threadTS)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get or create thread: %w", err)
	}

	m.logResumeDebug("session_manager", "Creating session internal", map[string]interface{}{
		"thread_id":           threadID,
		"should_resume":       shouldResume,
		"previous_session_id": previousSessionID,
	})

	// Generate temporary session ID
	tempSessionID := fmt.Sprintf("temp_%d", time.Now().UnixNano())

	// Create session in database (model will be updated from SystemMessage)
	_, err = m.queries.CreateSession(ctx, db.CreateSessionParams{
		ThreadID:         threadID,
		SessionID:        tempSessionID,
		WorkingDirectory: workDir,
		Model:            sql.NullString{Valid: false}, // Will be set from SystemMessage
	})

	if err != nil {
		return nil, false, fmt.Errorf("failed to create session in database: %w", err)
	}

	// Create Claude process
	var resumeSessionID string
	if shouldResume {
		resumeSessionID = previousSessionID
	}

	m.logResumeDebug("session_manager", "Creating Claude process", map[string]interface{}{
		"resume_session_id": resumeSessionID,
	})

	claudeProcess, err := process.NewClaudeProcess(ctx, process.Options{
		WorkDir:              workDir,
		MCPBaseURL:           m.mcpBaseURL,
		ExecutablePath:       m.config.Claude.Executable,
		PermissionPromptTool: m.config.Claude.PermissionPromptTool,
		Handlers: process.MessageHandlers{
			OnSystem:    m.createSystemHandler(channelID, threadTS, tempSessionID),
			OnAssistant: m.createAssistantHandler(channelID, threadTS),
			OnUser:      m.createUserHandler(channelID, threadTS),
			OnResult:    m.createResultHandler(channelID, threadTS, tempSessionID),
			OnError:     m.createErrorHandler(channelID, threadTS),
		},
		ResumeSessionID: resumeSessionID,
	})

	if err != nil {
		// Clean up database record on failure
		_ = m.queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
			Status:    sql.NullString{String: "failed", Valid: true},
			SessionID: tempSessionID,
		})
		return nil, false, fmt.Errorf("failed to create Claude process: %w", err)
	}

	// Create session object
	session := &Session{
		ID:         tempSessionID,
		Process:    claudeProcess,
		ChannelID:  channelID,
		ThreadTS:   threadTS,
		LastActive: time.Now(),
	}

	// Store session
	m.mu.Lock()
	m.sessions[tempSessionID] = session
	key := fmt.Sprintf("%s:%s", channelID, threadTS)
	m.threadToSession[key] = tempSessionID
	m.lastActiveID = tempSessionID
	m.mu.Unlock()

	m.logResumeDebug("session_manager", "Session created successfully", map[string]interface{}{
		"session_id": tempSessionID,
		"resumed":    shouldResume,
	})

	return &slack.Session{
		SessionID: tempSessionID,
		ChannelID: channelID,
		ThreadTS:  threadTS,
		WorkDir:   workDir,
	}, shouldResume, nil
}

// CreateSession creates a new session (compatibility method)
func (m *Manager) CreateSession(channelID, threadTS, workDir string) (*slack.Session, error) {
	ctx := context.Background()
	session, _, _, err := m.CreateSessionWithResume(ctx, channelID, threadTS, workDir)
	return session, err
}

// getOrCreateThread gets or creates a thread record
func (m *Manager) getOrCreateThread(ctx context.Context, channelID, threadTS string) (int64, error) {
	// Try to get existing thread
	thread, err := m.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err == nil {
		return thread.ID, nil
	}

	// Create new thread
	newThread, err := m.queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		return 0, err
	}

	return newThread.ID, nil
}

// updateSessionID updates the session ID from temporary to real
func (m *Manager) updateSessionID(channelID, threadTS string, newSessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", channelID, threadTS)
	oldSessionID, exists := m.threadToSession[key]
	if !exists {
		return
	}

	session, exists := m.sessions[oldSessionID]
	if !exists {
		return
	}

	// Update in memory
	delete(m.sessions, oldSessionID)
	session.ID = newSessionID
	m.sessions[newSessionID] = session
	m.threadToSession[key] = newSessionID

	if m.lastActiveID == oldSessionID {
		m.lastActiveID = newSessionID
	}

	// Update in database
	if strings.HasPrefix(oldSessionID, "temp_") {
		ctx := context.Background()

		// Update session ID
		err := m.queries.UpdateSessionID(ctx, db.UpdateSessionIDParams{
			SessionID:   newSessionID,
			SessionID_2: oldSessionID,
		})
		if err != nil {
			m.logResumeDebug("session_manager", "Failed to update session ID", map[string]interface{}{
				"old_session_id": oldSessionID,
				"new_session_id": newSessionID,
				"error":          err,
			})
			return
		}

		m.logResumeDebug("session_manager", "Session ID updated successfully", map[string]interface{}{
			"old_session_id": oldSessionID,
			"new_session_id": newSessionID,
		})
	}
}

// Message handlers
func (m *Manager) createSystemHandler(channelID, threadTS, tempSessionID string) func(process.SystemMessage) error {
	return func(msg process.SystemMessage) error {
		if msg.Subtype == "init" {
			// Update session ID if it was temporary
			if msg.SessionID != "" && msg.SessionID != tempSessionID {
				m.updateSessionID(channelID, threadTS, msg.SessionID)
			}

			// Update model information from Claude Code
			if msg.Model != "" {
				ctx := context.Background()
				sessionID := msg.SessionID
				if sessionID == "" {
					sessionID = tempSessionID
				}

				err := m.queries.UpdateSessionModel(ctx, db.UpdateSessionModelParams{
					Model:     sql.NullString{String: msg.Model, Valid: true},
					SessionID: sessionID,
				})
				if err != nil {
					m.logResumeDebug("session_manager", "Failed to update session model", map[string]interface{}{
						"session_id": sessionID,
						"model":      msg.Model,
						"error":      err,
					})
				} else {
					m.logResumeDebug("session_manager", "Session model updated", map[string]interface{}{
						"session_id": sessionID,
						"model":      msg.Model,
					})
				}
			}

			text := messages.FormatSessionStartMessage(msg.SessionID, msg.CWD, msg.Model)
			return m.slackHandler.PostToThread(channelID, threadTS, text)
		}
		return nil
	}
}

func (m *Manager) createAssistantHandler(channelID, threadTS string) func(process.AssistantMessage) error {
	return func(msg process.AssistantMessage) error {
		var text string

		for _, content := range msg.Message.Content {
			if content.Type == "text" {
				text = content.Text
				break
			}
		}

		if text != "" {
			return m.slackHandler.PostToThread(channelID, threadTS, text)
		}

		return nil
	}
}

func (m *Manager) createUserHandler(channelID, threadTS string) func(process.UserMessage) error {
	return func(msg process.UserMessage) error {
		// Update last active time
		m.mu.Lock()
		key := fmt.Sprintf("%s:%s", channelID, threadTS)
		if sessionID, exists := m.threadToSession[key]; exists {
			if session, exists := m.sessions[sessionID]; exists {
				session.LastActive = time.Now()
			}
		}
		m.mu.Unlock()
		return nil
	}
}

func (m *Manager) createResultHandler(channelID, threadTS, tempSessionID string) func(process.ResultMessage) error {
	return func(msg process.ResultMessage) error {
		m.logResumeDebug("session_manager", "ResultMessage received", map[string]interface{}{
			"session_id":   msg.SessionID,
			"is_error":     msg.IsError,
			"total_cost":   msg.TotalCostUSD,
			"num_turns":    msg.NumTurns,
			"temp_session": tempSessionID,
		})

		// Update database
		if msg.SessionID != "" {
			ctx := context.Background()

			// If session ID changed from temp, update it first
			if msg.SessionID != tempSessionID && strings.HasPrefix(tempSessionID, "temp_") {
				m.updateSessionID(channelID, threadTS, msg.SessionID)
			}

			// Update session completion status
			if err := m.UpdateSessionOnComplete(ctx, msg.SessionID, msg); err != nil {
				m.logResumeDebug("session_manager", "Failed to update session", map[string]interface{}{
					"session_id": msg.SessionID,
					"error":      err,
				})
			} else {
				m.logResumeDebug("session_manager", "Session updated successfully", map[string]interface{}{
					"session_id": msg.SessionID,
					"status": func() string {
						if msg.IsError {
							return "failed"
						}
						return "completed"
					}(),
				})
			}
		}

		// Clean up session
		m.mu.Lock()
		key := fmt.Sprintf("%s:%s", channelID, threadTS)
		sessionID := m.threadToSession[key]
		delete(m.sessions, sessionID)
		delete(m.threadToSession, key)
		m.mu.Unlock()

		// Post result message
		var text string
		if msg.IsError {
			text = messages.FormatErrorMessage(msg.SessionID)
		} else {
			text = messages.FormatCompletionMessage(msg.SessionID, msg.NumTurns, msg.TotalCostUSD)
		}

		return m.slackHandler.PostToThread(channelID, threadTS, text)
	}
}

func (m *Manager) createErrorHandler(channelID, threadTS string) func(error) {
	return func(err error) {
		text := fmt.Sprintf("⚠️ エラー: %v", err)
		m.slackHandler.PostToThread(channelID, threadTS, text)
	}
}

// SendMessage sends a message to a specific session
func (m *Manager) SendMessage(sessionID string, message string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if exists {
		session.LastActive = time.Now()
		m.lastActiveID = sessionID
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return session.Process.SendMessage(message)
}

// GetSession returns a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	return session, exists
}

// GetLastActiveSessionID returns the last active session ID
func (m *Manager) GetLastActiveSessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastActiveID
}

// CleanupIdleSessions removes sessions that have been idle for too long
func (m *Manager) CleanupIdleSessions(maxIdleTime time.Duration) {
	m.mu.Lock()
	sessions := make(map[string]*Session)
	for k, v := range m.sessions {
		sessions[k] = v
	}
	m.mu.Unlock()

	ctx := context.Background()
	now := time.Now()

	for sessionID, session := range sessions {
		if now.Sub(session.LastActive) > maxIdleTime {
			// Notify Slack about timeout
			if m.slackHandler != nil {
				idleMinutes := int(now.Sub(session.LastActive).Minutes())
				message := messages.FormatTimeoutMessage(idleMinutes, sessionID)
				m.slackHandler.PostToThread(session.ChannelID, session.ThreadTS, message)
			}

			// Update database
			_ = m.queries.UpdateSessionEndTime(ctx, db.UpdateSessionEndTimeParams{
				Status:    sql.NullString{String: "timeout", Valid: true},
				SessionID: sessionID,
			})

			// Close process and clean up
			session.Process.Close()

			m.mu.Lock()
			key := fmt.Sprintf("%s:%s", session.ChannelID, session.ThreadTS)
			delete(m.sessions, sessionID)
			delete(m.threadToSession, key)
			if m.lastActiveID == sessionID {
				m.lastActiveID = ""
			}
			m.mu.Unlock()
		}
	}
}

// Resume-related methods
func (m *Manager) ShouldResume(ctx context.Context, channelID, threadTS string) (bool, string, error) {
	m.logResumeDebug("resume_manager", "ShouldResume called", map[string]interface{}{
		"channel_id": channelID,
		"thread_ts":  threadTS,
	})

	// Get thread
	thread, err := m.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			m.logResumeDebug("resume_manager", "No previous session found", map[string]interface{}{
				"error": fmt.Errorf("thread not found for channel %s, thread %s", channelID, threadTS),
			})
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to get thread: %w", err)
	}

	// Get latest completed session
	session, err := m.queries.GetLatestSessionByThread(ctx, thread.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			m.logResumeDebug("resume_manager", "No previous session found", map[string]interface{}{
				"error": fmt.Errorf("no completed sessions found for thread"),
			})
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to get latest session: %w", err)
	}

	// Check if session ended within resume window
	if !session.EndedAt.Valid {
		m.logResumeDebug("resume_manager", "Session not properly ended", map[string]interface{}{
			"session_id": session.SessionID,
		})
		return false, "", nil
	}

	resumeWindow := m.config.Session.ResumeWindow
	timeSinceEnd := time.Since(session.EndedAt.Time)

	m.logResumeDebug("resume_manager", "Checking resume window", map[string]interface{}{
		"session_id":     session.SessionID,
		"ended_at":       session.EndedAt.Time,
		"time_since_end": timeSinceEnd,
		"resume_window":  resumeWindow,
		"within_window":  timeSinceEnd <= resumeWindow,
	})

	if timeSinceEnd <= resumeWindow {
		return true, session.SessionID, nil
	}

	return false, "", nil
}

func (m *Manager) CheckActiveSession(ctx context.Context, channelID, threadTS string) (bool, error) {
	m.logResumeDebug("resume_manager", "CheckActiveSession called", map[string]interface{}{
		"channel_id": channelID,
		"thread_ts":  threadTS,
	})

	// Get thread
	thread, err := m.queries.GetThread(ctx, db.GetThreadParams{
		ChannelID: channelID,
		ThreadTs:  threadTS,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			m.logResumeDebug("resume_manager", "No thread exists", map[string]interface{}{
				"channel_id": channelID,
				"thread_ts":  threadTS,
			})
			return false, nil
		}
		return false, fmt.Errorf("failed to get thread: %w", err)
	}

	// Check for active sessions
	count, err := m.queries.CountActiveSessionsByThread(ctx, thread.ID)
	if err != nil {
		return false, fmt.Errorf("failed to count active sessions: %w", err)
	}

	m.logResumeDebug("resume_manager", "Active session count", map[string]interface{}{
		"thread_id":    thread.ID,
		"active_count": count,
		"has_active":   count > 0,
	})

	return count > 0, nil
}

func (m *Manager) UpdateSessionOnComplete(ctx context.Context, sessionID string, result process.ResultMessage) error {
	status := "completed"
	if result.IsError {
		status = "failed"
	}

	return m.queries.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
		Status:       sql.NullString{String: status, Valid: true},
		TotalCostUsd: sql.NullFloat64{Float64: result.TotalCostUSD, Valid: true},
		InputTokens:  sql.NullInt64{Int64: int64(result.Usage.InputTokens), Valid: true},
		OutputTokens: sql.NullInt64{Int64: int64(result.Usage.OutputTokens), Valid: true},
		DurationMs:   sql.NullInt64{Int64: int64(result.DurationMS), Valid: true},
		NumTurns:     sql.NullInt64{Int64: int64(result.NumTurns), Valid: true},
		SessionID:    sessionID,
	})
}

// Helper methods
func (m *Manager) logResumeDebug(component, message string, fields map[string]interface{}) {
	if m.resumeDebugLogger != nil {
		event := m.resumeDebugLogger.Info().Str("component", component)
		for k, v := range fields {
			event = event.Interface(k, v)
		}
		event.Msg(message)
	}
}

// GetSessionByThread returns a session by channel and thread (for slack.SessionManager interface)
func (m *Manager) GetSessionByThread(channelID, threadTS string) (*slack.Session, error) {
	session, exists := m.GetSessionByThreadInternal(channelID, threadTS)
	if !exists {
		return nil, fmt.Errorf("session not found for thread %s:%s", channelID, threadTS)
	}

	return &slack.Session{
		SessionID: session.ID,
		ChannelID: session.ChannelID,
		ThreadTS:  session.ThreadTS,
		WorkDir:   "", // WorkDir not stored in memory session
	}, nil
}

// GetSessionByThreadInternal returns the internal session representation
func (m *Manager) GetSessionByThreadInternal(channelID, threadTS string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", channelID, threadTS)
	sessionID, exists := m.threadToSession[key]
	if !exists {
		return nil, false
	}

	session, exists := m.sessions[sessionID]
	return session, exists
}

// GetSessionInfo implements mcp.SessionLookup interface
func (m *Manager) GetSessionInfo(sessionID string) (channelID, threadTS string, exists bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return "", "", false
	}

	return session.ChannelID, session.ThreadTS, true
}

// Cleanup closes all active sessions
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		session.Process.Close()
	}

	m.sessions = make(map[string]*Session)
	m.threadToSession = make(map[string]string)
	m.lastActiveID = ""
}
