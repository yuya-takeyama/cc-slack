package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/richtext"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
	"golang.org/x/sync/errgroup"
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
	CreateSession(ctx context.Context, channelID, threadTS, workDir, initialPrompt string) (bool, string, error)
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

// HandleEvent handles Slack webhook events
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Verify request signature
	sv, err := slack.NewSecretsVerifier(r.Header, h.signingSecret)
	if err != nil {
		http.Error(w, "Failed to create secrets verifier", http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		http.Error(w, "Failed to verify signature", http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse event
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	// Handle URL verification
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			http.Error(w, "Failed to parse challenge", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Challenge))
		return
	}

	// Handle callback events
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			h.handleMessage(ev)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleMessage handles message events
func (h *Handler) handleMessage(event *slackevents.MessageEvent) {
	// Apply filtering
	if !h.shouldProcessMessage(event) {
		return
	}

	// Extract message text, optionally removing bot mention if it exists
	text := event.Text
	if h.config.Slack.MessageFilter.RequireMention {
		text = h.removeBotMention(text)
		if text == "" {
			return
		}
	}

	// Determine thread timestamp for session management
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	// Check if mentioned in a thread with an existing session
	if event.ThreadTimeStamp != "" {
		h.handleThreadMessageEvent(event, text)
	} else {
		h.handleNewSessionFromMessage(event, text, threadTS)
	}
}

// handleThreadMessageEvent handles message events in existing threads
func (h *Handler) handleThreadMessageEvent(event *slackevents.MessageEvent, text string) {
	// Try to find existing session
	session, err := h.sessionMgr.GetSessionByThread(event.Channel, event.ThreadTimeStamp)
	if err == nil && session != nil {
		// Process attachments directly from the event
		var imagePaths []string
		var files []slack.File
		if event.Message != nil {
			files = event.Message.Files
		}

		if h.fileUploadEnabled && len(files) > 0 {
			imagePaths = h.processMessageAttachments(event, files)
		}

		// Add image paths to the prompt if any
		if len(imagePaths) > 0 {
			text = h.appendImagePaths(text, imagePaths)
		}

		// Existing session found - send message to it
		err = h.sessionMgr.SendMessage(session.SessionID, text)
		if err != nil {
			h.client.PostMessage(
				event.Channel,
				slack.MsgOptionText(fmt.Sprintf("Failed to send message: %v", err), false),
				slack.MsgOptionTS(event.ThreadTimeStamp),
			)
		}
		return
	}

	// No existing session found - create new session
	h.handleNewSessionFromMessage(event, text, event.ThreadTimeStamp)
}

// handleNewSessionFromMessage creates a new session or resumes one for message events
func (h *Handler) handleNewSessionFromMessage(event *slackevents.MessageEvent, text string, threadTS string) {
	// In multi-directory mode, validate working directory availability
	if !h.config.IsSingleDirectoryMode() {
		// For new threads, prevent mention-based start
		if event.ThreadTimeStamp == "" {
			// Post error message with guidance
			blocks := []slack.Block{
				slack.NewSectionBlock(
					slack.NewTextBlockObject(
						slack.MarkdownType,
						":warning: *Multiple working directories are configured*\n\nPlease use the shortcut to select a working directory before starting a session.",
						false,
						false,
					),
					nil,
					nil,
				),
				slack.NewSectionBlock(
					slack.NewTextBlockObject(
						slack.MarkdownType,
						fmt.Sprintf("*How to start a session:*\n1. Type `%s` or use the shortcut menu\n2. Select a working directory\n3. Enter your initial prompt", h.config.Slack.SlashCommandName),
						false,
						false,
					),
					nil,
					nil,
				),
			}

			_, _, err := h.client.PostMessage(
				event.Channel,
				slack.MsgOptionBlocks(blocks...),
				slack.MsgOptionTS(threadTS),
			)
			if err != nil {
				fmt.Printf("Failed to post error message: %v\n", err)
			}
			return
		}

		// For existing threads, we'll let the session manager handle validation
		// It will check if the thread has a working directory stored
	}

	// Determine working directory
	workDir := h.determineWorkDir(event.Channel)

	// Process attachments if any, but defer actual download
	var hasImages bool
	var files []slack.File
	// Check files in Message field
	if event.Message != nil && len(event.Message.Files) > 0 {
		files = event.Message.Files
	}

	if h.fileUploadEnabled && len(files) > 0 {
		for _, file := range files {
			if strings.HasPrefix(file.Mimetype, "image/") {
				hasImages = true
				break
			}
		}
	}

	// Process images first if any
	var initialPrompt string = text
	if hasImages && len(files) > 0 {
		imagePaths := h.processMessageAttachments(event, files)
		if len(imagePaths) > 0 {
			// Append image paths to the initial prompt
			initialPrompt = h.appendImagePaths(text, imagePaths)
		}
	}

	// Create session with text including image paths
	ctx := context.Background()
	resumed, previousSessionID, err := h.sessionMgr.CreateSession(ctx, event.Channel, threadTS, workDir, initialPrompt)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("Failed to create session: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	// Post initial response based on whether session was resumed
	var initialMessage string
	if resumed {
		initialMessage = fmt.Sprintf("Resuming previous session `%s`...", previousSessionID)
	} else {
		initialMessage = "Starting Claude Code session..."
	}

	_, _, err = h.client.PostMessage(
		event.Channel,
		slack.MsgOptionText(initialMessage, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		fmt.Printf("Failed to post message: %v\n", err)
		return
	}

	// Image processing has already been done before session creation
}

// HandleInteraction handles Slack interactive components (buttons, etc.)
func (h *Handler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	var payload slack.InteractionCallback
	err := json.Unmarshal([]byte(r.FormValue("payload")), &payload)
	if err != nil {
		http.Error(w, "Failed to parse payload", http.StatusBadRequest)
		return
	}

	// Verify token (or use signing secret verification)
	// TODO: Implement proper verification

	// Log structured payload for debugging (JSONL format)
	if payload.Type == slack.InteractionTypeViewSubmission {
		log.Info().
			Str("type", "view_submission").
			Str("callback_id", payload.View.CallbackID).
			Interface("state_values", payload.View.State.Values).
			Msg("received view submission")
	}

	switch payload.Type {
	case slack.InteractionTypeBlockActions:
		// Handle button clicks for approval_prompt
		for _, action := range payload.ActionCallback.BlockActions {
			if strings.HasPrefix(action.ActionID, "approve_") {
				h.handleApprovalAction(&payload, action, true)
			} else if strings.HasPrefix(action.ActionID, "deny_") {
				h.handleApprovalAction(&payload, action, false)
			}
		}
	case slack.InteractionTypeViewSubmission:
		// Handle modal submissions
		if payload.View.CallbackID == "repo_modal" {
			h.handleRepoModalSubmission(w, &payload)
			return // Don't send 200 OK here, handleRepoModalSubmission will handle response
		}
		if payload.View.CallbackID == "repo_modal_single" {
			h.handleSingleDirModalSubmission(w, &payload)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleApprovalAction handles approval/denial button clicks
func (h *Handler) handleApprovalAction(payload *slack.InteractionCallback, action *slack.BlockAction, approved bool) {
	// Extract request ID from action ID
	var requestID string
	if strings.HasPrefix(action.ActionID, "approve_") {
		requestID = strings.TrimPrefix(action.ActionID, "approve_")
	} else if strings.HasPrefix(action.ActionID, "deny_") {
		requestID = strings.TrimPrefix(action.ActionID, "deny_")
	}

	// Send approval response to MCP server
	if h.approvalResponder != nil && requestID != "" {
		response := mcp.ApprovalResponse{
			Behavior: "deny",
			Message:  "Denied via Slack",
		}
		if approved {
			response.Behavior = "allow"
			response.Message = "Approved via Slack"
			// IMPORTANT: When behavior is "allow", updatedInput is required
			response.UpdatedInput = map[string]interface{}{} // Empty map for no changes
		}

		err := h.approvalResponder.SendApprovalResponse(requestID, response)
		if err != nil {
			fmt.Printf("Failed to send approval response: %v\n", err)
		}
	}

	// Update the message with enhanced status information
	h.updateApprovalMessage(payload, approved)
}

// removeBotMention removes bot mention from message text
func (h *Handler) removeBotMention(text string) string {
	return RemoveBotMentionFromText(text, h.botUserID)
}

// RemoveBotMentionFromText removes bot mention from text (pure function)
func RemoveBotMentionFromText(text string, botUserID string) string {
	if botUserID == "" {
		return text
	}

	// Remove <@BOTID> pattern
	botMention := fmt.Sprintf("<@%s>", botUserID)
	text = strings.TrimSpace(text)

	// Handle mention at the beginning
	if strings.HasPrefix(text, botMention) {
		text = strings.TrimSpace(text[len(botMention):])
	}

	// Handle mention anywhere else in the text
	text = strings.ReplaceAll(text, botMention, "")

	return strings.TrimSpace(text)
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

// PostToThread posts a message to a Slack thread
func (h *Handler) PostToThread(channelID, threadTS, text string) error {
	_, _, err := h.client.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	)
	return err
}

// PostRichTextToThread posts a rich text message to a Slack thread
func (h *Handler) PostRichTextToThread(channelID, threadTS string, elements []slack.RichTextElement) error {
	_, _, err := h.client.PostMessage(
		channelID,
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewRichTextBlock("rich_text", elements...),
		),
	)
	return err
}

// PostAssistantMessage posts a message with assistant display options
func (h *Handler) PostAssistantMessage(channelID, threadTS, text string) error {
	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	}

	// Add username if configured
	if h.assistantUsername != "" {
		options = append(options, slack.MsgOptionUsername(h.assistantUsername))
	}

	// Add icon (emoji takes precedence over URL)
	if h.assistantIconEmoji != "" {
		options = append(options, slack.MsgOptionIconEmoji(h.assistantIconEmoji))
	} else if h.assistantIconURL != "" {
		options = append(options, slack.MsgOptionIconURL(h.assistantIconURL))
	}

	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}

// PostToolMessage posts a message with tool-specific display options
func (h *Handler) PostToolMessage(channelID, threadTS, text, toolType string) error {
	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	}

	// Get tool display info
	toolInfo := tools.GetToolInfo(toolType)
	// Add username
	options = append(options, slack.MsgOptionUsername(toolInfo.Name))
	// Add icon emoji
	options = append(options, slack.MsgOptionIconEmoji(toolInfo.SlackIcon))

	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}

// PostToolRichTextMessage posts a rich text message with tool-specific display options
func (h *Handler) PostToolRichTextMessage(channelID, threadTS string, elements []slack.RichTextElement, toolType string) error {
	options := []slack.MsgOption{
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewRichTextBlock("rich_text", elements...),
		),
	}

	// Get tool display info
	toolInfo := tools.GetToolInfo(toolType)
	// Add username
	options = append(options, slack.MsgOptionUsername(toolInfo.Name))
	// Add icon emoji
	options = append(options, slack.MsgOptionIconEmoji(toolInfo.SlackIcon))

	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}

// PostApprovalRequest posts an approval request with buttons using markdown
func (h *Handler) PostApprovalRequest(channelID, threadTS, message, requestID string) error {
	// Parse the message to extract structured information
	// This is a simple parser for the current format from mcp/server.go
	info := parseApprovalMessage(message)

	// Build markdown text for the approval request
	markdownText := buildApprovalMarkdownText(info)

	// Get tool display info for permission prompt
	toolInfo := tools.GetToolInfo(MessageApprovalPrompt)

	options := []slack.MsgOption{
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject(slack.MarkdownType, markdownText, false, false),
				nil,
				nil,
			),
			slack.NewActionBlock(
				"approval_actions",
				slack.NewButtonBlockElement(
					fmt.Sprintf("approve_%s", requestID),
					"approve",
					slack.NewTextBlockObject(slack.PlainTextType, "Approve", false, false),
				).WithStyle(slack.StylePrimary),
				slack.NewButtonBlockElement(
					fmt.Sprintf("deny_%s", requestID),
					"deny",
					slack.NewTextBlockObject(slack.PlainTextType, "Deny", false, false),
				).WithStyle(slack.StyleDanger),
			),
		),
	}

	// Add username and icon
	options = append(options, slack.MsgOptionUsername(toolInfo.Name))
	options = append(options, slack.MsgOptionIconEmoji(toolInfo.SlackIcon))

	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}

