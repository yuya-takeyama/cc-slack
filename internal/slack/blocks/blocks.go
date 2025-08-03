package blocks

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/tools"
)

// MultiDirectoryError creates blocks for multi-directory mode error
func MultiDirectoryError(slashCommand string) []slack.Block {
	return []slack.Block{
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
				fmt.Sprintf("*How to start a session:*\n1. Type `%s` or use the shortcut menu\n2. Select a working directory\n3. Enter your initial prompt", slashCommand),
				false,
				false,
			),
			nil,
			nil,
		),
	}
}

// ApprovalRequest creates blocks for tool approval request
func ApprovalRequest(message, requestID, userID string) []slack.Block {
	// Parse the message to extract structured information
	info := parseApprovalMessage(message)
	info.UserID = userID

	// Build markdown text for the approval request
	markdownText := buildApprovalMarkdownText(info)

	return []slack.Block{
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
			slack.NewButtonBlockElement(
				fmt.Sprintf("deny_with_reason_%s", requestID),
				"deny_with_reason",
				slack.NewTextBlockObject(slack.PlainTextType, "Deny with Reason", false, false),
			),
		),
	}
}

// ApprovalMessageUpdate creates blocks for approval status update
func ApprovalMessageUpdate(originalText string, userID string, approved bool) []slack.Block {
	// Create status markdown text
	statusText := buildStatusMarkdownText(userID, approved)

	// Combine original text with status
	fullText := originalText + "\n\n" + statusText

	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, fullText, false, false),
			nil,
			nil,
		),
	}
}

// SessionStartModal creates a modal for starting a new session (multi-directory mode)
func SessionStartModal(channelID string, workingDirs []config.WorkingDirectoryConfig) slack.ModalViewRequest {
	// Build options from configured working directories
	var options []*slack.OptionBlockObject

	for _, wd := range workingDirs {
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

	return slack.ModalViewRequest{
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
}

// SessionStartModalSingle creates a modal for starting a new session (single-directory mode)
func SessionStartModalSingle(channelID string) slack.ModalViewRequest {
	return slack.ModalViewRequest{
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
}

// ApprovalRequestOptions returns message options for approval request
func ApprovalRequestOptions(channelID, threadTS, message, requestID, userID string) []slack.MsgOption {
	// Get tool display info for permission prompt
	toolInfo := tools.GetToolInfo(tools.MessageApprovalPrompt)

	blocks := ApprovalRequest(message, requestID, userID)

	return []slack.MsgOption{
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionUsername(toolInfo.Name),
		slack.MsgOptionIconEmoji(toolInfo.SlackIcon),
	}
}

// Helper functions (from handler.go)

// ApprovalInfo holds structured information about an approval request
type ApprovalInfo struct {
	ToolName    string
	URL         string
	Prompt      string
	Command     string
	Description string
	FilePath    string
	UserID      string
}

// parseApprovalMessage parses the approval message from mcp/server.go to extract structured information
func parseApprovalMessage(message string) *ApprovalInfo {
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
	if info.UserID != "" {
		text.WriteString(fmt.Sprintf("<@%s> *Tool execution permission required*\n\n", info.UserID))
	} else {
		text.WriteString("*Tool execution permission required*\n\n")
	}

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

// buildStatusMarkdownText creates markdown text for approval status
func buildStatusMarkdownText(userID string, approved bool) string {
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

// DenyReasonModal creates a modal for entering denial reason
func DenyReasonModal(metadata string) slack.ModalViewRequest {
	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      "deny_reason_modal",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Deny with Reason", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Deny", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		PrivateMetadata: metadata, // Store metadata for later use
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewInputBlock(
					"reason_block",
					slack.NewTextBlockObject(slack.PlainTextType, "Reason for denial", false, false),
					nil,
					slack.NewPlainTextInputBlockElement(
						slack.NewTextBlockObject(slack.PlainTextType, "Explain why this request is being denied...", false, false),
						"reason_input",
					),
				),
			},
		},
	}
}
