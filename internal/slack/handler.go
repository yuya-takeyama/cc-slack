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
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
	"golang.org/x/sync/errgroup"
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

// Constants for image download
const (
	// MaxConcurrentDownloads is the maximum number of concurrent image downloads
	MaxConcurrentDownloads = 4
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
}

// SessionManager interface for managing Claude Code sessions
type SessionManager interface {
	GetSessionByThread(channelID, threadTS string) (*Session, error)
	CreateSession(channelID, threadTS, workDir string) (*Session, error)
	CreateSessionWithResume(ctx context.Context, channelID, threadTS, workDir, initialPrompt string) (*Session, bool, string, error)
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
func NewHandler(token, signingSecret string, sessionMgr SessionManager) *Handler {
	h := &Handler{
		client:        slack.New(token),
		signingSecret: signingSecret,
		sessionMgr:    sessionMgr,
		botToken:      token,
	}

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
		case *slackevents.AppMentionEvent:
			h.handleAppMention(ev)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleAppMention handles bot mentions
func (h *Handler) handleAppMention(event *slackevents.AppMentionEvent) {
	// Extract message without mention
	text := h.removeBotMention(event.Text)
	if text == "" {
		return
	}

	// Determine thread timestamp for session management
	// If mentioned in a thread, use thread_ts; otherwise use the message ts
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	// Check if mentioned in a thread with an existing session
	if event.ThreadTimeStamp != "" {
		h.handleThreadMessage(event, text)
	} else {
		h.handleNewSession(event, text, threadTS)
	}
}

// handleThreadMessage handles messages in existing threads
func (h *Handler) handleThreadMessage(event *slackevents.AppMentionEvent, text string) {
	// Try to find existing session
	session, err := h.sessionMgr.GetSessionByThread(event.Channel, event.ThreadTimeStamp)
	if err == nil && session != nil {
		// Fetch full message details if file upload is enabled
		var imagePaths []string
		if h.fileUploadEnabled {
			imagePaths, err = h.fetchAndSaveImages(event.Channel, event.TimeStamp)
			if err != nil {
				fmt.Printf("Failed to fetch images: %v\n", err)
				// Continue without images
			}
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
				slack.MsgOptionText(fmt.Sprintf("メッセージ送信に失敗しました: %v", err), false),
				slack.MsgOptionTS(event.ThreadTimeStamp),
			)
		}
		return
	}

	// No existing session found - create new session
	h.handleNewSession(event, text, event.ThreadTimeStamp)
}

// handleNewSession creates a new session or resumes a previous one
func (h *Handler) handleNewSession(event *slackevents.AppMentionEvent, text string, threadTS string) {
	// Determine working directory
	workDir := h.determineWorkDir(event.Channel)

	// Fetch full message details if file upload is enabled
	var imagePaths []string
	if h.fileUploadEnabled {
		var err error
		imagePaths, err = h.fetchAndSaveImages(event.Channel, event.TimeStamp)
		if err != nil {
			fmt.Printf("Failed to fetch images: %v\n", err)
			// Continue without images
		}
	}

	// Add image paths to the prompt if any
	if len(imagePaths) > 0 {
		text = h.appendImagePaths(text, imagePaths)
	}

	// Create session with resume check
	ctx := context.Background()
	_, resumed, previousSessionID, err := h.sessionMgr.CreateSessionWithResume(ctx, event.Channel, threadTS, workDir, text)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("セッション作成に失敗しました: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	// Post initial response based on whether session was resumed
	var initialMessage string
	if resumed {
		initialMessage = fmt.Sprintf("前回のセッション `%s` を再開します...", previousSessionID)
	} else {
		initialMessage = "Claude Code セッションを開始しています..."
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
	// Remove <@BOTID> pattern
	// TODO: Get actual bot ID
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<@") {
		if idx := strings.Index(text, ">"); idx != -1 {
			text = strings.TrimSpace(text[idx+1:])
		}
	}
	return text
}

// determineWorkDir determines the working directory for a channel
func (h *Handler) determineWorkDir(channelID string) string {
	// TODO: Implement channel-specific configuration
	// For now, use current working directory
	cwd, err := os.Getwd()
	if err != nil {
		// Fallback to /tmp if we can't get current directory
		return "/tmp/cc-slack-workspace"
	}
	return cwd
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
					slack.NewTextBlockObject(slack.PlainTextType, "承認", false, false),
				).WithStyle(slack.StylePrimary),
				slack.NewButtonBlockElement(
					fmt.Sprintf("deny_%s", requestID),
					"deny",
					slack.NewTextBlockObject(slack.PlainTextType, "拒否", false, false),
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
	// For WebFetch: **ツール**: WebFetch \n **URL**: %s \n **内容**: %s
	// For Bash: **ツール**: Bash \n **コマンド**: %s \n **説明**: %s

	info := &ApprovalInfo{}
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "**ツール**: ") {
			info.ToolName = strings.TrimPrefix(line, "**ツール**: ")
		} else if strings.HasPrefix(line, "**URL**: ") {
			info.URL = strings.TrimPrefix(line, "**URL**: ")
		} else if strings.HasPrefix(line, "**内容**: ") {
			info.Prompt = strings.TrimPrefix(line, "**内容**: ")
		} else if strings.HasPrefix(line, "**コマンド**: ") {
			info.Command = strings.TrimPrefix(line, "**コマンド**: ")
		} else if strings.HasPrefix(line, "**説明**: ") {
			info.Description = strings.TrimPrefix(line, "**説明**: ")
		} else if strings.HasPrefix(line, "**ファイルパス**: ") {
			info.FilePath = strings.TrimPrefix(line, "**ファイルパス**: ")
		}
	}

	return info
}