// ApprovalInfo holds structured information about an approval request
type ApprovalInfo struct {
	ToolName    string
	URL         string
	Prompt      string
	Command     string
	Description string
	FilePath    string
}

// parseApprovalMessage parses the approval message from mcp/server.go to extract structured information
func parseApprovalMessage(message string) *ApprovalInfo {
	// Parse the message format from mcp/server.go:
	// For WebFetch: **Tool**: WebFetch \n **URL**: %s \n **Content**: %s
	// For Bash: **Tool**: Bash \n **Command**: %s \n **Description**: %s

	info := &ApprovalInfo{}
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "**Tool**: ") {
			info.ToolName = strings.TrimPrefix(line, "**Tool**: ")
		} else if strings.HasPrefix(line, "**URL**: ") {
			info.URL = strings.TrimPrefix(line, "**URL**: ")
		} else if strings.HasPrefix(line, "**Content**: ") {
			info.Prompt = strings.TrimPrefix(line, "**Content**: ")
		} else if strings.HasPrefix(line, "**Command**: ") {
			info.Command = strings.TrimPrefix(line, "**Command**: ")
		} else if strings.HasPrefix(line, "**Description**: ") {
			info.Description = strings.TrimPrefix(line, "**Description**: ")
		} else if strings.HasPrefix(line, "**File path**: ") {
			info.FilePath = strings.TrimPrefix(line, "**File path**: ")
		}
	}

	return info
}

