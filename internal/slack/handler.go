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

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
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
	resumeDebugLogger  *ResumeDebugLogger
}

// ResumeDebugLogger is a helper for debug logging
type ResumeDebugLogger struct {
	logger  zerolog.Logger
	logFile *os.File
}

// NewResumeDebugLogger creates a new resume debug logger
func NewResumeDebugLogger() (*ResumeDebugLogger, error) {
	// Create logs directory
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create log file with timestamp
	logFileName := fmt.Sprintf("resume-debug-%s.log", time.Now().Format("20060102-150405"))
	logPath := filepath.Join(logDir, logFileName)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Setup logger
	logger := zerolog.New(logFile).With().
		Timestamp().
		Str("component", "resume_debug").
		Logger()

	return &ResumeDebugLogger{
		logger:  logger,
		logFile: logFile,
	}, nil
}

// SessionManager interface for managing Claude Code sessions
type SessionManager interface {
	GetSessionByThread(channelID, threadTS string) (*Session, error)
	CreateSession(channelID, threadTS, workDir string) (*Session, error)
	CreateSessionWithResume(ctx context.Context, channelID, threadTS, workDir string) (*Session, bool, string, error)
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
	}

	// Initialize resume debug logger (errors are non-fatal)
	if debugLogger, err := NewResumeDebugLogger(); err == nil {
		h.resumeDebugLogger = debugLogger
	} else {
		fmt.Printf("Failed to create resume debug logger: %v\n", err)
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
		case *slackevents.MessageEvent:
			if ev.ThreadTimeStamp != "" && ev.ThreadTimeStamp != ev.TimeStamp {
				h.handleThreadMessage(ev)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleAppMention handles bot mentions
func (h *Handler) handleAppMention(event *slackevents.AppMentionEvent) {
	// Log the incoming mention
	if h.resumeDebugLogger != nil {
		h.resumeDebugLogger.logger.Info().
			Str("event", "app_mention").
			Str("channel", event.Channel).
			Str("timestamp", event.TimeStamp).
			Str("text", event.Text).
			Msg("Received app mention")
	}

	// Extract message without mention
	text := h.removeBotMention(event.Text)
	if text == "" {
		return
	}

	// Determine working directory
	workDir := h.determineWorkDir(event.Channel)

	// Determine thread timestamp for session management
	// If mentioned in a thread, use thread_ts; otherwise use the message ts
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	if h.resumeDebugLogger != nil {
		h.resumeDebugLogger.logger.Info().
			Str("event", "pre_create_session").
			Str("channel", event.Channel).
			Str("timestamp", event.TimeStamp).
			Str("thread_timestamp", event.ThreadTimeStamp).
			Str("resolved_thread_ts", threadTS).
			Str("work_dir", workDir).
			Msg("About to create session with resume check")
	}

	// Create session with resume check
	ctx := context.Background()
	session, resumed, previousSessionID, err := h.sessionMgr.CreateSessionWithResume(ctx, event.Channel, threadTS, workDir)

	if h.resumeDebugLogger != nil {
		h.resumeDebugLogger.logger.Info().
			Str("event", "post_create_session").
			Str("channel", event.Channel).
			Str("timestamp", event.TimeStamp).
			Bool("resumed", resumed).
			Err(err).
			Str("session_id", func() string {
				if session != nil {
					return session.SessionID
				}
				return ""
			}()).
			Msg("CreateSessionWithResume result")
	}

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
		initialMessage = fmt.Sprintf("前回のセッション %s を再開します...", previousSessionID)
	} else {
		initialMessage = "Claude Code セッションを開始しています..."
	}

	_, resp, err := h.client.PostMessage(
		event.Channel,
		slack.MsgOptionText(initialMessage, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		fmt.Printf("Failed to post message: %v\n", err)
		return
	}

	// Send initial message to Claude Code
	err = h.sessionMgr.SendMessage(session.SessionID, text)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("メッセージ送信に失敗しました: %v", err), false),
			slack.MsgOptionTS(resp),
		)
		return
	}
}

// handleThreadMessage handles messages in existing threads
func (h *Handler) handleThreadMessage(event *slackevents.MessageEvent) {
	// Skip bot messages
	if event.BotID != "" {
		return
	}

	// Find existing session
	session, err := h.sessionMgr.GetSessionByThread(event.Channel, event.ThreadTimeStamp)
	if err != nil {
		return // Not our thread
	}

	// Send message to Claude Code
	err = h.sessionMgr.SendMessage(session.SessionID, event.Text)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("メッセージ送信に失敗しました: %v", err), false),
			slack.MsgOptionTS(event.ThreadTimeStamp),
		)
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