// buildApprovalMarkdownText creates markdown text for approval request
func buildApprovalMarkdownText(info *ApprovalInfo) string {
	var text strings.Builder

	// Header
	text.WriteString("*ツールの実行許可が必要です*\n\n")

	if info.ToolName != "" {
		text.WriteString(fmt.Sprintf("*ツール:* %s\n", info.ToolName))
	}

	// Handle WebFetch tool
	if info.URL != "" {
		text.WriteString(fmt.Sprintf("*URL:* <%s>\n", info.URL))
	}

	if info.Prompt != "" {
		text.WriteString("*内容:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Prompt))
	}

	// Handle Bash tool
	if info.Command != "" {
		text.WriteString("*コマンド:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Command))
	}

	if info.Description != "" {
		if info.Command != "" {
			text.WriteString("\n")
		}
		text.WriteString("*説明:*\n")
		text.WriteString(fmt.Sprintf("```\n%s\n```", info.Description))
	}

	// Handle Write tool
	if info.FilePath != "" {
		text.WriteString(fmt.Sprintf("*ファイルパス:* `%s`", info.FilePath))
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
		statusText = "承認されました"
	} else {
		statusEmoji = ":x:"
		statusText = "拒否されました"
	}

	return fmt.Sprintf("────────────────\n%s *%s* by <@%s>", statusEmoji, statusText, userID)
}

// fetchAndSaveImages fetches message details and downloads attached images
func (h *Handler) fetchAndSaveImages(channelID, timestamp string) ([]string, error) {
	// Get conversation history to fetch the specific message
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Latest:    timestamp,
		Inclusive: true,
		Limit:     1,
	}

	history, err := h.client.GetConversationHistory(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	if len(history.Messages) == 0 {
		return nil, nil // No message found
	}

	msg := history.Messages[0]
	if len(msg.Files) == 0 {
		return nil, nil // No files attached
	}

	// Create images directory if it doesn't exist
	if err := os.MkdirAll(h.imagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}

	// Filter image files
	var imageFiles []slack.File
	for _, file := range msg.Files {
		if strings.HasPrefix(file.Mimetype, "image/") {
			imageFiles = append(imageFiles, file)
		}
	}

	if len(imageFiles) == 0 {
		return nil, nil // No image files
	}

	// Download images concurrently
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
			path, err := h.downloadAndSaveImage(file)
			if err != nil {
				errorChan <- fmt.Errorf("failed to download %s: %w", file.Name, err)
				return nil // Don't fail the whole group
			}
			resultChan <- result{path: path, idx: i}
			return nil
		})
	}

	// Wait for all downloads to complete
	if err := g.Wait(); err != nil {
		return nil, err
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

	return results, nil
}

// downloadAndSaveImage downloads a Slack file and saves it locally
func (h *Handler) downloadAndSaveImage(file slack.File) (string, error) {
	// Generate unique filename with timestamp
	timeStr := time.Now().Format("20060102-150405")
	ext := filepath.Ext(file.Name)
	if ext == "" {
		ext = ".jpg" // Default extension
	}

	// Remove extension from original filename to avoid double extensions
	baseName := strings.TrimSuffix(file.Name, ext)
	if len(baseName) > 20 {
		baseName = baseName[:20]
	}

	filename := fmt.Sprintf("%s-%s-%s%s", timeStr, file.ID, baseName, ext)
	filePath := filepath.Join(h.imagesDir, filename)

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
		return "", fmt.Errorf("failed to download file: status %d", resp.StatusCode)
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

	return builder.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
