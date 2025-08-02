package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

// SlackPoster interface for posting to Slack
type SlackPoster interface {
	PostApprovalRequest(channelID, threadTS, message, requestID, userID string) error
}

// SessionLookup interface for finding session information
type SessionLookup interface {
	GetSessionInfo(sessionID string) (channelID, threadTS, userID string, exists bool)
}

// Server wraps the MCP server and HTTP handler
type Server struct {
	mcp     *mcpsdk.Server
	handler *mcpsdk.StreamableHTTPHandler

	// Approval requests waiting for response
	approvalRequests map[string]chan ApprovalResponse
	approvalInputs   map[string]map[string]interface{} // Store original inputs by requestID
	approvalMu       sync.Mutex

	// Slack integration
	slackPoster   SlackPoster
	sessionLookup SessionLookup

	// Logger
	logger  zerolog.Logger
	logFile *os.File
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

// generateLogFileName generates a log file name with prefix and timestamp
func generateLogFileName(prefix string) string {
	return fmt.Sprintf("%s-%s.log", prefix, time.Now().Format("20060102-150405"))
}

// NewServer creates a new MCP server
func NewServer() (*Server, error) {
	// Create log directory if it doesn't exist
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file
	logFileName := generateLogFileName("mcp")
	logPath := filepath.Join(logDir, logFileName)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Setup logger
	logger := zerolog.New(logFile).With().
		Timestamp().
		Str("component", "mcp_server").
		Logger()

	// Create MCP server instance
	mcp := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "cc-slack",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcp:              mcp,
		approvalRequests: make(map[string]chan ApprovalResponse),
		approvalInputs:   make(map[string]map[string]interface{}),
		logger:           logger,
		logFile:          logFile,
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
	// Generate request ID
	requestID := fmt.Sprintf("approval_%d", time.Now().UnixNano())

	// Log incoming request
	s.logger.Info().
		Str("method", "HandleApprovalPrompt").
		Str("request_id", requestID).
		Str("tool_name", params.Arguments.ToolName).
		Interface("input", params.Arguments.Input).
		Str("tool_use_id", params.Arguments.ToolUseID).
		Msg("Received approval prompt request")

	// Create channel for response and store original input
	respChan := make(chan ApprovalResponse, 1)
	s.approvalMu.Lock()
	s.approvalRequests[requestID] = respChan
	s.approvalInputs[requestID] = params.Arguments.Input
	s.approvalMu.Unlock()

	// Send approval request to Slack
	if s.slackPoster != nil && s.sessionLookup != nil {
		// MCP SDK doesn't provide direct access to session ID
		// Use empty string to trigger lastActiveID fallback in GetSessionInfo
		sessionID := ""

		// Get Slack channel and thread information
		channelID, threadTS, userID, exists := s.sessionLookup.GetSessionInfo(sessionID)
		if exists {
			// Build approval message based on tool name and input
			message := fmt.Sprintf("🔐 **Tool execution permission required**\n\n**Tool**: %s", params.Arguments.ToolName)

			// Add tool input details if available
			if params.Arguments.Input != nil {
				// Handle WebFetch tool
				if url, ok := params.Arguments.Input["url"].(string); ok {
					message += fmt.Sprintf("\n\n**URL**: %s", url)
				}
				if prompt, ok := params.Arguments.Input["prompt"].(string); ok && len(prompt) > 100 {
					message += fmt.Sprintf("\n**Content**: %s...", prompt[:100])
				} else if prompt, ok := params.Arguments.Input["prompt"].(string); ok && prompt != "" {
					message += fmt.Sprintf("\n**Content**: %s", prompt)
				}

				// Handle Bash tool
				if command, ok := params.Arguments.Input["command"].(string); ok {
					message += fmt.Sprintf("\n\n**Command**: %s", command)
				}
				if description, ok := params.Arguments.Input["description"].(string); ok && description != "" {
					message += fmt.Sprintf("\n**Description**: %s", description)
				}

				// Handle Write tool
				if filePath, ok := params.Arguments.Input["file_path"].(string); ok {
					message += fmt.Sprintf("\n\n**File path**: %s", filePath)
				}
			}

			err := s.slackPoster.PostApprovalRequest(channelID, threadTS, message, requestID, userID)
			if err != nil {
				// Log error but continue with timeout fallback
				s.logger.Error().
					Err(err).
					Str("method", "HandleApprovalPrompt").
					Str("request_id", requestID).
					Str("channel_id", channelID).
					Str("thread_ts", threadTS).
					Msg("Failed to post approval request to Slack")
			}
		}
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		// Clean up and get original input
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		originalInput := s.approvalInputs[requestID]
		delete(s.approvalInputs, requestID)
		s.approvalMu.Unlock()

		// Create permission prompt response
		promptResp := PermissionPromptResponse{
			Behavior:     resp.Behavior,
			Message:      resp.Message,
			UpdatedInput: resp.UpdatedInput,
		}

		// Ensure updatedInput is set for allow behavior
		// If Slack sent empty map or nil, use the original input
		if promptResp.Behavior == "allow" && (promptResp.UpdatedInput == nil || len(promptResp.UpdatedInput) == 0) {
			promptResp.UpdatedInput = originalInput
		}

		// Marshal response to JSON
		jsonData, err := json.Marshal(promptResp)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("method", "HandleApprovalPrompt").
				Msg("Failed to marshal approval response")
			return nil, fmt.Errorf("failed to marshal approval response: %w", err)
		}
		// Log approval response
		s.logger.Info().
			Str("method", "HandleApprovalPrompt").
			Str("request_id", requestID).
			Str("behavior", promptResp.Behavior).
			Str("json_response", string(jsonData)).
			Interface("updated_input", promptResp.UpdatedInput).
			Msg("Returning approval response")

		result := &mcpsdk.CallToolResultFor[PermissionPromptResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: string(jsonData),
				},
			},
		}

		s.logger.Debug().
			Str("method", "HandleApprovalPrompt").
			Str("request_id", requestID).
			Str("content_type", "text/json").
			Int("content_length", len(jsonData)).
			Msg("CallToolResultFor created with Content only")

		return result, nil

	case <-time.After(5 * time.Minute):
		// Timeout - return deny
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		delete(s.approvalInputs, requestID)
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
		}, nil

	case <-ctx.Done():
		// Context cancelled
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		delete(s.approvalInputs, requestID)
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
