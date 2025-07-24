package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/yuya-takeyama/cc-slack/pkg/config"
	"github.com/yuya-takeyama/cc-slack/pkg/types"
)

// MCP JSON-RPC message structures
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Server implements MCP server with stdio transport
type Server struct {
	config          *config.Config
	input           *bufio.Scanner
	output          io.Writer
	approvalManager *ApprovalManager
	currentChannel  string // Track current Slack channel for approvals
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
		input:  bufio.NewScanner(os.Stdin),
		output: os.Stdout,
	}
}

func (s *Server) SetApprovalManager(manager *ApprovalManager) {
	s.approvalManager = manager
}

func (s *Server) SetCurrentChannel(channelID string) {
	s.currentChannel = channelID
}

func (s *Server) Run(ctx context.Context) error {
	slog.Info("Starting MCP server", "name", s.config.MCPServerName)

	for {
		select {
		case <-ctx.Done():
			slog.Info("MCP server shutting down")
			return nil
		default:
			if !s.input.Scan() {
				if err := s.input.Err(); err != nil {
					slog.Error("Error reading from stdin", "error", err)
					return err
				}
				slog.Info("EOF received, shutting down MCP server")
				return nil
			}

			line := s.input.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			if err := s.handleRequest(line); err != nil {
				slog.Error("Error handling request", "error", err)
			}
		}
	}
}

func (s *Server) handleRequest(line string) error {
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return s.sendError(nil, -32700, "Parse error", err.Error())
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return s.handleInitialized(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return s.sendError(req.ID, -32601, "Method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) error {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.config.MCPServerName,
			"version": "0.1.0",
		},
	}

	return s.sendResponse(req.ID, result)
}

func (s *Server) handleInitialized(req JSONRPCRequest) error {
	slog.Info("MCP client initialized")
	return nil
}

func (s *Server) handleToolsList(req JSONRPCRequest) error {
	tools := []map[string]interface{}{
		{
			"name": "approval_prompt",
			"description": "Custom permission prompt tool that asks for approval via Slack",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tool_name": map[string]interface{}{
						"type": "string",
						"description": "Name of the tool requesting approval",
					},
					"input": map[string]interface{}{
						"type": "object",
						"description": "Input parameters for the tool",
					},
					"context": map[string]interface{}{
						"type": "string",
						"description": "Context or reason for the tool execution",
					},
				},
				"required": []string{"tool_name"},
			},
		},
	}

	return s.sendResponse(req.ID, map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) handleToolsCall(req JSONRPCRequest) error {
	// Parse tool call parameters
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return s.sendError(req.ID, -32602, "Invalid params", err.Error())
	}

	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return s.sendError(req.ID, -32602, "Invalid params", err.Error())
	}

	switch params.Name {
	case "approval_prompt":
		return s.handleApprovalPrompt(req.ID, params.Arguments)
	default:
		return s.sendError(req.ID, -32601, "Tool not found", params.Name)
	}
}

func (s *Server) handleApprovalPrompt(id interface{}, args map[string]interface{}) error {
	// Extract approval request parameters
	var approvalReq types.ApprovalRequest
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return s.sendError(id, -32602, "Invalid arguments", err.Error())
	}

	if err := json.Unmarshal(argsJSON, &approvalReq); err != nil {
		return s.sendError(id, -32602, "Invalid arguments", err.Error())
	}

	var response types.ApprovalResponse

	// Use Slack approval if available
	if s.approvalManager != nil && s.currentChannel != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		approvalResponse, err := s.approvalManager.RequestApproVal(ctx, approvalReq, s.currentChannel)
		if err != nil {
			slog.Error("Failed to get approval from Slack", "error", err)
			response = types.ApprovalResponse{
				Behavior: "deny",
				Message:  fmt.Sprintf("Approval request failed: %s", err.Error()),
			}
		} else {
			response = approvalResponse
		}
	} else {
		// Fallback: auto-approve for now
		slog.Warn("No approval manager or channel configured, auto-approving", 
			"tool", approvalReq.ToolName)
		response = types.ApprovalResponse{
			Behavior: "allow",
			Message:  fmt.Sprintf("Tool %s auto-approved (no Slack integration)", approvalReq.ToolName),
		}
	}

	// Convert response to JSON string as required by MCP
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return s.sendError(id, -32603, "Internal error", err.Error())
	}

	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": string(responseJSON),
			},
		},
	}

	return s.sendResponse(id, result)
}

func (s *Server) sendResponse(id interface{}, result interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	return s.writeJSON(response)
}

func (s *Server) sendError(id interface{}, code int, message string, data interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	return s.writeJSON(response)
}

func (s *Server) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.output, "%s\n", data)
	return err
}