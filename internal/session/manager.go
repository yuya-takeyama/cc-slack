package session

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuya-takeyama/cc-slack/pkg/types"
)

var sessionCounter uint64

// Manager manages Claude Code sessions
type Manager struct {
	sessions       map[string]*Session // session_id -> Session
	threadSessions map[string]*Session // thread_ts -> Session
	mu             sync.RWMutex
}

// Session represents a session with additional methods
type Session struct {
	*types.Session
}

func NewManager() *Manager {
	return &Manager{
		sessions:       make(map[string]*Session),
		threadSessions: make(map[string]*Session),
	}
}

func (m *Manager) CreateSession(channelID, workDir string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		Session: &types.Session{
			ChannelID:  channelID,
			WorkDir:    workDir,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		},
	}

	// Session ID will be set when we receive it from Claude Code
	// For now, we'll use a temporary ID
	tempID := generateTempSessionID()
	session.SessionID = tempID

	m.sessions[tempID] = session

	return session, nil
}

func (m *Manager) GetBySessionID(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

func (m *Manager) GetByThreadTS(threadTS string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.threadSessions[threadTS]
}

func (m *Manager) UpdateSessionID(oldID, newID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[oldID]; exists {
		delete(m.sessions, oldID)
		session.SessionID = newID
		m.sessions[newID] = session
	}
}

func (m *Manager) UpdateThreadTS(sessionID, threadTS string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.ThreadTS = threadTS
		m.threadSessions[threadTS] = session
	}
}

func (m *Manager) UpdateLastActive(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.LastActive = time.Now()
	}
}

func (m *Manager) RemoveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		if session.ThreadTS != "" {
			delete(m.threadSessions, session.ThreadTS)
		}
		
		// Clean up Claude process if running
		if session.Process != nil {
			if session.Process.Cmd != nil && session.Process.Cmd.Process != nil {
				session.Process.Cmd.Process.Kill()
			}
			if session.Process.Stdin != nil {
				session.Process.Stdin.Close()
			}
		}
	}
}

func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (m *Manager) CleanupIdleSessions(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sessionID, session := range m.sessions {
		if now.Sub(session.LastActive) > timeout {
			delete(m.sessions, sessionID)
			if session.ThreadTS != "" {
				delete(m.threadSessions, session.ThreadTS)
			}
			
			// Clean up Claude process
			if session.Process != nil {
				if session.Process.Cmd != nil && session.Process.Cmd.Process != nil {
					session.Process.Cmd.Process.Kill()
				}
				if session.Process.Stdin != nil {
					session.Process.Stdin.Close()
				}
			}
		}
	}
}

func generateTempSessionID() string {
	counter := atomic.AddUint64(&sessionCounter, 1)
	return fmt.Sprintf("temp-%d-%d", time.Now().UnixNano(), counter)
}