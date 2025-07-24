package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/yuya-takeyama/cc-slack/pkg/types"
)

// ApprovalManager handles approval requests through Slack
type ApprovalManager struct {
	client           *slack.Client
	pendingApprovals map[string]*PendingApproval
	mu               sync.RWMutex
}

type PendingApproval struct {
	Request   types.ApprovalRequest
	ChannelID string
	MessageTS string
	ResponseCh chan types.ApprovalResponse
	ExpiresAt  time.Time
}

func NewApprovalManager(client *slack.Client) *ApprovalManager {
	return &ApprovalManager{
		client:           client,
		pendingApprovals: make(map[string]*PendingApproval),
	}
}

func (am *ApprovalManager) RequestApproVal(ctx context.Context, req types.ApprovalRequest, channelID string) (types.ApprovalResponse, error) {
	// Generate unique approval ID
	approvalID := generateApprovalID()

	// Create pending approval
	pending := &PendingApproval{
		Request:    req,
		ChannelID:  channelID,
		ResponseCh: make(chan types.ApprovalResponse, 1),
		ExpiresAt:  time.Now().Add(5 * time.Minute), // 5 minute timeout
	}

	am.mu.Lock()
	am.pendingApprovals[approvalID] = pending
	am.mu.Unlock()

	// Clean up after timeout
	defer func() {
		am.mu.Lock()
		delete(am.pendingApprovals, approvalID)
		am.mu.Unlock()
	}()

	// Post approval request to Slack with interactive buttons
	if err := am.postApprovalRequest(approvalID, pending); err != nil {
		return types.ApprovalResponse{
			Behavior: "deny",
			Message:  fmt.Sprintf("Failed to post approval request: %s", err.Error()),
		}, nil
	}

	// Wait for response or timeout
	select {
	case response := <-pending.ResponseCh:
		return response, nil
	case <-time.After(5 * time.Minute):
		return types.ApprovalResponse{
			Behavior: "deny",
			Message:  "Approval request timed out",
		}, nil
	case <-ctx.Done():
		return types.ApprovalResponse{
			Behavior: "deny",
			Message:  "Approval request cancelled",
		}, nil
	}
}

func (am *ApprovalManager) postApprovalRequest(approvalID string, pending *PendingApproval) error {
	// Format tool input for display
	inputJSON, _ := json.MarshalIndent(pending.Request.Input, "", "  ")
	
	text := fmt.Sprintf("🔧 *Claude Code 承認リクエスト*\n\n"+
		"ツール: `%s`\n"+
		"コンテキスト: %s\n"+
		"入力パラメータ:\n```json\n%s\n```",
		pending.Request.ToolName,
		pending.Request.Context,
		string(inputJSON))

	// Create interactive message with buttons
	blocks := []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: text,
			},
			nil, nil,
		),
		slack.NewActionBlock(
			"approval_actions",
			slack.NewButtonBlockElement(
				fmt.Sprintf("approve_%s", approvalID),
				"approve",
				&slack.TextBlockObject{
					Type: slack.PlainTextType,
					Text: "承認 ✅",
				},
			).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement(
				fmt.Sprintf("deny_%s", approvalID),
				"deny",
				&slack.TextBlockObject{
					Type: slack.PlainTextType,
					Text: "拒否 ❌",
				},
			).WithStyle(slack.StyleDanger),
		),
	}

	_, timestamp, err := am.client.PostMessage(
		pending.ChannelID,
		slack.MsgOptionBlocks(blocks...),
	)

	if err != nil {
		return err
	}

	pending.MessageTS = timestamp
	return nil
}

func (am *ApprovalManager) HandleInteraction(payload slack.InteractionCallback) error {
	if len(payload.ActionCallback.BlockActions) == 0 {
		return fmt.Errorf("no block actions found")
	}

	action := payload.ActionCallback.BlockActions[0]
	
	// Extract approval ID from action ID
	var approvalID string
	var approved bool
	
	if action.Value == "approve" && len(action.ActionID) > 8 { // "approve_" prefix
		approvalID = action.ActionID[8:] // Remove "approve_" prefix
		approved = true
	} else if action.Value == "deny" && len(action.ActionID) > 5 { // "deny_" prefix
		approvalID = action.ActionID[5:] // Remove "deny_" prefix
		approved = false
	} else {
		return fmt.Errorf("unknown action: %s", action.ActionID)
	}

	am.mu.RLock()
	pending, exists := am.pendingApprovals[approvalID]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("approval request not found or expired")
	}

	// Check if expired
	if time.Now().After(pending.ExpiresAt) {
		return fmt.Errorf("approval request expired")
	}

	// Create response
	var response types.ApprovalResponse
	if approved {
		response = types.ApprovalResponse{
			Behavior: "allow",
			Message:  fmt.Sprintf("Tool %s approved by %s", pending.Request.ToolName, payload.User.Name),
		}
	} else {
		response = types.ApprovalResponse{
			Behavior: "deny",
			Message:  fmt.Sprintf("Tool %s denied by %s", pending.Request.ToolName, payload.User.Name),
		}
	}

	// Send response
	select {
	case pending.ResponseCh <- response:
		// Update Slack message to show result
		am.updateApprovalMessage(payload, pending, approved)
	default:
		slog.Warn("Failed to send approval response", "approval_id", approvalID)
	}

	return nil
}

func (am *ApprovalManager) updateApprovalMessage(payload slack.InteractionCallback, pending *PendingApproval, approved bool) {
	var resultText string
	var emoji string
	
	if approved {
		resultText = fmt.Sprintf("✅ *承認されました* (by %s)", payload.User.Name)
		emoji = "✅"
	} else {
		resultText = fmt.Sprintf("❌ *拒否されました* (by %s)", payload.User.Name)
		emoji = "❌"
	}

	// Format original request info
	inputJSON, _ := json.MarshalIndent(pending.Request.Input, "", "  ")
	
	originalText := fmt.Sprintf("🔧 *Claude Code 承認リクエスト* %s\n\n"+
		"ツール: `%s`\n"+
		"コンテキスト: %s\n"+
		"入力パラメータ:\n```json\n%s\n```\n\n%s",
		emoji,
		pending.Request.ToolName,
		pending.Request.Context,
		string(inputJSON),
		resultText)

	// Update message to remove buttons and show result
	blocks := []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: originalText,
			},
			nil, nil,
		),
	}

	_, _, _, err := am.client.UpdateMessage(
		payload.Channel.ID,
		pending.MessageTS,
		slack.MsgOptionBlocks(blocks...),
	)

	if err != nil {
		slog.Error("Failed to update approval message", "error", err)
	}
}

func generateApprovalID() string {
	return fmt.Sprintf("approval_%d", time.Now().UnixNano())
}