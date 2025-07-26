package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server and HTTP handler
type Server struct {
	mcp     *mcpsdk.Server
	handler *mcpsdk.StreamableHTTPHandler

	// Approval requests waiting for response
	approvalRequests map[string]chan ApprovalResponse
	approvalMu       sync.Mutex
}

// ApprovalRequest represents a request for user approval
type ApprovalRequest struct {
	Message  string `json:"message"`
	ToolName string `json:"toolName"`
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
	mcpsdk.AddTool(mcp, &mcpsdk.Tool{
		Name:        "approval_prompt",
		Description: "Request user approval via Slack",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"message": {
					Type:        "string",
					Description: "Message to display in Slack for approval",
				},
				"toolName": {
					Type:        "string",
					Description: "Name of the tool requesting approval",
				},
			},
			Required: []string{"message", "toolName"},
		},
	}, s.HandleApprovalPrompt)

	// Create StreamableHTTPHandler
	s.handler = mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		return mcp
	}, nil)

	return s, nil
}

// Handle processes MCP requests
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// HandleApprovalPrompt handles approval requests
func (s *Server) HandleApprovalPrompt(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ApprovalRequest]) (*mcpsdk.CallToolResultFor[any], error) {
	// Generate request ID
	requestID := fmt.Sprintf("approval_%d", time.Now().UnixNano())

	// Create channel for response
	respChan := make(chan ApprovalResponse, 1)
	s.approvalMu.Lock()
	s.approvalRequests[requestID] = respChan
	s.approvalMu.Unlock()

	// TODO: Send approval request to Slack
	// For now, we'll need to integrate with the Slack handler

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		// Clean up
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		s.approvalMu.Unlock()

		// Return response according to MCP format
		content := []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: fmt.Sprintf("Approval %s: %s", resp.Behavior, resp.Message),
			},
		}

		return &mcpsdk.CallToolResultFor[any]{
			Content: content,
		}, nil

	case <-time.After(5 * time.Minute):
		// Timeout
		s.approvalMu.Lock()
		delete(s.approvalRequests, requestID)
		s.approvalMu.Unlock()

		return &mcpsdk.CallToolResultFor[any]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: "Approval request timed out",
				},
			},
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