// PostApprovalRequest posts an approval request with buttons using rich text
func (h *Handler) PostApprovalRequest(channelID, threadTS, message, requestID string) error {
	// Parse the message to extract structured information
	// This is a simple parser for the current format from mcp/server.go
	info := parseApprovalMessage(message)

	// Create rich text elements for the approval request
	elements := buildApprovalRichTextElements(info)

	// Get tool display info for permission prompt
	toolInfo := tools.GetToolInfo(MessageApprovalPrompt)

	options := []slack.MsgOption{
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewRichTextBlock("rich_text", elements...),
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
		}
	}

	return info
}

// buildApprovalRichTextElements creates rich text elements for approval request
func buildApprovalRichTextElements(info *ApprovalInfo) []slack.RichTextElement {
	var elements []slack.RichTextElement

	// Header
	elements = append(elements, slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement("ツールの実行許可が必要です", &slack.RichTextSectionTextStyle{Bold: true}),
	))

	// Empty line
	elements = append(elements, slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement("\n", nil),
	))

	if info.ToolName != "" {
		// Tool name
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("ツール: ", &slack.RichTextSectionTextStyle{Bold: true}),
			slack.NewRichTextSectionTextElement(info.ToolName, nil),
		))
	}

	// Handle WebFetch tool
	if info.URL != "" {
		// URL
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("URL: ", &slack.RichTextSectionTextStyle{Bold: true}),
			slack.NewRichTextSectionLinkElement(info.URL, info.URL, nil),
		))
	}

	if info.Prompt != "" {
		// Content/Prompt
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("内容:", &slack.RichTextSectionTextStyle{Bold: true}),
		))

		// Add prompt as code block style with triple quotes
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(info.Prompt, &slack.RichTextSectionTextStyle{Code: true}),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
	}

	// Handle Bash tool
	if info.Command != "" {
		// Command
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("コマンド:", &slack.RichTextSectionTextStyle{Bold: true}),
		))

		// Add command as code block style with triple quotes
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(info.Command, &slack.RichTextSectionTextStyle{Code: true}),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
	}

	if info.Description != "" {
		// Description
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("説明:", &slack.RichTextSectionTextStyle{Bold: true}),
		))

		// Add description as code block style with triple quotes
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(info.Description, &slack.RichTextSectionTextStyle{Code: true}),
		))
		elements = append(elements, slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("```", nil),
		))
	}

	return elements
}

// updateApprovalMessage updates the approval message with status and user information
func (h *Handler) updateApprovalMessage(payload *slack.InteractionCallback, approved bool) {
	// Preserve the original blocks and add a status block
	originalBlocks := payload.Message.Blocks.BlockSet

	// Remove the action block (last block) which contains the buttons
	if len(originalBlocks) > 0 {
		originalBlocks = originalBlocks[:len(originalBlocks)-1]
	}

	// Create status elements with user information
	statusElements := h.buildStatusRichTextElements(payload.User.ID, payload.User.Name, approved)

	// Create new blocks with original content + status
	newBlocks := make([]slack.Block, 0, len(originalBlocks)+1)
	newBlocks = append(newBlocks, originalBlocks...)
	newBlocks = append(newBlocks, slack.NewRichTextBlock("status_rich_text", statusElements...))

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

// buildStatusRichTextElements creates rich text elements for approval status
func (h *Handler) buildStatusRichTextElements(userID, userName string, approved bool) []slack.RichTextElement {
	var elements []slack.RichTextElement

	// Add separator line
	elements = append(elements, slack.NewRichTextSection(
		slack.NewRichTextSectionTextElement("────────────────", &slack.RichTextSectionTextStyle{Italic: true}),
	))

	// Status message with emoji and user mention
	var statusEmoji, statusText string
	if approved {
		statusEmoji = "white_check_mark"
		statusText = "承認されました"
	} else {
		statusEmoji = "x"
		statusText = "拒否されました"
	}

	elements = append(elements, slack.NewRichTextSection(
		slack.NewRichTextSectionEmojiElement(statusEmoji, 0, nil),
		slack.NewRichTextSectionTextElement(" ", nil),
		slack.NewRichTextSectionTextElement(statusText, &slack.RichTextSectionTextStyle{Bold: true}),
		slack.NewRichTextSectionTextElement(" by ", nil),
		slack.NewRichTextSectionUserElement(userID, nil),
	))

	return elements
}
