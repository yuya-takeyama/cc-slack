package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

// RepositoryRouter uses Claude to route messages to appropriate repositories
type RepositoryRouter struct {
	logger    zerolog.Logger
	config    *config.Config
	repos     []db.Repository
	processID string
}

// NewRepositoryRouter creates a new repository router
func NewRepositoryRouter(logger zerolog.Logger, cfg *config.Config, repos []db.Repository) *RepositoryRouter {
	return &RepositoryRouter{
		logger: logger,
		config: cfg,
		repos:  repos,
	}
}

// Route determines which repository should handle the message
func (r *RepositoryRouter) Route(ctx context.Context, channelID, message string) (*slack.RouteResult, error) {
	// If only one repository, skip AI routing
	if len(r.repos) == 1 {
		return &slack.RouteResult{
			RepositoryID:   r.repos[0].ID,
			RepositoryName: r.repos[0].Name,
			Confidence:     "high",
			Reason:         "Only one repository configured for this channel",
		}, nil
	}

	// Build system prompt with repository information
	systemPrompt := r.buildSystemPrompt()

	// Create routing prompt
	routingPrompt := fmt.Sprintf(`Based on the following message, determine which repository it should be routed to.

Message: %s

Respond with a JSON object in the following format:
{
  "repository_id": <number>,
  "repository_name": "<string>",
  "confidence": "<high|medium|low>",
  "reason": "<brief explanation>"
}`, message)

	// Execute Claude with custom system prompt
	result, err := r.executeClaude(ctx, systemPrompt, routingPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Claude router: %w", err)
	}

	// Parse JSON response
	var routeResult slack.RouteResult
	if err := json.Unmarshal([]byte(result), &routeResult); err != nil {
		return nil, fmt.Errorf("failed to parse routing result: %w", err)
	}

	// Validate result
	if err := r.validateResult(&routeResult); err != nil {
		return nil, fmt.Errorf("invalid routing result: %w", err)
	}

	return &routeResult, nil
}

// buildSystemPrompt creates the system prompt with repository information
func (r *RepositoryRouter) buildSystemPrompt() string {
	var repoInfo []string
	for _, repo := range r.repos {
		info := fmt.Sprintf("- ID: %d, Name: %s, Path: %s", repo.ID, repo.Name, repo.Path)
		if repo.DefaultBranch.Valid {
			info += fmt.Sprintf(", Default Branch: %s", repo.DefaultBranch.String)
		}
		repoInfo = append(repoInfo, info)
	}

	return fmt.Sprintf(`You are a repository router for a multi-repository Claude Code system.
Your task is to analyze incoming messages and determine which repository they should be routed to.

Available repositories:
%s

Guidelines:
1. Look for explicit repository names or related terms in the message
2. Consider the context and technical domain of the message
3. If unclear, choose the most likely repository with "medium" or "low" confidence
4. Always provide a brief reason for your choice

You must respond with valid JSON only.`, strings.Join(repoInfo, "\n"))
}

// executeClaude runs Claude with the routing prompt
func (r *RepositoryRouter) executeClaude(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Create simplified process for routing (no MCP needed)
	args := []string{
		"--model", "claude-3-5-sonnet-latest",
		"--no-mcp",
	}

	if systemPrompt != "" {
		// Save system prompt to temp file
		tmpFile, err := os.CreateTemp("", "router-prompt-*.txt")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(systemPrompt); err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to write system prompt: %w", err)
		}
		tmpFile.Close()

		args = append(args, "--system-prompt-file", tmpFile.Name())
	}

	// Execute Claude command
	cmd := exec.CommandContext(ctx, r.config.Claude.Executable, args...)
	cmd.Dir = r.config.WorkingDirectories.Default

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start Claude: %w", err)
	}

	// Write prompt to stdin
	if _, err := stdin.Write([]byte(userPrompt)); err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}
	stdin.Close()

	// Wait for command to complete and get output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute Claude: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// validateResult ensures the routing result is valid
func (r *RepositoryRouter) validateResult(result *slack.RouteResult) error {
	// Check if repository ID exists
	found := false
	for _, repo := range r.repos {
		if repo.ID == result.RepositoryID {
			found = true
			// Ensure name matches
			if result.RepositoryName != repo.Name {
				result.RepositoryName = repo.Name
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("repository with ID %d not found", result.RepositoryID)
	}

	// Validate confidence level
	switch result.Confidence {
	case "high", "medium", "low":
		// Valid
	default:
		result.Confidence = "medium" // Default if invalid
	}

	return nil
}

// GetProcessID returns the Claude process ID used for routing
func (r *RepositoryRouter) GetProcessID() string {
	return r.processID
}
