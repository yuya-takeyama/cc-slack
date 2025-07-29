package messages

import (
	"fmt"
	"strings"
	"time"
)

// FormatSessionStartMessage formats the session start message
func FormatSessionStartMessage(sessionID, cwd, model string) string {
	return fmt.Sprintf("🚀 Claude Code session started\n"+
		"Session ID: `%s`\n"+
		"Working directory: %s\n"+
		"Model: %s",
		sessionID, cwd, model)
}

// FormatSessionCompleteMessage formats the session completion message
func FormatSessionCompleteMessage(sessionID string, duration time.Duration, turns int, cost float64, inputTokens, outputTokens int) string {
	text := fmt.Sprintf("✅ Session completed\n"+
		"Session ID: `%s`\n"+
		"Duration: %s\n"+
		"Turns: %d\n"+
		"Cost: $%.6f USD\n"+
		"Tokens used: input=%d, output=%d",
		sessionID,
		FormatDuration(duration),
		turns,
		cost,
		inputTokens,
		outputTokens)

	// Cost warning
	if cost > 1.0 {
		text += "\n⚠️ High cost session"
	}

	return text
}

// FormatTimeoutMessage formats the timeout message
func FormatTimeoutMessage(idleMinutes int, sessionID string) string {
	return fmt.Sprintf("⏰ Session timed out\n"+
		"Idle time: %d minutes\n"+
		"Session ID: `%s`\n\n"+
		"To start a new session, please mention me again.",
		idleMinutes, sessionID)
}

// FormatBashToolMessage formats the Bash tool message
func FormatBashToolMessage(command string) string {
	// Escape triple backticks in command
	escapedCmd := strings.ReplaceAll(command, "```", "\\`\\`\\`")
	// Format command as code block
	return fmt.Sprintf("```\n%s\n```", escapedCmd)
}

// FormatReadToolMessage formats the Read tool message
func FormatReadToolMessage(filePath string, offset, limit int) string {
	if offset > 0 || limit > 0 {
		// Include line range information when offset or limit is specified
		if offset > 0 && limit > 0 {
			return fmt.Sprintf("Reading `%s` (lines %d-%d)", filePath, offset, offset+limit-1)
		} else if offset > 0 {
			return fmt.Sprintf("Reading `%s` (from line %d)", filePath, offset)
		} else {
			return fmt.Sprintf("Reading `%s` (first %d lines)", filePath, limit)
		}
	}
	// Default format when reading entire file
	return fmt.Sprintf("Reading `%s`", filePath)
}

// FormatGrepToolMessage formats the Grep tool message
func FormatGrepToolMessage(pattern, path string) string {
	if path != "" {
		return fmt.Sprintf("Searching for `%s` in `%s`", pattern, path)
	}
	return fmt.Sprintf("Searching for `%s`", pattern)
}

// FormatEditToolMessage formats the Edit/MultiEdit tool message
func FormatEditToolMessage(filePath string) string {
	return fmt.Sprintf("Editing `%s`", filePath)
}

// FormatWriteToolMessage formats the Write tool message
func FormatWriteToolMessage(filePath string) string {
	return fmt.Sprintf("Writing `%s`", filePath)
}

// FormatLSToolMessage formats the LS tool message
func FormatLSToolMessage(path string) string {
	return fmt.Sprintf("Listing `%s`", path)
}

// FormatGlobToolMessage formats the Glob tool message
func FormatGlobToolMessage(pattern string) string {
	return fmt.Sprintf("`%s`", pattern)
}

// FormatTaskToolMessage formats the Task tool message
func FormatTaskToolMessage(description, prompt string) string {
	// Truncate prompt if too long
	const maxPromptLength = 500
	truncatedPrompt := prompt
	if len(prompt) > maxPromptLength {
		truncatedPrompt = prompt[:maxPromptLength] + "..."
	}

	// Escape triple backticks in prompt
	escapedPrompt := strings.ReplaceAll(truncatedPrompt, "```", "\\`\\`\\`")

	return fmt.Sprintf("Task: %s\n```\n%s\n```", description, escapedPrompt)
}

// FormatWebFetchToolMessage formats the WebFetch tool message
func FormatWebFetchToolMessage(url, prompt string) string {
	// Truncate prompt if too long
	const maxPromptLength = 300
	truncatedPrompt := prompt
	if len(prompt) > maxPromptLength {
		truncatedPrompt = prompt[:maxPromptLength] + "..."
	}

	// Escape triple backticks in prompt
	escapedPrompt := strings.ReplaceAll(truncatedPrompt, "```", "\\`\\`\\`")

	return fmt.Sprintf("Fetching: <%s>\n```\n%s\n```", url, escapedPrompt)
}

// FormatWebSearchToolMessage formats the WebSearch tool message
func FormatWebSearchToolMessage(query string) string {
	// Escape triple backticks in query
	escapedQuery := strings.ReplaceAll(query, "```", "\\`\\`\\`")

	return fmt.Sprintf("Searching web for: `%s`", escapedQuery)
}

// FormatCompletionMessage formats the completion message with session info
func FormatCompletionMessage(sessionID string, turns int, cost float64) string {
	text := fmt.Sprintf("✅ Session completed\n"+
		"Session ID: `%s`\n"+
		"Turns: %d\n"+
		"Cost: $%.6f USD",
		sessionID, turns, cost)

	// Cost warning
	if cost > 1.0 {
		text += "\n⚠️ High cost session"
	}

	return text
}

// FormatErrorMessage formats the error completion message
func FormatErrorMessage(sessionID string) string {
	return fmt.Sprintf("❌ Session ended with error\n"+
		"Session ID: `%s`", sessionID)
}

// FormatDuration converts duration to human-readable string
// Examples:
//   - 5s -> "5s"
//   - 2m5s -> "2m5s"
//   - 1h1m5s -> "1h1m5s"
func FormatDuration(d time.Duration) string {
	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm%ds", minutes, remainingSeconds)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	return fmt.Sprintf("%dh%dm%ds", hours, remainingMinutes, remainingSeconds)
}
