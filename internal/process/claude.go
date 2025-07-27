package process

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ClaudeProcess represents a running Claude Code process
type ClaudeProcess struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	stderr     *bufio.Scanner
	sessionID  string
	workDir    string
	configPath string
	createdAt  time.Time
	mu         sync.Mutex
	handlers   MessageHandlers
	logger     zerolog.Logger
	logFile    *os.File
}

// MessageHandlers contains callback functions for different message types
type MessageHandlers struct {
	OnSystem    func(msg SystemMessage) error
	OnAssistant func(msg AssistantMessage) error
	OnUser      func(msg UserMessage) error
	OnResult    func(msg ResultMessage) error
	OnError     func(err error)
}

// Message types from Claude Code
type BaseMessage struct {
	Type            string `json:"type"`
	SessionID       string `json:"session_id,omitempty"`
	ParentToolUseID string `json:"parent_tool_use_id,omitempty"`
}

type SystemMessage struct {
	BaseMessage
	Subtype        string      `json:"subtype"`
	CWD            string      `json:"cwd,omitempty"`
	Tools          []string    `json:"tools,omitempty"`
	MCPServers     interface{} `json:"mcp_servers,omitempty"` // Can be array or object
	Model          string      `json:"model,omitempty"`
	PermissionMode string      `json:"permissionMode,omitempty"`
	APIKeySource   string      `json:"apiKeySource,omitempty"`
}

type AssistantMessage struct {
	BaseMessage
	Message struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type     string                 `json:"type"`
			Text     string                 `json:"text,omitempty"`
			Thinking string                 `json:"thinking,omitempty"`
			ID       string                 `json:"id,omitempty"`
			Name     string                 `json:"name,omitempty"`
			Input    map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type UserMessage struct {
	BaseMessage
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			ToolUseID string      `json:"tool_use_id,omitempty"`
			Type      string      `json:"type"`
			Content   interface{} `json:"content,omitempty"` // Can be string or array
			IsError   bool        `json:"is_error,omitempty"`
		} `json:"content"`
	} `json:"message"`
}

type ResultMessage struct {
	BaseMessage
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMS   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Options for creating a new Claude process
type Options struct {
	WorkDir              string
	MCPBaseURL           string
	PermissionPromptTool string // MCP tool name for permission prompts (default: mcp__cc-slack__approval_prompt)
	// Must follow pattern: mcp__<serverName>__<toolName>
	Handlers MessageHandlers
}

// NewClaudeProcess creates and starts a new Claude Code process
func NewClaudeProcess(ctx context.Context, opts Options) (*ClaudeProcess, error) {
	// Set default permission prompt tool if not specified
	// IMPORTANT: Must match the tool name registered in MCP server
	// Format: mcp__<serverName>__<toolName>
	if opts.PermissionPromptTool == "" {
		opts.PermissionPromptTool = "mcp__cc-slack__approval_prompt"
	}

	// Create logs directory
	logDir := filepath.Join("logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create log file with timestamp
	logFileName := generateLogFileName(time.Now())
	logPath := filepath.Join(logDir, logFileName)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Setup logger
	logger := zerolog.New(logFile).With().
		Timestamp().
		Str("component", "claude_process").
		Logger()

	// Create MCP config file
	configPath, err := createMCPConfig(opts.MCPBaseURL)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to create MCP config: %w", err)
	}

	// Log MCP configuration for debugging
	logger.Info().
		Str("mcp_base_url", opts.MCPBaseURL).
		Str("permission_prompt_tool", opts.PermissionPromptTool).
		Str("config_path", configPath).
		Msg("Created MCP configuration")

	// Prepare command
	cmd := exec.CommandContext(ctx, "claude",
		"--mcp-config", configPath,
		"--permission-prompt-tool", opts.PermissionPromptTool,
		"--print",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	)
	cmd.Dir = opts.WorkDir

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(configPath)
		logFile.Close()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(configPath)
		logFile.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		os.Remove(configPath)
		logFile.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		os.Remove(configPath)
		logFile.Close()
		return nil, fmt.Errorf("failed to start claude process: %w", err)
	}

	p := &ClaudeProcess{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewScanner(stdout),
		stderr:     bufio.NewScanner(stderr),
		workDir:    opts.WorkDir,
		configPath: configPath,
		createdAt:  time.Now(),
		handlers:   opts.Handlers,
		logger:     logger,
		logFile:    logFile,
	}

	// Start reading stdout and stderr
	go p.readStdout()
	go p.readStderr()

	return p, nil
}

