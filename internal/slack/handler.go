package slack

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
)

// Constants for image download
const (
	// MaxConcurrentDownloads is the maximum number of concurrent image downloads
	MaxConcurrentDownloads = 4
)

// Re-export tool constants for backward compatibility
const (
	ToolTodoWrite    = tools.ToolTodoWrite
	ToolBash         = tools.ToolBash
	ToolRead         = tools.ToolRead
	ToolGlob         = tools.ToolGlob
	ToolEdit         = tools.ToolEdit
	ToolMultiEdit    = tools.ToolMultiEdit
	ToolWrite        = tools.ToolWrite
	ToolLS           = tools.ToolLS
	ToolGrep         = tools.ToolGrep
	ToolWebFetch     = tools.ToolWebFetch
	ToolWebSearch    = tools.ToolWebSearch
	ToolTask         = tools.ToolTask
	ToolExitPlanMode = tools.ToolExitPlanMode
	ToolNotebookRead = tools.ToolNotebookRead
	ToolNotebookEdit = tools.ToolNotebookEdit

	// Special message types
	MessageThinking       = tools.MessageThinking
	MessageApprovalPrompt = tools.MessageApprovalPrompt
)

// Handler handles Slack events and interactions
type Handler struct {
	client             *slack.Client
	signingSecret      string
	sessionMgr         SessionManager
	approvalResponder  ApprovalResponder
	assistantUsername  string
	assistantIconEmoji string
	assistantIconURL   string
	fileUploadEnabled  bool
	imagesDir          string
	botToken           string // Store bot token for file downloads
	config             *config.Config
	botUserID          string // Store bot user ID for mention detection
}

// SessionManager interface for managing Claude Code sessions
type SessionManager interface {
	GetSessionByThread(channelID, threadTS string) (*Session, error)
	CreateSession(ctx context.Context, channelID, threadTS, workDir, initialPrompt, userID string) (bool, string, error)
	SendMessage(sessionID, message string) error
}

// ApprovalResponder interface for sending approval responses
type ApprovalResponder interface {
	SendApprovalResponse(requestID string, response mcp.ApprovalResponse) error
}

// Session represents a Claude Code session
type Session struct {
	SessionID string
	ChannelID string
	ThreadTS  string
	WorkDir   string
}

// NewHandler creates a new Slack handler
func NewHandler(cfg *config.Config, sessionMgr SessionManager, botUserID string) *Handler {
	h := &Handler{
		client:        slack.New(cfg.Slack.BotToken),
		signingSecret: cfg.Slack.SigningSecret,
		sessionMgr:    sessionMgr,
		botToken:      cfg.Slack.BotToken,
		config:        cfg,
		botUserID:     botUserID,
	}

	// Apply configuration
	h.Configure()

	return h
}

// SetApprovalResponder sets the approval responder for handling approvals
func (h *Handler) SetApprovalResponder(responder ApprovalResponder) {
	h.approvalResponder = responder
}

// SetAssistantOptions sets the display options for assistant messages
func (h *Handler) SetAssistantOptions(username, iconEmoji, iconURL string) {
	h.assistantUsername = username
	h.assistantIconEmoji = iconEmoji
	h.assistantIconURL = iconURL
}

// SetFileUploadOptions sets file upload configuration
func (h *Handler) SetFileUploadOptions(enabled bool, imagesDir string) {
	h.fileUploadEnabled = enabled
	h.imagesDir = imagesDir
}

// Configure applies configuration settings to the handler
func (h *Handler) Configure() {
	// Apply assistant options
	h.assistantUsername = h.config.Slack.Assistant.Username
	h.assistantIconEmoji = h.config.Slack.Assistant.IconEmoji
	h.assistantIconURL = h.config.Slack.Assistant.IconURL

	// Apply file upload options
	h.fileUploadEnabled = h.config.Slack.FileUpload.Enabled
	h.imagesDir = h.config.Slack.FileUpload.ImagesDir
}

// GetClient returns the Slack client
func (h *Handler) GetClient() *slack.Client {
	return h.client
}

// determineWorkDir determines the working directory for a channel
func (h *Handler) determineWorkDir(channelID string) string {
	// In single directory mode, use that directory
	if h.config.IsSingleDirectoryMode() {
		return h.config.GetSingleWorkingDirectory()
	}

	// In multi-directory mode, return empty string to indicate
	// that working directory must be explicitly selected
	return ""
}

// formatWorkingDirectory formats the working directory for display in messages
func (h *Handler) formatWorkingDirectory(workDir string) string {
	if workDir == "" {
		return ""
	}

	// In single directory mode, show only the directory name
	if h.config.IsSingleDirectoryMode() {
		return fmt.Sprintf("\nWorking directory: `%s`", filepath.Base(workDir))
	}

	// In multi directory mode, find the name from config
	dirName := ""
	for _, wd := range h.config.WorkingDirs {
		if wd.Path == workDir {
			dirName = wd.Name
			break
		}
	}

	if dirName != "" {
		return fmt.Sprintf("\nWorking directory: %s (`%s`)", dirName, workDir)
	}
	return fmt.Sprintf("\nWorking directory: `%s`", workDir)
}

// createThreadAndStartSession creates a new Slack thread and starts a Claude session
func (h *Handler) createThreadAndStartSession(channelID, workDir, prompt, userID string) {
	// Create initial message with working directory information
	var initialText strings.Builder
	initialText.WriteString(fmt.Sprintf("🚀 Starting Claude Code session\n<@%s>", userID))

	// Add working directory info
	initialText.WriteString(h.formatWorkingDirectory(workDir))

	// Add initial prompt if provided
	if prompt != "" {
		initialText.WriteString("\nInitial prompt:\n")
		initialText.WriteString(prompt)
	}

	// Post initial message to create thread
	_, threadTS, err := h.client.PostMessage(
		channelID,
		slack.MsgOptionText(initialText.String(), false),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create thread")
		return
	}

	// Create session with the selected working directory
	ctx := context.Background()
	resumed, previousSessionID, err := h.sessionMgr.CreateSession(ctx, channelID, threadTS, workDir, prompt, userID)
	if err != nil {
		h.client.PostMessage(
			channelID,
			slack.MsgOptionText(fmt.Sprintf("Failed to create session: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	// Log session creation result
	if resumed {
		log.Info().
			Str("channel_id", channelID).
			Str("thread_ts", threadTS).
			Str("previous_session_id", previousSessionID).
			Msg("resumed previous Claude session")
	} else {
		// Log new session creation
	}
}
