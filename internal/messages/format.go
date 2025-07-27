package messages

import (
	"fmt"
	"strings"
	"time"
)

// FormatSessionStartMessage formats the session start message
func FormatSessionStartMessage(sessionID, cwd, model string) string {
	return fmt.Sprintf("🚀 Claude Code セッション開始\n"+
		"セッションID: %s\n"+
		"作業ディレクトリ: %s\n"+
		"モデル: %s",
		sessionID, cwd, model)
}

// FormatSessionCompleteMessage formats the session completion message
func FormatSessionCompleteMessage(duration time.Duration, turns int, cost float64, inputTokens, outputTokens int) string {
	text := fmt.Sprintf("✅ セッション完了\n"+
		"実行時間: %s\n"+
		"ターン数: %d\n"+
		"コスト: $%.6f USD\n"+
		"使用トークン: 入力=%d, 出力=%d",
		FormatDuration(duration),
		turns,
		cost,
		inputTokens,
		outputTokens)

	// Cost warning
	if cost > 1.0 {
		text += "\n⚠️ 高コストセッション"
	}

	return text
}

// FormatTimeoutMessage formats the timeout message
func FormatTimeoutMessage(idleMinutes int, sessionID string) string {
	return fmt.Sprintf("⏰ セッションがタイムアウトしました\n"+
		"アイドル時間: %d分\n"+
		"セッションID: %s\n\n"+
		"新しいセッションを開始するには、再度メンションしてください。",
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

// FormatDuration converts duration to human-readable string
// Examples:
//   - 5s -> "5秒"
//   - 2m5s -> "2分5秒"
//   - 1h1m5s -> "1時間1分5秒"
func FormatDuration(d time.Duration) string {
	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%d分%d秒", minutes, remainingSeconds)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	return fmt.Sprintf("%d時間%d分%d秒", hours, remainingMinutes, remainingSeconds)
}
