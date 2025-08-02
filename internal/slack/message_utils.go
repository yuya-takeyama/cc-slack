package slack

import (
	"fmt"

	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/slack/blocks"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
)

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
func (h *Handler) PostApprovalRequest(channelID, threadTS, message, requestID, userID string) error {
	// Add user mention at the beginning of the message if userID is provided
	if userID != "" {
		message = fmt.Sprintf("<@%s> %s", userID, message)
	}
	options := blocks.ApprovalRequestOptions(channelID, threadTS, message, requestID)
	_, _, err := h.client.PostMessage(channelID, options...)
	return err
}
