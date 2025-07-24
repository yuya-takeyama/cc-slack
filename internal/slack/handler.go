package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/slack-go/slack"

	"github.com/yuya-takeyama/cc-slack/internal/claude"
	"github.com/yuya-takeyama/cc-slack/internal/session"
	"github.com/yuya-takeyama/cc-slack/pkg/types"
)

const (
	MAX_SLACK_MESSAGE_LENGTH = 3000
)

// SlackOutputHandler handles Claude Code output and posts to Slack
type SlackOutputHandler struct {
	client         *slack.Client
	session        *session.Session
	processManager *claude.ProcessManager
}

func NewSlackOutputHandler(client *slack.Client, session *session.Session, processManager *claude.ProcessManager) *SlackOutputHandler {
	return &SlackOutputHandler{
		client:         client,
		session:        session,
		processManager: processManager,
	}
}

func (h *SlackOutputHandler) HandleMessage(msg interface{}) error {
	switch m := msg.(type) {
	case types.SystemMessage:
		return h.handleSystemMessage(m)
	case types.AssistantMessage:
		return h.handleAssistantMessage(m)
	case types.UserMessage:
		return h.handleUserMessage(m)
	case types.ResultMessage:
		return h.handleResultMessage(m)
	default:
		slog.Debug("Unknown message type", "message", msg)
		return nil
	}
}

func (h *SlackOutputHandler) handleSystemMessage(msg types.SystemMessage) error {
	if msg.Subtype == "init" {
		// Update session with actual session_id from Claude Code
		if msg.SessionID != "" && msg.SessionID != h.session.SessionID {
			oldID := h.session.SessionID
			// TODO: Update session manager with new session ID
			slog.Info("Updated session ID", "old", oldID, "new", msg.SessionID)
		}

		// Update available tools
		h.session.AvailableTools = msg.Tools

		// Post initialization message to Slack
		text := fmt.Sprintf("🚀 Claude Code セッション開始\n"+
			"セッションID: %s\n"+
			"作業ディレクトリ: %s\n"+
			"モデル: %s\n"+
			"利用可能ツール: %d個",
			msg.SessionID, msg.CWD, msg.Model, len(msg.Tools))

		return h.postToSlack(text)
	}

	return nil
}

func (h *SlackOutputHandler) handleAssistantMessage(msg types.AssistantMessage) error {
	text := h.formatAssistantMessage(msg)
	if text == "" {
		return nil
	}

	return h.postToSlack(text)
}

func (h *SlackOutputHandler) handleUserMessage(msg types.UserMessage) error {
	// User messages are tool results, typically not shown in Slack
	// but we might want to show them for debugging
	slog.Debug("Tool result received", "session_id", msg.SessionID)
	return nil
}

func (h *SlackOutputHandler) handleResultMessage(msg types.ResultMessage) error {
	if msg.Subtype == "success" {
		summary := fmt.Sprintf("✅ セッション完了\n"+
			"実行時間: %dms\n"+
			"ターン数: %d\n"+
			"コスト: $%.6f USD\n"+
			"使用トークン: 入力=%d, 出力=%d",
			msg.DurationMS,
			msg.NumTurns,
			msg.TotalCostUSD,
			msg.Usage.InputTokens,
			msg.Usage.OutputTokens)

		// Cost warning
		if msg.TotalCostUSD > 1.0 {
			summary += "\n⚠️ 高コストセッション"
		}

		return h.postToSlack(summary)
	} else if msg.Subtype == "error" {
		errorMsg := fmt.Sprintf("❌ セッションエラー\n結果: %s", msg.Result)
		return h.postToSlack(errorMsg)
	}

	return nil
}

func (h *SlackOutputHandler) formatAssistantMessage(msg types.AssistantMessage) string {
	var parts []string

	for _, content := range msg.Message.Content {
		switch content.Type {
		case "text":
			if content.Text != "" {
				parts = append(parts, content.Text)
			}
		case "tool_use":
			// Show tool usage in a user-friendly way
			toolDesc := fmt.Sprintf("🔧 *%s* を実行中...", content.Name)
			parts = append(parts, toolDesc)
		case "thinking":
			// Skip thinking blocks for now (or show them in debug mode)
			slog.Debug("Thinking block received", "content", content.Thinking[:min(100, len(content.Thinking))])
		}
	}

	return strings.Join(parts, "\n")
}

func (h *SlackOutputHandler) postToSlack(text string) error {
	if text == "" {
		return nil
	}

	// Truncate if too long
	text = h.truncateForSlack(text)

	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	if h.session.ThreadTS != "" {
		options = append(options, slack.MsgOptionTS(h.session.ThreadTS))
	}

	_, _, err := h.client.PostMessage(h.session.ChannelID, options...)
	if err != nil {
		slog.Error("Failed to post message to Slack", "error", err)
		return err
	}

	return nil
}

func (h *SlackOutputHandler) truncateForSlack(text string) string {
	if len(text) <= MAX_SLACK_MESSAGE_LENGTH {
		return text
	}

	return text[:MAX_SLACK_MESSAGE_LENGTH-100] + "\n\n... (省略) ..."
}

// Update the Bot to use Claude process integration
func (b *Bot) sendToClaudeProcess(session *session.Session, message string) error {
	if session.Process == nil {
		return fmt.Errorf("no Claude process running for session %s", session.SessionID)
	}

	processManager := claude.NewProcessManager(b.config.ClaudeCodePath)
	return processManager.SendMessage(session.Process, message)
}

func (b *Bot) processClaudeOutput(session *session.Session) {
	if session.Process == nil {
		slog.Error("No Claude process to monitor", "session_id", session.SessionID)
		return
	}

	processManager := claude.NewProcessManager(b.config.ClaudeCodePath)
	handler := NewSlackOutputHandler(b.client, session, processManager)

	// Start stderr monitoring in separate goroutine
	go processManager.ReadErrors(session.Process)

	// Process stdout (main JSON Lines stream)
	if err := processManager.ReadOutput(session.Process, handler); err != nil {
		slog.Error("Error reading Claude output", "error", err, "session_id", session.SessionID)
		
		// Post error to Slack
		errorMsg := fmt.Sprintf("❌ Claude Code プロセスでエラーが発生しました: %s", err.Error())
		if _, err := b.postMessage(session.ChannelID, errorMsg, session.ThreadTS); err != nil {
			slog.Error("Failed to post error message", "error", err)
		}
	}

	// Clean up session when process ends
	b.sessionManager.RemoveSession(session.SessionID)
	slog.Info("Session cleaned up", "session_id", session.SessionID)
}

// Update session creation to start Claude process
func (b *Bot) createSessionWithProcess(channelID, workDir string) (*session.Session, error) {
	session, err := b.sessionManager.CreateSession(channelID, workDir)
	if err != nil {
		return nil, err
	}

	// Start Claude Code process
	processManager := claude.NewProcessManager(b.config.ClaudeCodePath)
	process, err := processManager.StartProcess(context.Background(), workDir)
	if err != nil {
		b.sessionManager.RemoveSession(session.SessionID)
		return nil, fmt.Errorf("failed to start Claude process: %w", err)
	}

	session.Process = process
	return session, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}