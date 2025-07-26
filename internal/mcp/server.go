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

// Server implements the MCP server with Streamable HTTP transport
type Server struct {
	mcp      *mcpsdk.Server
	sessions map[string]*Session
	mu       sync.RWMutex

	// Approval requests waiting for response
	approvalRequests map[string]chan ApprovalResponse
	approvalMu       sync.Mutex
}

// Session represents a connected MCP client session
type Session struct {
	ID       string
	Messages chan []byte
	ctx      context.Context
	cancel   context.CancelFunc
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
		sessions:         make(map[string]*Session),
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

	return s, nil
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

// Handle processes MCP requests based on HTTP method
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleStream(w, r)
	case http.MethodPost:
		s.handleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStream handles SSE streaming connections
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Get or create session
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	session := s.getOrCreateSession(sessionID)

	// Send initial connection event
	fmt.Fprintf(w, "data: {\"type\":\"connection\",\"session_id\":\"%s\"}\n\n", sessionID)
	flusher.Flush()

	// Stream messages to client
	for {
		select {
		case msg := <-session.Messages:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			s.removeSession(sessionID)
			return
		case <-session.ctx.Done():
			return
		}
	}
}

// handleMessage handles individual MCP messages
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Process MCP request through the MCP server
	// For now, just acknowledge
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  map[string]interface{}{"status": "ok"},
		"id":      req["id"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getOrCreateSession retrieves or creates a session
func (s *Server) getOrCreateSession(sessionID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		return session
	}

	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		ID:       sessionID,
		Messages: make(chan []byte, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
	s.sessions[sessionID] = session
	return session
}

// removeSession removes a session
func (s *Server) removeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.cancel()
		close(session.Messages)
		delete(s.sessions, sessionID)
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

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// TODO: Use UUID library
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