// buildApprovalMarkdownText creates markdown text for approval request
func buildApprovalMarkdownText(info *ApprovalInfo) string {
	var text strings.Builder

	// Header
	text.WriteString("*Tool execution permission required*\n\n")

	if info.ToolName != "" {
		text.WriteString(fmt.Sprintf("*Tool:* %s\n", info.ToolName))
	}

	// Handle WebFetch tool
	if info.URL != "" {
		text.WriteString(fmt.Sprintf("*URL:* <%s>\n", info.URL))
	}

	if info.Prompt != "" {
		text.WriteString("*Content:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Prompt))
	}

	// Handle Bash tool
	if info.Command != "" {
		text.WriteString("*Command:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Command))
	}

	if info.Description != "" {
		if info.Command != "" {
			text.WriteString("\n")
		}
		text.WriteString("*Description:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Description))
	}

	// Handle Write tool
	if info.FilePath != "" {
		text.WriteString(fmt.Sprintf("*File path:* `%s`", info.FilePath))
	}

	return text.String()
}

// updateApprovalMessage updates the approval message with status and user information
func (h *Handler) updateApprovalMessage(payload *slack.InteractionCallback, approved bool) {
	// Preserve the original blocks and add a status block
	originalBlocks := payload.Message.Blocks.BlockSet

	// Remove the action block (last block) which contains the buttons
	if len(originalBlocks) > 0 {
		originalBlocks = originalBlocks[:len(originalBlocks)-1]
	}

	// Get the original markdown text from the first section block
	var originalText string
	if len(originalBlocks) > 0 {
		if section, ok := originalBlocks[0].(*slack.SectionBlock); ok && section.Text != nil {
			originalText = section.Text.Text
		}
	}

	// Create status markdown text
	statusText := h.buildStatusMarkdownText(payload.User.ID, approved)

	// Combine original text with status
	fullText := originalText + "\n\n" + statusText

	// Create new blocks with updated text
	newBlocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, fullText, false, false),
			nil,
			nil,
		),
	}

	// Update the message
	_, _, _, err := h.client.UpdateMessage(
		payload.Channel.ID,
		payload.Message.Timestamp,
		slack.MsgOptionBlocks(newBlocks...),
		slack.MsgOptionReplaceOriginal(payload.ResponseURL),
	)
	if err != nil {
		fmt.Printf("Failed to update message: %v\n", err)
	}
}

