package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
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
			case "thinking":
				// Handle thinking messages
				if content.Thinking != "" {
					// Create rich text with thinking emoji and italicized text
					elements := []slack.RichTextElement{
						slack.NewRichTextSection(
							slack.NewRichTextSectionTextElement("🤔 ", nil),
							slack.NewRichTextSectionTextElement("Thinking: ", &slack.RichTextSectionTextStyle{Bold: true}),
							slack.NewRichTextSectionTextElement(content.Thinking, &slack.RichTextSectionTextStyle{Italic: true}),
						),
					}
					if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
						fmt.Printf("Failed to post thinking to Slack: %v\n", err)
					}
				}
			case "tool_use":
				// Check if this is TodoWrite
				if content.Name == "TodoWrite" && content.Input != nil {
					// Handle TodoWrite tool
					if todosInterface, ok := content.Input["todos"]; ok {
						// Create rich text for todo list
						elements := []slack.RichTextElement{
							slack.NewRichTextSection(
								slack.NewRichTextSectionTextElement("📋 ", nil),
								slack.NewRichTextSectionTextElement("TodoWrite", &slack.RichTextSectionTextStyle{Bold: true}),
								slack.NewRichTextSectionTextElement(":", nil),
							),
						}

						// Try to parse todos
						if todos, ok := todosInterface.([]interface{}); ok {
							// Build list elements for todos
							listElements := []slack.RichTextElement{}

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

									// Create status emoji
									statusEmoji := ""
									switch status {
									case "completed":
										statusEmoji = "✅"
									case "in_progress":
										statusEmoji = "▶️"
									default: // pending
										statusEmoji = "⏳"
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

									// Create list item as RichTextSection
									listElements = append(listElements, slack.NewRichTextSection(
										slack.NewRichTextSectionTextElement(statusEmoji+" ", nil),
										slack.NewRichTextSectionTextElement(content, textStyle),
									))
								}
							}

							// Add the list if we have any todos
							if len(listElements) > 0 {
								elements = append(elements, slack.NewRichTextList(slack.RTEListBullet, 0, listElements...))
							}
						}

						if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
							fmt.Printf("Failed to post TodoWrite to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Bash" && content.Input != nil {
					// Extract command from input
					if cmd, ok := content.Input["command"].(string); ok {
						// Create rich text with bold "Bash" and code-style command
						elements := []slack.RichTextElement{
							slack.NewRichTextSection(
								slack.NewRichTextSectionTextElement("🖥️ ", nil),
								slack.NewRichTextSectionTextElement("Bash", &slack.RichTextSectionTextStyle{Bold: true}),
								slack.NewRichTextSectionTextElement(": ", nil),
								slack.NewRichTextSectionTextElement(cmd, &slack.RichTextSectionTextStyle{Code: true}),
							),
						}
						if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
							fmt.Printf("Failed to post Bash tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Read" && content.Input != nil {
					// Handle Read tool
					if filePath, ok := content.Input["file_path"].(string); ok {
						// Get relative path from work directory
						relPath := m.getRelativePath(channelID, threadTS, filePath)
						// Create rich text with bold "Read" and code-style path
						elements := []slack.RichTextElement{
							slack.NewRichTextSection(
								slack.NewRichTextSectionTextElement("📖 ", nil),
								slack.NewRichTextSectionTextElement("Read", &slack.RichTextSectionTextStyle{Bold: true}),
								slack.NewRichTextSectionTextElement(": ", nil),
								slack.NewRichTextSectionTextElement(relPath, &slack.RichTextSectionTextStyle{Code: true}),
							),
						}
						if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
							fmt.Printf("Failed to post Read tool to Slack: %v\n", err)
						}
					}
				} else if content.Name == "Glob" && content.Input != nil {
					// Handle Glob tool
					if pattern, ok := content.Input["pattern"].(string); ok {
						// Create rich text with bold "Glob" and code-style pattern
						elements := []slack.RichTextElement{
							slack.NewRichTextSection(
								slack.NewRichTextSectionTextElement("🔍 ", nil),
								slack.NewRichTextSectionTextElement("Glob", &slack.RichTextSectionTextStyle{Bold: true}),
								slack.NewRichTextSectionTextElement(": ", nil),
								slack.NewRichTextSectionTextElement(pattern, &slack.RichTextSectionTextStyle{Code: true}),
							),
						}
						if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
							fmt.Printf("Failed to post Glob tool to Slack: %v\n", err)
						}
					}
				} else {
					// Other tools - use tool-specific emoji and format
					emoji := m.getToolEmoji(content.Name)

					// Create rich text with bold tool name
					elements := []slack.RichTextElement{
						slack.NewRichTextSection(
							slack.NewRichTextSectionTextElement(emoji+" ", nil),
							slack.NewRichTextSectionTextElement(content.Name, &slack.RichTextSectionTextStyle{Bold: true}),
						),
					}
					if err := m.slackHandler.PostRichTextToThread(channelID, threadTS, elements); err != nil {
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

// getToolEmoji returns an appropriate emoji for each tool
func (m *Manager) getToolEmoji(toolName string) string {
	emojiMap := map[string]string{
		"Edit":         "✏️",
		"MultiEdit":    "✏️",
		"Write":        "📝",
		"LS":           "📁",
		"Grep":         "🔍",
		"WebFetch":     "🌐",
		"WebSearch":    "🌎",
		"Task":         "🤖",
		"TodoWrite":    "📋",
		"ExitPlanMode": "🏁",
		"NotebookRead": "📓",
		"NotebookEdit": "📔",
	}

	if emoji, ok := emojiMap[toolName]; ok {
		return emoji
	}

	// Default emoji for unknown tools or MCP tools
	if strings.HasPrefix(toolName, "mcp__") {
		return "🔌" // Plugin emoji
	}

	return "🔧" // Default wrench emoji
}
