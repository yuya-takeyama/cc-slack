package types

import (
	"bufio"
	"io"
	"os/exec"
	"time"
)

// Session represents a Claude Code session
type Session struct {
	SessionID      string        `json:"session_id"`
	ThreadTS       string        `json:"thread_ts"`
	ChannelID      string        `json:"channel_id"`
	WorkDir        string        `json:"work_dir"`
	Process        *ClaudeProcess `json:"-"`
	AvailableTools []string      `json:"available_tools"`
	CreatedAt      time.Time     `json:"created_at"`
	LastActive     time.Time     `json:"last_active"`
}

// ClaudeProcess represents a running Claude Code process
type ClaudeProcess struct {
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser
	Stdout    *bufio.Scanner
	Stderr    *bufio.Scanner
	WorkDir   string
	CreatedAt time.Time
}

// ApprovalRequest represents a request for tool approval
type ApprovalRequest struct {
	ToolName string      `json:"tool_name"`
	Input    interface{} `json:"input"`
	Context  string      `json:"context"`
}

// ApprovalResponse represents the response to an approval request
type ApprovalResponse struct {
	Behavior     string      `json:"behavior"` // "allow" or "deny"
	Message      string      `json:"message,omitempty"`
	UpdatedInput interface{} `json:"updatedInput,omitempty"`
}