package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
)

// SocketModeHandler handles Slack events via Socket Mode
type SocketModeHandler struct {
	handler       *Handler
	client        *socketmode.Client
	slackClient   *slack.Client
	signingSecret string
}

// NewSocketModeHandler creates a new Socket Mode handler
func NewSocketModeHandler(cfg *config.Config, handler *Handler) (*SocketModeHandler, error) {
	if cfg.Slack.AppToken == "" {
		return nil, fmt.Errorf("slack.app_token is required for Socket Mode")
	}

	slackClient := slack.New(
		cfg.Slack.BotToken,
		slack.OptionAppLevelToken(cfg.Slack.AppToken),
	)

	socketClient := socketmode.New(
		slackClient,
		socketmode.OptionDebug(false),
	)

	return &SocketModeHandler{
		handler:       handler,
		client:        socketClient,
		slackClient:   slackClient,
		signingSecret: cfg.Slack.SigningSecret,
	}, nil
}

// Run starts the Socket Mode handler
func (h *SocketModeHandler) Run(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-h.client.Events:
				switch evt.Type {
				case socketmode.EventTypeConnecting:
					log.Info().Msg("Connecting to Slack Socket Mode...")
				case socketmode.EventTypeConnectionError:
					log.Error().Msg("Connection failed, retrying...")
				case socketmode.EventTypeConnected:
					log.Info().Msg("Connected to Slack Socket Mode")
				case socketmode.EventTypeEventsAPI:
					h.handleEventsAPI(evt)
				case socketmode.EventTypeInteractive:
					h.handleInteractive(evt)
				case "slash_commands":
					h.handleSlashCommand(evt)
				default:
					log.Warn().Str("type", string(evt.Type)).Msg("Unsupported Socket Mode event type")
				}
			}
		}
	}()

	return h.client.RunContext(ctx)
}

// handleEventsAPI handles Events API messages
func (h *SocketModeHandler) handleEventsAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		log.Error().Msg("Failed to cast event to EventsAPIEvent")
		return
	}

	// Acknowledge the event
	h.client.Ack(*evt.Request)

	// Process the event
	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			h.handleAppMention(ev)
		case *slackevents.MessageEvent:
			// Handle message events if needed
			if ev.ThreadTimeStamp != "" && ev.User != "" {
				// This is a reply in a thread
				h.handleThreadReply(ev)
			}
		default:
			log.Warn().Str("type", innerEvent.Type).Msg("Unsupported event type")
		}
	}
}

// handleAppMention handles app mention events
func (h *SocketModeHandler) handleAppMention(event *slackevents.AppMentionEvent) {
	log.Info().
		Str("channel", event.Channel).
		Str("user", event.User).
		Str("text", event.Text).
		Msg("Received app mention")

	// Remove bot mention from text
	cleanedText := h.handler.removeBotMention(event.Text)

	// Get or create session
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	session, err := h.handler.sessionMgr.GetSessionByThread(event.Channel, threadTS)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get session")
		h.handler.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("Failed to get session: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	if session == nil {
		// Create new session
		h.handler.createThreadAndStartSession(event.Channel, h.handler.determineWorkDir(event.Channel), cleanedText, event.User)
		return
	}

	// Send message to existing session
	if err := h.handler.sessionMgr.SendMessage(session.SessionID, cleanedText); err != nil {
		log.Error().Err(err).Msg("Failed to send message to session")
		h.handler.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("Failed to send message: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
	}
}

// handleThreadReply handles thread reply events
func (h *SocketModeHandler) handleThreadReply(event *slackevents.MessageEvent) {
	// Check if this is a reply to a Claude thread
	session, err := h.handler.sessionMgr.GetSessionByThread(event.Channel, event.ThreadTimeStamp)
	if err != nil || session == nil {
		// Not a Claude thread, ignore
		return
	}

	// Check if message is from bot itself
	if event.User == h.handler.botUserID {
		return
	}

	// Send message to session
	if err := h.handler.sessionMgr.SendMessage(session.SessionID, event.Text); err != nil {
		log.Error().Err(err).Msg("Failed to send message to session")
		h.handler.client.PostMessage(
			event.Channel,
			slack.MsgOptionText(fmt.Sprintf("Failed to send message: %v", err), false),
			slack.MsgOptionTS(event.ThreadTimeStamp),
		)
	}
}

// handleInteractive handles interactive events (button clicks, etc)
func (h *SocketModeHandler) handleInteractive(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		log.Error().Msg("Failed to cast event to InteractionCallback")
		return
	}

	// Acknowledge the event
	h.client.Ack(*evt.Request)

	// Handle the interaction
	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		for _, action := range callback.ActionCallback.BlockActions {
			if strings.HasPrefix(action.ActionID, "approve_") || strings.HasPrefix(action.ActionID, "deny_") {
				h.handleApprovalAction(&callback, action)
			} else if strings.HasPrefix(action.ActionID, "select_dir_") {
				h.handleDirectorySelection(&callback, action)
			}
		}
	default:
		log.Warn().Str("type", string(callback.Type)).Msg("Unsupported interaction type")
	}
}

