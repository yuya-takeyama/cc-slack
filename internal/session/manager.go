package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/messages"
	"github.com/yuya-takeyama/cc-slack/internal/process"
	ccslack "github.com/yuya-takeyama/cc-slack/internal/slack"
)

// Manager manages Claude Code sessions
type Manager struct {
	sessions        map[string]*Session
	threadToSession map[string]string // channelID:threadTS -> sessionID
	mu              sync.RWMutex
	mcpServer       *mcp.Server
	slackHandler    *ccslack.Handler
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
func (m *Manager) SetSlackHandler(handler *ccslack.Handler) {
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
func (m *Manager) GetSessionByThread(channelID, threadTS string) (*ccslack.Session, error) {
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

	return &ccslack.Session{
		SessionID: session.ID,
		ChannelID: session.ChannelID,
		ThreadTS:  session.ThreadTS,
		WorkDir:   session.WorkDir,
	}, nil
}

// CreateSession creates a new Claude Code session
func (m *Manager) CreateSession(channelID, threadTS, workDir string) (*ccslack.Session, error) {
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

	return &ccslack.Session{
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
			switch content.Type {
			case "text":
				text += content.Text + "\n"
			case "thinking":
				// Handle thinking messages
				if content.Thinking != "" {
					// Create rich text with italicized text
					elements := []slack.RichTextElement{
						slack.NewRichTextSection(
							slack.NewRichTextSectionTextElement(content.Thinking, &slack.RichTextSectionTextStyle{Italic: true}),
						),
					}
					if err := m.slackHandler.PostToolRichTextMessage(channelID, threadTS, elements, ccslack.MessageThinking); err != nil {
						fmt.Printf("Failed to post thinking to Slack: %v\n", err)
					}
				}
			case "tool_use":
				// Check if this is TodoWrite
				if content.Name == "TodoWrite" && content.Input != nil {
					// Handle TodoWrite tool
					if todosInterface, ok := content.Input["todos"]; ok {
						// Create rich text elements for todo list
						var elements []slack.RichTextElement

						if todos, ok := todosInterface.([]interface{}); ok {
							for _, todoInterface := range todos {
								if todo, ok := todoInterface.(map[string]interface{}); ok {
									content := ""
									status := ""
									priority := ""

									if c, ok := todo["content"].(string); ok {
										content = c
									}
									if s, ok := todo["status"].(string); ok {
										status = s
									}
									if p, ok := todo["priority"].(string); ok {
										priority = p
									}

									// Create text style based on priority
									var textStyle *slack.RichTextSectionTextStyle
									switch priority {
									case "high":
										// Bold for high priority
										textStyle = &slack.RichTextSectionTextStyle{Bold: true}
									case "low":
										// Italic for low priority
										textStyle = &slack.RichTextSectionTextStyle{Italic: true}
									default:
										// Normal for medium priority
										textStyle = nil
									}

									// Create rich text section for each todo item with proper emoji handling
									var sectionElements []slack.RichTextSectionElement

									switch status {
									case "completed":
										// Unicode emoji can be used as text
										sectionElements = append(sectionElements, slack.NewRichTextSectionTextElement("✅ ", nil))
									case "in_progress":
										// Unicode emoji can be used as text
										sectionElements = append(sectionElements, slack.NewRichTextSectionTextElement("▶️ ", nil))
									default: // pending
										// Slack emoji needs to use emoji element
										sectionElements = append(sectionElements, slack.NewRichTextSectionEmojiElement("ballot_box_with_check", 0, nil))
										sectionElements = append(sectionElements, slack.NewRichTextSectionTextElement(" ", nil))
									}

									// Add the todo content with priority-based styling
									sectionElements = append(sectionElements, slack.NewRichTextSectionTextElement(content, textStyle))

									elements = append(elements, slack.NewRichTextSection(sectionElements...))
								}
							}
						}

						if len(elements) == 0 {
							// Fallback if no todos
							elements = append(elements, slack.NewRichTextSection(
								slack.NewRichTextSectionTextElement("Todo list updated", nil),
							))
						}

						// Post using tool-specific rich text
						if err := m.slackHandler.PostToolRichTextMessage(channelID, threadTS, elements, ccslack.ToolTodoWrite); err != nil {
							fmt.Printf("Failed to post TodoWrite to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Bash" && content.Input != nil {
					// Extract command from input
					if cmd, ok := content.Input["command"].(string); ok {
						formattedCmd := messages.FormatBashToolMessage(cmd)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, formattedCmd, ccslack.ToolBash); err != nil {
							fmt.Printf("Failed to post Bash tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Read" && content.Input != nil {
					// Handle Read tool
					if filePath, ok := content.Input["file_path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, filePath)

						// Get optional offset and limit parameters
						offset := 0
						limit := 0
						if offsetVal, ok := content.Input["offset"].(float64); ok {
							offset = int(offsetVal)
						}
						if limitVal, ok := content.Input["limit"].(float64); ok {
							limit = int(limitVal)
						}

						message := messages.FormatReadToolMessage(relPath, offset, limit)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolRead); err != nil {
							fmt.Printf("Failed to post Read tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Glob" && content.Input != nil {
					// Handle Glob tool
					if pattern, ok := content.Input["pattern"].(string); ok {
						message := messages.FormatGlobToolMessage(pattern)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolGlob); err != nil {
							fmt.Printf("Failed to post Glob tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Grep" && content.Input != nil {
					// Handle Grep tool
					pattern, _ := content.Input["pattern"].(string)
					path, _ := content.Input["path"].(string)

					var relPath string
					if path != "" {
						// Get relative path from work directory
						relPath = m.getRelativePath(channelID, threadTS, path)
					}

					message := messages.FormatGrepToolMessage(pattern, relPath)
					// Post using tool-specific icon and username
					if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolGrep); err != nil {
						fmt.Printf("Failed to post Grep tool to Slack: %v\n", err)
					}
				} else if content.Name == "Edit" && content.Input != nil {
					// Handle Edit tool
					if filePath, ok := content.Input["file_path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, filePath)
						message := messages.FormatEditToolMessage(relPath)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolEdit); err != nil {
							fmt.Printf("Failed to post Edit tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "MultiEdit" && content.Input != nil {
					// Handle MultiEdit tool
					if filePath, ok := content.Input["file_path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, filePath)
						message := messages.FormatEditToolMessage(relPath)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolMultiEdit); err != nil {
							fmt.Printf("Failed to post MultiEdit tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Write" && content.Input != nil {
					// Handle Write tool
					if filePath, ok := content.Input["file_path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, filePath)
						message := messages.FormatWriteToolMessage(relPath)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolWrite); err != nil {
							fmt.Printf("Failed to post Write tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "LS" && content.Input != nil {
					// Handle LS tool
					if path, ok := content.Input["path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, path)
						message := messages.FormatLSToolMessage(relPath)
						// Post using tool-specific icon and username
						if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolLS); err != nil {
							fmt.Printf("Failed to post LS tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Task" && content.Input != nil {
					// Handle Task tool
					description, _ := content.Input["description"].(string)
					prompt, _ := content.Input["prompt"].(string)

					message := messages.FormatTaskToolMessage(description, prompt)
					// Post using tool-specific icon and username
					if err := m.slackHandler.PostToolMessage(channelID, threadTS, message, ccslack.ToolTask); err != nil {
						fmt.Printf("Failed to post Task tool to Slack: %v\n", err)
					}
				} else {
					// Other tools - use tool-specific display or fallback
					if err := m.slackHandler.PostToolMessage(channelID, threadTS, content.Name, content.Name); err != nil {
						fmt.Printf("Failed to post %s tool to Slack: %v\n", content.Name, err)
					}
				}
			}
		}

		if text != "" {
			return m.slackHandler.PostAssistantMessage(channelID, threadTS, text)
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
			duration := time.Duration(msg.DurationMS) * time.Millisecond
			text = messages.FormatSessionCompleteMessage(
				duration,
				msg.NumTurns,
				msg.TotalCostUSD,
				msg.Usage.InputTokens,
				msg.Usage.OutputTokens,
			)
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
				message := messages.FormatTimeoutMessage(idleMinutes, sessionID)
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

// getRelativePath converts absolute path to relative path from work directory
func (m *Manager) getRelativePath(channelID, threadTS, absolutePath string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", channelID, threadTS)
	sessionID, exists := m.threadToSession[key]
	if !exists {
		return absolutePath
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return absolutePath
	}

	relPath, err := filepath.Rel(session.WorkDir, absolutePath)
	if err != nil {
		// If relative path cannot be computed, return absolute path
		return absolutePath
	}

	return relPath
}