// SendMessage sends a message to Claude Code
func (p *ClaudeProcess) SendMessage(message string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	input := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": message,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log outgoing message
	p.logger.Info().
		Str("type", "claude_message").
		Str("direction", "sent").
		RawJSON("raw", data).
		Msg("Sent message to Claude")

	_, err = p.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	return nil
}

// readStdout reads and processes stdout messages
func (p *ClaudeProcess) readStdout() {
	for p.stdout.Scan() {
		line := p.stdout.Bytes()
		if err := p.processJSONLine(line); err != nil {
			if p.handlers.OnError != nil {
				p.handlers.OnError(fmt.Errorf("failed to process JSON line: %w", err))
			}
		}
	}

	if err := p.stdout.Err(); err != nil && p.handlers.OnError != nil {
		p.handlers.OnError(fmt.Errorf("stdout scanner error: %w", err))
	}
}

// readStderr reads and logs stderr messages
func (p *ClaudeProcess) readStderr() {
	for p.stderr.Scan() {
		line := p.stderr.Text()
		// Log stderr output
		p.logger.Warn().
			Str("type", "claude_stderr").
			Str("message", line).
			Msg("Claude stderr output")
	}
}

// processJSONLine processes a single JSON line from stdout
func (p *ClaudeProcess) processJSONLine(line []byte) error {
	// Log raw JSON for debugging
	p.logger.Debug().
		Str("type", "claude_message").
		Str("direction", "received").
		Bytes("raw", line).
		Msg("Received message from Claude")

	var base BaseMessage
	if err := json.Unmarshal(line, &base); err != nil {
		p.logger.Error().
			Err(err).
			Str("type", "parse_error").
			Bytes("raw", line).
			Msg("Failed to parse JSON message")
		return err
	}

	switch base.Type {
	case "system":
		var msg SystemMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			p.logger.Error().
				Err(err).
				Str("type", "parse_error").
				Str("message_type", "system").
				Bytes("raw", line).
				Msg("Failed to parse system message")
			return err
		}
		if msg.Subtype == "init" && msg.SessionID != "" {
			p.sessionID = msg.SessionID
		}
		if p.handlers.OnSystem != nil {
			return p.handlers.OnSystem(msg)
		}

	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return err
		}
		if p.handlers.OnAssistant != nil {
			return p.handlers.OnAssistant(msg)
		}

	case "user":
		var msg UserMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return err
		}
		if p.handlers.OnUser != nil {
			return p.handlers.OnUser(msg)
		}

	case "result":
		var msg ResultMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return err
		}
		if p.handlers.OnResult != nil {
			return p.handlers.OnResult(msg)
		}

	default:
		// Unknown message type, log it
		p.logger.Warn().
			Str("message_type", base.Type).
			Msg("Unknown message type received")
	}

	return nil
}

// Close terminates the Claude process and cleans up resources
func (p *ClaudeProcess) Close() error {
	// Close stdin to signal we're done
	p.stdin.Close()

	// Wait for process to exit
	err := p.cmd.Wait()

	// Clean up config file
	if p.configPath != "" {
		os.Remove(p.configPath)
	}

	// Close log file
	if p.logFile != nil {
		p.logFile.Close()
	}

	return err
}

// SessionID returns the session ID assigned by Claude Code
func (p *ClaudeProcess) SessionID() string {
	return p.sessionID
}

// createMCPConfig creates a temporary MCP configuration file
func createMCPConfig(baseURL string) (string, error) {
	config := buildMCPConfig(baseURL)

	// Create temp directory if needed
	tmpDir := filepath.Join(os.TempDir(), "cc-slack")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	// Create temp file
	tmpfile, err := os.CreateTemp(tmpDir, "claude-config-*.json")
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	// Write config
	if err := json.NewEncoder(tmpfile).Encode(config); err != nil {
		os.Remove(tmpfile.Name())
		return "", err
	}

	return tmpfile.Name(), nil
}

// generateLogFileName generates a log file name with timestamp
func generateLogFileName(t time.Time) string {
	return fmt.Sprintf("claude-%s.log", t.Format("20060102-150405"))
}

// buildMCPConfig builds the MCP configuration object
func buildMCPConfig(baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cc-slack": map[string]interface{}{
				"type": "http",
				"url":  fmt.Sprintf("%s/mcp", baseURL),
			},
		},
	}
}
