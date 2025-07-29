package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

// RepositoryRouter uses Claude to route messages to appropriate repositories
type RepositoryRouter struct {
	logger    zerolog.Logger
	config    *config.Config
	repos     []config.RepositoryConfig
	processID string
}

// NewRepositoryRouter creates a new repository router
func NewRepositoryRouter(logger zerolog.Logger, cfg *config.Config, repos []config.RepositoryConfig) *RepositoryRouter {
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
			RepositoryPath: r.repos[0].Path,
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
  "repository_path": "<string>",
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

	// DEBUG: Write to debug file
	debugFile, _ := os.OpenFile("logs/debug/router-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if debugFile != nil {
		fmt.Fprintf(debugFile, "[%s] [DEBUG] Router LLM raw response: %s\n",
			time.Now().Format("2006-01-02 15:04:05"), result)
		defer debugFile.Close()
	}

	if err := json.Unmarshal([]byte(result), &routeResult); err != nil {
		return nil, fmt.Errorf("failed to parse routing result: %w", err)
	}

	if debugFile != nil {
		fmt.Fprintf(debugFile, "[%s] [DEBUG] Parsed route result - path: '%s', name: '%s', confidence: '%s', reason: '%s'\n",
			time.Now().Format("2006-01-02 15:04:05"),
			routeResult.RepositoryPath,
			routeResult.RepositoryName,
			routeResult.Confidence,
			routeResult.Reason)
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
		info := fmt.Sprintf("- Name: %s, Path: %s", repo.Name, repo.Path)
		if repo.DefaultBranch != "" {
			info += fmt.Sprintf(", Default Branch: %s", repo.DefaultBranch)
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
	// Create simplified process for routing (no MCP config = no MCP)
	args := []string{
		"--model", "sonnet", // Use model alias
		"--print",                       // Run once and exit
		"--system-prompt", systemPrompt, // Override default system prompt
	}

	// Execute Claude command
	cmd := exec.CommandContext(ctx, r.config.Claude.Executable, args...)
	cmd.Dir = r.config.WorkingDirectories.Default

	// Set stdin to the user prompt
	cmd.Stdin = strings.NewReader(userPrompt)

	// Run command and get output
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Include stderr in error message
			return "", fmt.Errorf("failed to execute Claude: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute Claude: %w", err)
	}

	// Parse response - look for JSON in output (might be wrapped in markdown code block)
	outputStr := string(output)

	// First, try to extract from markdown code block
	if strings.Contains(outputStr, "```json") {
		start := strings.Index(outputStr, "```json")
		if start != -1 {
			// Find the actual start of JSON (after newline)
			jsonStart := start + 7 // Skip "```json"
			// Skip any whitespace or newline after ```json
			for jsonStart < len(outputStr) && (outputStr[jsonStart] == '\n' || outputStr[jsonStart] == '\r' || outputStr[jsonStart] == ' ') {
				jsonStart++
			}

			end := strings.Index(outputStr[jsonStart:], "```")
			if end != -1 {
				jsonResponse := strings.TrimSpace(outputStr[jsonStart : jsonStart+end])
				return jsonResponse, nil
			}
		}
	}

	// Fallback: look for raw JSON
	lines := strings.Split(outputStr, "\n")
	var jsonStart, jsonEnd int = -1, -1

	// Find the first line that starts with {
	for i, line := range lines {
		if strings.TrimSpace(line) == "{" {
			jsonStart = i
			break
		}
	}

	// Find the last line that ends with }
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "}" {
			jsonEnd = i
			break
		}
	}

	if jsonStart != -1 && jsonEnd != -1 && jsonStart <= jsonEnd {
		// Join the lines to form the JSON
		jsonLines := lines[jsonStart : jsonEnd+1]
		jsonResponse := strings.Join(jsonLines, "\n")
		return jsonResponse, nil
	}

	return "", fmt.Errorf("no JSON response found in output: %s", outputStr)
}

// validateResult ensures the routing result is valid
func (r *RepositoryRouter) validateResult(result *slack.RouteResult) error {
	// Check if repository path exists
	found := false
	for _, repo := range r.repos {
		if repo.Path == result.RepositoryPath {
			found = true
			// Ensure name matches
			if result.RepositoryName != repo.Name {
				result.RepositoryName = repo.Name
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("repository with path %s not found", result.RepositoryPath)
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
