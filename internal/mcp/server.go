package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SlackPoster interface for posting to Slack
type SlackPoster interface {
	PostApprovalRequest(channelID, threadTS, message, requestID string) error
}

// SessionLookup interface for finding session information
type SessionLookup interface {
	GetSessionInfo(sessionID string) (channelID, threadTS string, exists bool)
}

// Server wraps the MCP server and HTTP handler
type Server struct {
	mcp     *mcpsdk.Server
	handler *mcpsdk.StreamableHTTPHandler

	// Approval requests waiting for response
	approvalRequests map[string]chan ApprovalResponse
	approvalMu       sync.Mutex

	// Slack integration
	slackPoster   SlackPoster
	sessionLookup SessionLookup
}

// ApprovalRequest represents a request for user approval
// According to Claude Code docs, permission prompt receives:
// - tool_name: Name of the tool requesting permission
// - input: Input for the tool
// - tool_use_id: Unique tool use request ID (optional)
type ApprovalRequest struct {
	ToolName  string                 `json:"tool_name"`
	Input     map[string]interface{} `json:"input,omitempty"`       // Tool input parameters
	ToolUseID string                 `json:"tool_use_id,omitempty"` // Tool use identifier
}

// ApprovalResponse represents the approval response
type ApprovalResponse struct {
	Behavior     string                 `json:"behavior"` // "allow" or "deny"
	Message      string                 `json:"message,omitempty"`
	UpdatedInput map[string]interface{} `json:"updatedInput,omitempty"`
}

// NewServer creates a new MCP server
func NewServer() (*Server, error) {
	// Create MCP server instance
	mcp := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "cc-slack",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcp:              mcp,
		approvalRequests: make(map[string]chan ApprovalResponse),
	}

	// Register the approval_prompt tool
	// IMPORTANT: MCP SDK automatically prefixes tools with mcp__<serverName>__
	// So we only need to specify the base tool name here: "approval_prompt"
	// The final tool name will be: mcp__cc-slack__approval_prompt
	mcpsdk.AddTool(mcp, &mcpsdk.Tool{
		Name:        "approval_prompt",
		Description: "Request user approval via Slack",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"tool_name": {
					Type:        "string",
					Description: "Name of the tool requesting approval",
				},
				"input": {
					Type:        "object",
					Description: "Input parameters for the tool",
				},
				"tool_use_id": {
					Type:        "string",
					Description: "Unique tool use request ID",
				},
			},
			Required: []string{"tool_name"},
		},
	}, s.HandleApprovalPrompt)

	// Create StreamableHTTPHandler
	s.handler = mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		return mcp
	}, nil)

	return s, nil
}

// SetSlackIntegration sets the Slack integration components
func (s *Server) SetSlackIntegration(poster SlackPoster, lookup SessionLookup) {
	s.slackPoster = poster
	s.sessionLookup = lookup
}

// Handle processes MCP requests
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// PermissionPromptResponse represents the response format for permission prompts
type PermissionPromptResponse struct {
	Behavior     string                 `json:"behavior"`
	Message      string                 `json:"message,omitempty"`
	UpdatedInput map[string]interface{} `json:"updatedInput"` // Required for "allow", no omitempty
}

// HandleApprovalPrompt handles approval requests
// Permission prompt tools must return a specific response format
func (s *Server) HandleApprovalPrompt(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ApprovalRequest]) (*mcpsdk.CallToolResultFor[PermissionPromptResponse], error) {
	// Debug logging
	fmt.Printf("[MCP] HandleApprovalPrompt called\n")
	fmt.Printf("[MCP]   Arguments: %+v\n", params.Arguments)
	fmt.Printf("[MCP]   ToolName: %s\n", params.Arguments.ToolName)
	fmt.Printf("[MCP]   Input: %+v\n", params.Arguments.Input)
	fmt.Printf("[MCP]   ToolUseID: %s\n", params.Arguments.ToolUseID)

	// Generate request ID
	requestID := fmt.Sprintf("approval_%d", time.Now().UnixNano())

	// Create channel for response
	respChan := make(chan ApprovalResponse, 1)
	s.approvalMu.Lock()
	s.approvalRequests[requestID] = respChan
	s.approvalMu.Unlock()

	// Send approval request to Slack
	if s.slackPoster != nil && s.sessionLookup != nil {
		// MCP SDK doesn't provide direct access to session ID
		// Use empty string to trigger lastActiveID fallback in GetSessionInfo
		sessionID := ""

		// Get Slack channel and thread information
		channelID, threadTS, exists := s.sessionLookup.GetSessionInfo(sessionID)
		if exists {
			// Build approval message based on tool name and input
			message := fmt.Sprintf("🔐 **ツールの実行許可が必要です**\n\n**ツール**: %s", params.Arguments.ToolName)

			// Add tool input details if available
			if params.Arguments.Input != nil {
				if url, ok := params.Arguments.Input["url"].(string); ok {
					message += fmt.Sprintf("\n\n**URL**: %s", url)
				}
				if prompt, ok := params.Arguments.Input["prompt"].(string); ok && len(prompt) > 100 {
					message += fmt.Sprintf("\n**内容**: %s...", prompt[:100])
				} else if prompt != "" {
					message += fmt.Sprintf("\n**内容**: %s", prompt)
				}
			}

			err := s.slackPoster.PostApprovalRequest(channelID, threadTS, message, requestID)
			if err != nil {
				// Log error but continue with timeout fallback
				// Log error but continue with timeout fallback
			}
		}
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		// Clean up
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		s.approvalMu.Unlock()

		// Create permission prompt response
		promptResp := PermissionPromptResponse{
			Behavior:     resp.Behavior,
			Message:      resp.Message,
			UpdatedInput: resp.UpdatedInput,
		}

		// Ensure updatedInput is set for allow behavior
		if promptResp.Behavior == "allow" && promptResp.UpdatedInput == nil {
			promptResp.UpdatedInput = map[string]interface{}{}
		}

		// Debug log
		jsonData, _ := json.Marshal(promptResp)
		fmt.Printf("[MCP] Returning approval response: %s\n", string(jsonData))

		// Return response with both Content and StructuredContent
		result := &mcpsdk.CallToolResultFor[PermissionPromptResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: string(jsonData),
				},
			},
			StructuredContent: promptResp,
		}

		fmt.Printf("[MCP] CallToolResultFor created with both Content and StructuredContent\n")

		return result, nil

	case <-time.After(5 * time.Minute):
		// Timeout - return deny
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		s.approvalMu.Unlock()

		// Create deny response for timeout
		promptResp := PermissionPromptResponse{
			Behavior: "deny",
			Message:  "Approval request timed out",
		}

		// Convert to JSON for Content field
		jsonData, _ := json.Marshal(promptResp)

		return &mcpsdk.CallToolResultFor[PermissionPromptResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: string(jsonData),
				},
			},
			StructuredContent: promptResp,
		}, nil

	case <-ctx.Done():
		// Context cancelled
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		s.approvalMu.Unlock()

		return nil, ctx.Err()
	}
}

// SendApprovalResponse sends an approval response for a request
func (s *Server) SendApprovalResponse(requestID string, response ApprovalResponse) error {
	s.approvalMu.Lock()
	respChan, exists := s.approvalRequests[requestID]
	s.approvalMu.Unlock()

	if !exists {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	select {
	case respChan <- response:
		return nil
	default:
		return fmt.Errorf("approval response channel full")
	}
}
