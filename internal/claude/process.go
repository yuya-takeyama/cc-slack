package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/yuya-takeyama/cc-slack/pkg/types"
)

// ProcessManager handles Claude Code process lifecycle
type ProcessManager struct {
	claudeCodePath string
}

func NewProcessManager(claudeCodePath string) *ProcessManager {
	return &ProcessManager{
		claudeCodePath: claudeCodePath,
	}
}

// StartProcess starts a new Claude Code process
func (pm *ProcessManager) StartProcess(ctx context.Context, workDir string) (*types.ClaudeProcess, error) {
	cmd := exec.CommandContext(ctx, pm.claudeCodePath,
		"--print",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	)
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start Claude Code process: %w", err)
	}

	process := &types.ClaudeProcess{
		Cmd:       cmd,
		Stdin:     stdin,
		Stdout:    bufio.NewScanner(stdout),
		Stderr:    bufio.NewScanner(stderr),
		WorkDir:   workDir,
		CreatedAt: time.Now(),
	}

	slog.Info("Started Claude Code process", 
		"pid", cmd.Process.Pid, 
		"workdir", workDir)

	return process, nil
}

// SendMessage sends a message to Claude Code process via stdin
func (pm *ProcessManager) SendMessage(process *types.ClaudeProcess, message string) error {
	input := types.InputMessage{
		Type: "message",
		Message: types.HumanMessage{
			Type:    "human",
			Content: message,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input message: %w", err)
	}

	slog.Debug("Sending message to Claude Code", "message", message)

	_, err = fmt.Fprintf(process.Stdin, "%s\n", data)
	return err
}

// ReadOutput reads and parses JSON Lines output from Claude Code process
func (pm *ProcessManager) ReadOutput(process *types.ClaudeProcess, handler OutputHandler) error {
	for process.Stdout.Scan() {
		line := process.Stdout.Bytes()
		
		if len(line) == 0 {
			continue
		}

		slog.Debug("Received output from Claude Code", "line", string(line))

		msg, err := types.ParseMessage(line)
		if err != nil {
			slog.Error("Failed to parse JSON message", "error", err, "line", string(line))
			continue
		}

		if err := handler.HandleMessage(msg); err != nil {
			slog.Error("Failed to handle message", "error", err)
		}
	}

	if err := process.Stdout.Err(); err != nil {
		return fmt.Errorf("error reading stdout: %w", err)
	}

	return nil
}

// ReadErrors reads stderr from Claude Code process
func (pm *ProcessManager) ReadErrors(process *types.ClaudeProcess) {
	for process.Stderr.Scan() {
		line := process.Stderr.Text()
		slog.Warn("Claude Code stderr", "line", line)
	}

	if err := process.Stderr.Err(); err != nil {
		slog.Error("Error reading stderr", "error", err)
	}
}

// StopProcess gracefully stops the Claude Code process
func (pm *ProcessManager) StopProcess(process *types.ClaudeProcess) error {
	if process == nil {
		return nil
	}

	slog.Info("Stopping Claude Code process", "pid", process.Cmd.Process.Pid)

	// Close stdin to signal process to exit
	if process.Stdin != nil {
		process.Stdin.Close()
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- process.Cmd.Wait()
	}()

	select {
	case err := <-done:
		slog.Info("Claude Code process exited", "error", err)
		return err
	case <-time.After(10 * time.Second):
		slog.Warn("Claude Code process did not exit gracefully, killing")
		if err := process.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		return nil
	}
}

// OutputHandler interface for handling Claude Code output messages
type OutputHandler interface {
	HandleMessage(msg interface{}) error
}

// Simple output handler that logs messages
type LoggingOutputHandler struct{}

func (h *LoggingOutputHandler) HandleMessage(msg interface{}) error {
	switch m := msg.(type) {
	case types.SystemMessage:
		slog.Info("System message", "subtype", m.Subtype, "session_id", m.SessionID)
	case types.AssistantMessage:
		slog.Info("Assistant message", "session_id", m.SessionID)
	case types.UserMessage:
		slog.Info("User message", "session_id", m.SessionID)
	case types.ResultMessage:
		slog.Info("Result message", "subtype", m.Subtype, "session_id", m.SessionID)
	default:
		slog.Info("Unknown message type", "message", msg)
	}
	return nil
}