// buildStatusMarkdownText creates markdown text for approval status
func (h *Handler) buildStatusMarkdownText(userID string, approved bool) string {
	var statusEmoji, statusText string
	if approved {
		statusEmoji = ":white_check_mark:"
		statusText = "Approved"
	} else {
		statusEmoji = ":x:"
		statusText = "Denied"
	}

	return fmt.Sprintf("────────────────\n%s *%s* by <@%s>", statusEmoji, statusText, userID)
}

// downloadAndSaveImage downloads a Slack file and saves it locally
func (h *Handler) downloadAndSaveImage(file slack.File, imageDir string) (string, error) {
	// Generate unique filename
	ext := filepath.Ext(file.Name)
	if ext == "" {
		ext = ".jpg" // Default extension
	}

	// Keep original filename if possible
	filename := file.Name
	if filename == "" || filename == "image"+ext {
		// Generate a meaningful name if original is generic
		filename = fmt.Sprintf("%s-%s%s", file.ID, time.Now().Format("20060102-150405"), ext)
	}

	filePath := filepath.Join(imageDir, filename)

	// Download the file
	var downloadURL string
	if file.URLPrivateDownload != "" {
		downloadURL = file.URLPrivateDownload
	} else if file.URLPrivate != "" {
		downloadURL = file.URLPrivate
	} else {
		return "", fmt.Errorf("no download URL available for file %s", file.Name)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+h.botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read error response body for debugging
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error response body: %s\n", string(body))
		return "", fmt.Errorf("failed to download file: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Check if we got HTML instead of an image
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		return "", fmt.Errorf("received HTML instead of image, likely authentication issue - missing files:read scope?")
	}

	// Save to file
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath, nil // Return relative path if abs fails
	}

	return absPath, nil
}

