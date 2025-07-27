package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/process"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

// Manager manages Claude Code sessions
type Manager struct {
	sessions        map[string]*Session
	threadToSession map[string]string // channelID:threadTS -> sessionID
	mu              sync.RWMutex
	mcpServer       *mcp.Server
	slackHandler    *slack.Handler
	mcpBaseURL      string
	lastActiveID    string // Track the last active session for approval prompts
}

// Session represents an active Claude Code session
type Session struct {
	ID         string
	ChannelID  string
	ThreadTS   string
	WorkDir    string
	Process    *process.ClaudeProcess
	CreatedAt  time.Time
	LastActive time.Time
}

// NewManager creates a new session manager
func NewManager(mcpServer *mcp.Server, mcpBaseURL string) *Manager {
	return &Manager{
		sessions:        make(map[string]*Session),
		threadToSession: make(map[string]string),
		mcpServer:       mcpServer,
		mcpBaseURL:      mcpBaseURL,
	}
}

// SetSlackHandler sets the Slack handler for posting messages
func (m *Manager) SetSlackHandler(handler *slack.Handler) {
	m.slackHandler = handler
}

// GetSessionInfo returns channel and thread information for a session ID
func (m *Manager) GetSessionInfo(sessionID string) (channelID, threadTS string, exists bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If sessionID is empty, use the last active session
	if sessionID == "" && m.lastActiveID != "" {
		sessionID = m.lastActiveID
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return "", "", false
	}

	return session.ChannelID, session.ThreadTS, true
}

// GetSessionByThread retrieves a session by channel and thread timestamp
func (m *Manager) GetSessionByThread(channelID, threadTS string) (*slack.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", channelID, threadTS)
	sessionID, exists := m.threadToSession[key]
	if !exists {
		return nil, fmt.Errorf("session not found for thread")
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	return &slack.Session{
		SessionID: session.ID,
		ChannelID: session.ChannelID,
		ThreadTS:  session.ThreadTS,
		WorkDir:   session.WorkDir,
	}, nil
}

// CreateSession creates a new Claude Code session
func (m *Manager) CreateSession(channelID, threadTS, workDir string) (*slack.Session, error) {
	ctx := context.Background()

	// Create Claude process with message handlers
	claude, err := process.NewClaudeProcess(ctx, process.Options{
		WorkDir:    workDir,
		MCPBaseURL: m.mcpBaseURL,
		Handlers: process.MessageHandlers{
			OnSystem:    m.createSystemHandler(channelID, threadTS),
			OnAssistant: m.createAssistantHandler(channelID, threadTS),
			OnUser:      m.createUserHandler(channelID, threadTS),
			OnResult:    m.createResultHandler(channelID, threadTS),
			OnError:     m.createErrorHandler(channelID, threadTS),
		},
	})
	if err != nil {
		return nil, err
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
	m.mu.Lock()
	m.sessions[sessionID] = session
	m.threadToSession[fmt.Sprintf("%s:%s", channelID, threadTS)] = sessionID
	m.lastActiveID = sessionID // Track as last active
	m.mu.Unlock()

	return &slack.Session{
		SessionID: session.ID,
		ChannelID: session.ChannelID,
		ThreadTS:  session.ThreadTS,
		WorkDir:   session.WorkDir,
	}, nil
}

// SendMessage sends a message to a Claude Code session
func (m *Manager) SendMessage(sessionID, message string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if exists {
		// Update last active time and track as last active
		session.LastActive = time.Now()
		m.lastActiveID = sessionID
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Send message to Claude process
	return session.Process.SendMessage(message)
}

// Message handlers
func (m *Manager) createSystemHandler(channelID, threadTS string) func(process.SystemMessage) error {
	return func(msg process.SystemMessage) error {
		if msg.Subtype == "init" {
			// Update session ID if it was temporary
			if msg.SessionID != "" {
				m.updateSessionID(channelID, threadTS, msg.SessionID)
			}

			text := fmt.Sprintf("🚀 Claude Code セッション開始\n"+
				"セッションID: %s\n"+
				"作業ディレクトリ: %s\n"+
				"モデル: %s",
				msg.SessionID, msg.CWD, msg.Model)

			return m.slackHandler.PostToThread(channelID, threadTS, text)
		}
		return nil
	}
}

func (m *Manager) createAssistantHandler(channelID, threadTS string) func(process.AssistantMessage) error {
	return func(msg process.AssistantMessage) error {
		var text string

		for _, content := range msg.Message.Content {
			switch content.Type {
			case "text":
				text += content.Text + "\n"
			case "tool_use":
				text += fmt.Sprintf("🔧 *%s* を実行中...\n", content.Name)
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
		// Tool results are usually not shown to Slack
		// Can be enabled for debugging
		return nil
	}
}

func (m *Manager) createResultHandler(channelID, threadTS string) func(process.ResultMessage) error {
	return func(msg process.ResultMessage) error {
		var text string

		if msg.IsError {
			text = fmt.Sprintf("❌ エラーが発生しました: %s", msg.Result)
		} else {
			text = fmt.Sprintf("✅ セッション完了\n"+
				"実行時間: %dms\n"+
				"ターン数: %d\n"+
				"コスト: $%.6f USD\n"+
				"使用トークン: 入力=%d, 出力=%d",
				msg.DurationMS,
				msg.NumTurns,
				msg.TotalCostUSD,
				msg.Usage.InputTokens,
				msg.Usage.OutputTokens)

			// Cost warning
			if msg.TotalCostUSD > 1.0 {
				text += "\n⚠️ 高コストセッション"
			}
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

// updateSessionID updates a temporary session ID with the real one
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

	// Update session ID
	delete(m.sessions, oldSessionID)
	session.ID = newSessionID
	m.sessions[newSessionID] = session
	m.threadToSession[key] = newSessionID

	// Update lastActiveID if it was the old session
	if m.lastActiveID == oldSessionID {
		m.lastActiveID = newSessionID
	}
}

// CleanupIdleSessions removes sessions that have been idle for too long
func (m *Manager) CleanupIdleSessions(maxIdleTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sessionID, session := range m.sessions {
		if now.Sub(session.LastActive) > maxIdleTime {
			// Notify Slack about timeout
			if m.slackHandler != nil {
				idleMinutes := int(now.Sub(session.LastActive).Minutes())
				message := fmt.Sprintf("⏰ セッションがタイムアウトしました\n"+
					"アイドル時間: %d分\n"+
					"セッションID: %s\n\n"+
					"新しいセッションを開始するには、再度メンションしてください。",
					idleMinutes, sessionID)
				m.slackHandler.PostToThread(session.ChannelID, session.ThreadTS, message)
			}

			// Close Claude process
			session.Process.Close()

			// Remove from maps
			delete(m.sessions, sessionID)
			key := fmt.Sprintf("%s:%s", session.ChannelID, session.ThreadTS)
			delete(m.threadToSession, key)

			// Clear lastActiveID if it was this session
			if m.lastActiveID == sessionID {
				m.lastActiveID = ""
			}
		}
	}
}
