package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/yuya-takeyama/cc-slack/internal/mcp"
	"github.com/yuya-takeyama/cc-slack/internal/richtext"
	"github.com/yuya-takeyama/cc-slack/internal/slack/blocks"
)

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

	// Create new blocks with updated text
	newBlocks := blocks.ApprovalMessageUpdate(originalText, payload.User.ID, approved)

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

// openRepoModal opens the working directory selection modal
func (h *Handler) openRepoModal(triggerID, channelID, userID, initialText string) {
	// In single directory mode, show modal with only prompt input
	if h.config.IsSingleDirectoryMode() {
		modal := blocks.SessionStartModalSingle(channelID)

		// Open modal
		_, err := h.client.OpenView(triggerID, modal)
		if err != nil {
			log.Error().Err(err).Msg("failed to open modal")
		}
		return
	}

	// Multi-directory mode: Create modal view
	modal := blocks.SessionStartModal(channelID, h.config.WorkingDirs)

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