// appendImagePaths appends image paths to the prompt
func (h *Handler) appendImagePaths(text string, imagePaths []string) string {
	if len(imagePaths) == 0 {
		return text
	}

	var builder strings.Builder
	builder.WriteString(text)
	builder.WriteString("\n\n# Images attached with the message\n")

	for i, path := range imagePaths {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, path))
	}

	builder.WriteString("\n**IMPORTANT: Please read and analyze these images as they are part of the context for this message. Consider their content when formulating your response.**")

	return builder.String()
}

// shouldProcessMessage filters message events based on configuration
func (h *Handler) shouldProcessMessage(event *slackevents.MessageEvent) bool {
	// If filtering is disabled, process all messages
	if !h.config.Slack.MessageFilter.Enabled {
		return true
	}

	// Skip bot messages
	if event.BotID != "" {
		return false
	}

	// Skip subtypes (edits, deletes, etc.) but allow file_share
	if event.SubType != "" && event.SubType != "file_share" {
		return false
	}

	// Check if bot mention is required
	if h.config.Slack.MessageFilter.RequireMention {
		if !h.containsBotMention(event.Text) {
			return false
		}
	}

	// Check include patterns
	if len(h.config.Slack.MessageFilter.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range h.config.Slack.MessageFilter.IncludePatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(event.Text) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	if len(h.config.Slack.MessageFilter.ExcludePatterns) > 0 {
		for _, pattern := range h.config.Slack.MessageFilter.ExcludePatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(event.Text) {
				return false
			}
		}
	}

	return true
}

// containsBotMention checks if the text contains a mention of the bot
func (h *Handler) containsBotMention(text string) bool {
	if h.botUserID == "" {
		return false
	}
	return strings.Contains(text, fmt.Sprintf("<@%s>", h.botUserID))
}

// processMessageAttachments processes attachments directly from message event
func (h *Handler) processMessageAttachments(event *slackevents.MessageEvent, files []slack.File) []string {
	// Create session-specific directory structure
	// Format: images/{thread_ts}/{uuid}/
	sessionID := uuid.New().String()
	sessionDir := event.ThreadTimeStamp
	if sessionDir == "" {
		sessionDir = event.TimeStamp
	}
	sessionDir = strings.ReplaceAll(sessionDir, ".", "_")

	imageDir := filepath.Join(h.imagesDir, sessionDir, sessionID)
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		fmt.Printf("Failed to create image directory: %v\n", err)
		return nil
	}

	// Filter image files
	var imageFiles []slack.File
	for _, file := range files {
		if strings.HasPrefix(file.Mimetype, "image/") {
			imageFiles = append(imageFiles, file)
		}
	}

	if len(imageFiles) == 0 {
		return nil
	}

	// Result structure to maintain order
	type result struct {
		path string
		idx  int
	}

	resultChan := make(chan result, len(imageFiles))
	errorChan := make(chan error, len(imageFiles))

	// Create a worker pool with limited concurrency
	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(MaxConcurrentDownloads)

	// Start download goroutines
	for i, file := range imageFiles {
		i, file := i, file // Capture loop variables
		g.Go(func() error {
			imagePath, err := h.downloadAndSaveImage(slack.File{
				ID:                 file.ID,
				Name:               file.Name,
				Mimetype:           file.Mimetype,
				URLPrivate:         file.URLPrivate,
				URLPrivateDownload: file.URLPrivateDownload,
			}, imageDir)

			if err != nil {
				errorChan <- fmt.Errorf("failed to download %s: %w", file.Name, err)
				return nil // Don't fail the whole group
			}

			resultChan <- result{path: imagePath, idx: i}
			return nil
		})
	}

	// Wait for all downloads to complete
	if err := g.Wait(); err != nil {
		return nil
	}

	close(resultChan)
	close(errorChan)

	// Log any errors
	for err := range errorChan {
		fmt.Printf("Download error: %v\n", err)
	}

	// Collect results in order
	results := make([]string, 0, len(imageFiles))
	pathsByIdx := make(map[int]string)
	for res := range resultChan {
		pathsByIdx[res.idx] = res.path
	}

	// Sort by original order
	for i := 0; i < len(imageFiles); i++ {
		if path, ok := pathsByIdx[i]; ok {
			results = append(results, path)
		}
	}

	return results
}