// handleApprovalAction handles approval/denial actions
func (h *SocketModeHandler) handleApprovalAction(callback *slack.InteractionCallback, action *slack.BlockAction) {
	// Extract request ID from action ID
	parts := strings.SplitN(action.ActionID, "_", 2)
	if len(parts) != 2 {
		log.Error().Str("action_id", action.ActionID).Msg("Invalid action ID format")
		return
	}

	isApproved := parts[0] == "approve"
	requestID := parts[1]

	// Send approval response
	response := mcp.ApprovalResponse{
		Behavior: "allow",
	}
	if !isApproved {
		response.Behavior = "deny"
	}

	if h.handler.approvalResponder != nil {
		if err := h.handler.approvalResponder.SendApprovalResponse(requestID, response); err != nil {
			log.Error().Err(err).Msg("Failed to send approval response")
		}
	}

	// Update the message
	var responseText string
	if isApproved {
		responseText = fmt.Sprintf("✅ Approved by <@%s>", callback.User.ID)
	} else {
		responseText = fmt.Sprintf("❌ Denied by <@%s>", callback.User.ID)
	}

	_, _, err := h.handler.client.PostMessage(
		callback.Channel.ID,
		slack.MsgOptionUpdate(callback.MessageTs),
		slack.MsgOptionText(responseText, false),
		slack.MsgOptionReplaceOriginal(callback.ResponseURL),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update message")
	}
}

// handleSlashCommand handles slash command events
func (h *SocketModeHandler) handleSlashCommand(evt socketmode.Event) {
	command, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		log.Error().Msg("Failed to cast event to SlashCommand")
		return
	}

	// Acknowledge the event
	h.client.Ack(*evt.Request)

	// Handle based on command
	switch command.Command {
	case h.handler.config.Slack.SlashCommandName:
		h.handleCCCommand(&command)
	default:
		log.Warn().Str("command", command.Command).Msg("Unknown slash command")
	}
}

// handleCCCommand handles the /cc slash command
func (h *SocketModeHandler) handleCCCommand(cmd *slack.SlashCommand) {
	// In single directory mode, start session immediately
	if h.handler.config.IsSingleDirectoryMode() {
		workDir := h.handler.config.GetSingleWorkingDirectory()
		h.handler.createThreadAndStartSession(cmd.ChannelID, workDir, cmd.Text, cmd.UserID)
		return
	}

	// In multi-directory mode, show directory selection
	h.showDirectorySelection(cmd)
}

// showDirectorySelection shows working directory selection UI
func (h *SocketModeHandler) showDirectorySelection(cmd *slack.SlashCommand) {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "Select a working directory:", false, false),
			nil,
			nil,
		),
	}

	// Add directory options
	for _, wd := range h.handler.config.WorkingDirs {
		buttonText := wd.Name
		if wd.Description != "" {
			buttonText = fmt.Sprintf("%s - %s", wd.Name, wd.Description)
		}

		block := slack.NewActionBlock(
			"",
			slack.NewButtonBlockElement(
				fmt.Sprintf("select_dir_%s", wd.Path),
				wd.Path,
				slack.NewTextBlockObject("plain_text", buttonText, false, false),
			),
		)
		blocks = append(blocks, block)
	}

	// Send ephemeral message with directory options
	_, err := h.handler.client.PostEphemeral(
		cmd.ChannelID,
		cmd.UserID,
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send directory selection message")
		h.handler.client.PostEphemeral(
			cmd.ChannelID,
			cmd.UserID,
			slack.MsgOptionText("Failed to show directory selection", false),
		)
	}
}

// handleDirectorySelection handles working directory selection
func (h *SocketModeHandler) handleDirectorySelection(callback *slack.InteractionCallback, action *slack.BlockAction) {
	// Extract working directory path from action value
	workDir := action.Value

	// Get initial prompt from original slash command (if any)
	// Note: In Socket Mode, we don't have access to the original slash command text
	// So we'll start with an empty prompt
	prompt := ""

	// Delete the ephemeral message
	h.handler.client.DeleteMessage(callback.Channel.ID, callback.MessageTs)

	// Create thread and start session
	h.handler.createThreadAndStartSession(callback.Channel.ID, workDir, prompt, callback.User.ID)
}
