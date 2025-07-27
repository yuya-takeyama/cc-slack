package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
)

// Tool type constants for tool-specific display
const (
	ToolTodoWrite    = "TodoWrite"
	ToolBash         = "Bash"
	ToolRead         = "Read"
	ToolGlob         = "Glob"
	ToolEdit         = "Edit"
	ToolMultiEdit    = "MultiEdit"
	ToolWrite        = "Write"
	ToolLS           = "LS"
	ToolGrep         = "Grep"
	ToolWebFetch     = "WebFetch"
	ToolWebSearch    = "WebSearch"
	ToolTask         = "Task"
	ToolExitPlanMode = "ExitPlanMode"
	ToolNotebookRead = "NotebookRead"
	ToolNotebookEdit = "NotebookEdit"
)

// ToolDisplayInfo holds display information for tools
type ToolDisplayInfo struct {
	Username string
	Emoji    string
}

// toolDisplayMap maps tool types to their display information
var toolDisplayMap = map[string]ToolDisplayInfo{
	ToolTodoWrite:    {Username: "todo-list", Emoji: ":memo:"},
	ToolBash:         {Username: "terminal", Emoji: ":computer:"},
	ToolRead:         {Username: "file-reader", Emoji: ":open_book:"},
	ToolGlob:         {Username: "file-finder", Emoji: ":mag:"},
	ToolEdit:         {Username: "editor", Emoji: ":pencil2:"},
	ToolMultiEdit:    {Username: "editor", Emoji: ":pencil2:"},
	ToolWrite:        {Username: "writer", Emoji: ":memo:"},
	ToolLS:           {Username: "directory", Emoji: ":file_folder:"},
	ToolGrep:         {Username: "searcher", Emoji: ":mag:"},
	ToolWebFetch:     {Username: "web-fetcher", Emoji: ":globe_with_meridians:"},
	ToolWebSearch:    {Username: "web-search", Emoji: ":earth_americas:"},
	ToolTask:         {Username: "agent", Emoji: ":robot_face:"},
	ToolExitPlanMode: {Username: "planner", Emoji: ":checkered_flag:"},
	ToolNotebookRead: {Username: "notebook-reader", Emoji: ":notebook:"},
	ToolNotebookEdit: {Username: "notebook-editor", Emoji: ":notebook_with_decorative_cover:"},
}

// Handler handles Slack events and interactions
type Handler struct {
	client             *slack.Client
	signingSecret      string
	sessionMgr         SessionManager
	approvalResponder  ApprovalResponder
	assistantUsername  string
	assistantIconEmoji string
	assistantIconURL   string
}

// SessionManager interface for managing Claude Code sessions
type SessionManager interface {
	GetSessionByThread(channelID, threadTS string) (*Session, error)
	CreateSession(channelID, threadTS, workDir string) (*Session, error)
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
	return &Handler{
		client:        slack.New(token),
		signingSecret: signingSecret,
		sessionMgr:    sessionMgr,
	}
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
	// Extract message without mention
	text := h.removeBotMention(event.Text)
	if text == "" {
		return
	}

	// Determine working directory
	workDir := h.determineWorkDir(event.Channel)

	// Post initial response
	_, resp, err := h.client.PostMessage(
		event.Channel,
		slack.MsgOptionText("Claude Code セッションを開始しています...", false),
		slack.MsgOptionTS(event.TimeStamp),
	)
	if err != nil {
		fmt.Printf("Failed to post message: %v\n", err)
		return
	}

	// Create new session
	session, err := h.sessionMgr.CreateSession(event.Channel, resp, workDir)
	if err != nil {
		h.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("セッション作成に失敗しました: %v", err), false),
			slack.MsgOptionTS(resp),
		)
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

	// Update the message
	status := "承認されました ✅"
	if !approved {
		status = "拒否されました ❌"
	}

	_, _, _, err := h.client.UpdateMessage(
		payload.Channel.ID,
		payload.Message.Timestamp,
		slack.MsgOptionText(fmt.Sprintf("%s\n\n%s", payload.Message.Text, status), false),
		slack.MsgOptionReplaceOriginal(payload.ResponseURL),
	)
	if err != nil {
		fmt.Printf("Failed to update message: %v\n", err)
	}
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
	if displayInfo, exists := toolDisplayMap[toolType]; exists {
		// Add username
		options = append(options, slack.MsgOptionUsername(displayInfo.Username))
		// Add icon emoji
		options = append(options, slack.MsgOptionIconEmoji(displayInfo.Emoji))
	} else {
		// Fallback to generic tool display
		options = append(options, slack.MsgOptionUsername("tool"))
		options = append(options, slack.MsgOptionIconEmoji(":wrench:"))
	}

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
	if displayInfo, exists := toolDisplayMap[toolType]; exists {
		// Add username
		options = append(options, slack.MsgOptionUsername(displayInfo.Username))
		// Add icon emoji
		options = append(options, slack.MsgOptionIconEmoji(displayInfo.Emoji))
	} else {
		// Fallback to generic tool display
		options = append(options, slack.MsgOptionUsername("tool"))
		options = append(options, slack.MsgOptionIconEmoji(":wrench:"))
	}

	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}

// PostApprovalRequest posts an approval request with buttons
func (h *Handler) PostApprovalRequest(channelID, threadTS, message, requestID string) error {
	// Create minimal interactive payload for debugging
	approvePayload := map[string]interface{}{
		"type": "block_actions",
		"actions": []map[string]interface{}{
			{
				"action_id": fmt.Sprintf("approve_%s", requestID),
				"type":      "button",
				"value":     "approve",
			},
		},
		"channel": map[string]string{
			"id": channelID,
		},
		"message": map[string]string{
			"ts":   threadTS,
			"text": message,
		},
	}

	denyPayload := map[string]interface{}{
		"type": "block_actions",
		"actions": []map[string]interface{}{
			{
				"action_id": fmt.Sprintf("deny_%s", requestID),
				"type":      "button",
				"value":     "deny",
			},
		},
		"channel": map[string]string{
			"id": channelID,
		},
		"message": map[string]string{
			"ts":   threadTS,
			"text": message,
		},
	}

	approveJSON, _ := json.Marshal(approvePayload)
	denyJSON, _ := json.Marshal(denyPayload)

	// Add debug curl commands to the message
	debugMessage := message + "\n\n*【デバッグ用curlコマンド】*\n" +
		"```bash\n" +
		"# 承認する場合:\n" +
		fmt.Sprintf("curl -X POST http://localhost:8080/slack/interactive \\\n") +
		fmt.Sprintf("  -H \"Content-Type: application/x-www-form-urlencoded\" \\\n") +
		fmt.Sprintf("  --data-urlencode 'payload=%s'\n\n", string(approveJSON)) +
		"# 拒否する場合:\n" +
		fmt.Sprintf("curl -X POST http://localhost:8080/slack/interactive \\\n") +
		fmt.Sprintf("  -H \"Content-Type: application/x-www-form-urlencoded\" \\\n") +
		fmt.Sprintf("  --data-urlencode 'payload=%s'\n", string(denyJSON)) +
		"```"

	_, _, err := h.client.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject(slack.MarkdownType, debugMessage, false, false),
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
	)
	return err
}