// HandleSlashCommand handles Slack slash commands (e.g., /cc)
func (h *Handler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	// Verify request signature
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	sv, err := slack.NewSecretsVerifier(r.Header, h.signingSecret)
	if err != nil {
		http.Error(w, "Failed to create secrets verifier", http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		http.Error(w, "Failed to verify signature", http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse slash command
	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		http.Error(w, "Failed to parse slash command", http.StatusBadRequest)
		return
	}

	// Log the command for debugging
	log.Info().
		Str("command", cmd.Command).
		Str("text", cmd.Text).
		Str("user_id", cmd.UserID).
		Str("channel_id", cmd.ChannelID).
		Msg("received slash command")

	// Handle /cc command
	if cmd.Command == "/cc" {
		// Open modal asynchronously
		go h.openRepoModal(cmd.TriggerID, cmd.ChannelID, cmd.UserID, cmd.Text)

		// Return 200 immediately
		w.WriteHeader(http.StatusOK)
		return
	}

	// Unknown command
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Unknown command")
}

// openRepoModal opens the working directory selection modal
func (h *Handler) openRepoModal(triggerID, channelID, userID, initialText string) {
	// In single directory mode, show modal with only prompt input
	if h.config.IsSingleDirectoryMode() {
		modal := slack.ModalViewRequest{
			Type:            slack.VTModal,
			CallbackID:      "repo_modal_single",
			Title:           slack.NewTextBlockObject(slack.PlainTextType, "Start Claude Session", false, false),
			Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Start", false, false),
			Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
			PrivateMetadata: channelID, // Store channel ID for later use
			Blocks: slack.Blocks{
				BlockSet: []slack.Block{
					slack.NewInputBlock(
						"prompt_block",
						slack.NewTextBlockObject(slack.PlainTextType, "Initial prompt", false, false),
						nil,
						slack.NewRichTextInputBlockElement(
							slack.NewTextBlockObject(slack.PlainTextType, "What would you like to work on? You can use **bold**, `code`, lists, etc.", false, false),
							"prompt_input",
						),
					),
				},
			},
		}

		// Open modal
		_, err := h.client.OpenView(triggerID, modal)
		if err != nil {
			log.Error().Err(err).Msg("failed to open modal")
		}
		return
	}

	// Multi-directory mode: Build options from configured working directories
	var options []*slack.OptionBlockObject

	// Add configured working directories
	for _, wd := range h.config.WorkingDirectories {
		descText := wd.Name
		if wd.Description != "" {
			descText = fmt.Sprintf("%s - %s", wd.Name, wd.Description)
		}
		options = append(options, slack.NewOptionBlockObject(
			wd.Path,
			slack.NewTextBlockObject(slack.PlainTextType, descText, false, false),
			nil,
		))
	}

	// Create modal view
	modal := slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      "repo_modal",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Start Claude Session", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Start", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		PrivateMetadata: channelID, // Store channel ID for later use
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewInputBlock(
					"repo_block",
					slack.NewTextBlockObject(slack.PlainTextType, "Select working directory", false, false),
					nil,
					slack.NewOptionsSelectBlockElement(
						slack.OptTypeStatic,
						slack.NewTextBlockObject(slack.PlainTextType, "Choose directory", false, false),
						"repo_select",
						options...,
					),
				),
				slack.NewInputBlock(
					"prompt_block",
					slack.NewTextBlockObject(slack.PlainTextType, "Initial prompt", false, false),
					nil,
					slack.NewRichTextInputBlockElement(
						slack.NewTextBlockObject(slack.PlainTextType, "What would you like to work on? You can use **bold**, `code`, lists, etc.", false, false),
						"prompt_input",
					),
				),
			},
		},
	}

	// Set initial text if provided
	if initialText != "" {
		// TODO: Pre-populate the rich text input with initialText
	}

	// Open modal
	_, err := h.client.OpenView(triggerID, modal)
	if err != nil {
		log.Error().Err(err).Msg("failed to open modal")
	}
}

// handleRepoModalSubmission handles the working directory selection modal submission
func (h *Handler) handleRepoModalSubmission(w http.ResponseWriter, payload *slack.InteractionCallback) {
	values := payload.View.State.Values

	// Extract selected repository path
	repoPath := ""
	if repoBlock, ok := values["repo_block"]; ok {
		if repoSelect, ok := repoBlock["repo_select"]; ok {
			if repoSelect.SelectedOption.Value != "" {
				repoPath = repoSelect.SelectedOption.Value
			}
		}
	}

	// Extract initial prompt from rich text input
	var prompt string
	if promptBlock, ok := values["prompt_block"]; ok {
		if promptInput, ok := promptBlock["prompt_input"]; ok {
			// Extract text from rich text value
			// The RichTextValue field contains the actual rich text data
			if promptInput.RichTextValue.Elements != nil {
				prompt = h.convertRichTextToString(&promptInput.RichTextValue)
			}
		}
	}

	// Validation
	if repoPath == "" {
		// Return error response
		errorResponse := map[string]interface{}{
			"response_action": "errors",
			"errors": map[string]string{
				"repo_block": "Please select a working directory",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// Success - close modal
	successResponse := map[string]interface{}{
		"response_action": "clear",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(successResponse)

	// Get channel ID from private metadata (stored during modal creation)
	channelID := payload.View.PrivateMetadata
	if channelID == "" {
		log.Error().Msg("channel ID not found in private metadata")
		return
	}

	// Create thread and start session asynchronously
	go h.createThreadAndStartSession(channelID, repoPath, prompt, payload.User.ID)
}

// handleSingleDirModalSubmission handles the modal submission in single directory mode
func (h *Handler) handleSingleDirModalSubmission(w http.ResponseWriter, payload *slack.InteractionCallback) {
	values := payload.View.State.Values

	// Extract initial prompt from rich text input
	var prompt string
	if promptBlock, ok := values["prompt_block"]; ok {
		if promptInput, ok := promptBlock["prompt_input"]; ok {
			// Extract text from rich text value
			if promptInput.RichTextValue.Elements != nil {
				prompt = h.convertRichTextToString(&promptInput.RichTextValue)
			}
		}
	}

	// Success - close modal
	successResponse := map[string]interface{}{
		"response_action": "clear",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(successResponse)

	// Get channel ID from private metadata (stored during modal creation)
	channelID := payload.View.PrivateMetadata
	if channelID == "" {
		log.Error().Msg("channel ID not found in private metadata (single mode)")
		return
	}

	// Use the configured single working directory
	go h.createThreadAndStartSession(channelID, h.config.GetSingleWorkingDirectory(), prompt, payload.User.ID)
}

// convertRichTextToString converts Slack rich text to plain string
func (h *Handler) convertRichTextToString(richText *slack.RichTextBlock) string {
	// Using the richtext package for conversion
	// Import will be added automatically by goimports
	return richtext.ConvertToString(richText)
}

// createThreadAndStartSession creates a new Slack thread and starts a Claude session
func (h *Handler) createThreadAndStartSession(channelID, workDir, prompt, userID string) {
	// Create initial message with working directory information
	var initialText strings.Builder
	initialText.WriteString("🚀 Starting Claude Code session\n")

	// Add working directory info
	if workDir != "" {
		// In single directory mode, show only the directory name
		if h.config.IsSingleDirectoryMode() {
			initialText.WriteString(fmt.Sprintf("\n📁 Working directory: `%s`", filepath.Base(workDir)))
		} else {
			// In multi directory mode, find the name from config
			dirName := ""
			for _, wd := range h.config.WorkingDirectories {
				if wd.Path == workDir {
					dirName = wd.Name
					break
				}
			}
			if dirName != "" {
				initialText.WriteString(fmt.Sprintf("\n📁 Working directory: %s (`%s`)", dirName, workDir))
			} else {
				initialText.WriteString(fmt.Sprintf("\n📁 Working directory: `%s`", workDir))
			}
		}
	}

	// Add initial prompt if provided
	if prompt != "" {
		initialText.WriteString("\n💬 Initial prompt:\n")
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
	resumed, previousSessionID, err := h.sessionMgr.CreateSession(ctx, channelID, threadTS, workDir, prompt)
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
		log.Info().
			Str("channel_id", channelID).
			Str("thread_ts", threadTS).
			Str("working_dir", workDir).
			Msg("started new Claude session")
	}
}
