package slack

import (
	"context"
	"testing"

	"github.com/yuya-takeyama/cc-slack/internal/config"
)

func TestRemoveBotMentionFromText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		botUserID string
		expected  string
	}{
		{
			name:      "simple mention",
			input:     "<@U123456> hello",
			botUserID: "U123456",
			expected:  "hello",
		},
		{
			name:      "mention with no space",
			input:     "<@U123456>hello",
			botUserID: "U123456",
			expected:  "hello",
		},
		{
			name:      "no mention",
			input:     "hello world",
			botUserID: "U123456",
			expected:  "hello world",
		},
		{
			name:      "mention only",
			input:     "<@U123456>",
			botUserID: "U123456",
			expected:  "",
		},
		{
			name:      "mention with extra spaces",
			input:     "<@U123456>   hello world",
			botUserID: "U123456",
			expected:  "hello world",
		},
		{
			name:      "multiple words after mention",
			input:     "<@U123456> hello world test",
			botUserID: "U123456",
			expected:  "hello world test",
		},
		{
			name:      "empty string",
			input:     "",
			botUserID: "U123456",
			expected:  "",
		},
		{
			name:      "only spaces",
			input:     "   ",
			botUserID: "U123456",
			expected:  "",
		},
		{
			name:      "mention with newline",
			input:     "<@U123456>\nhello",
			botUserID: "U123456",
			expected:  "hello",
		},
		{
			name:      "incomplete mention",
			input:     "<@U123456 hello",
			botUserID: "U123456",
			expected:  "<@U123456 hello",
		},
		{
			name:      "wrong bot id",
			input:     "<@U999999> hello",
			botUserID: "U123456",
			expected:  "<@U999999> hello",
		},
		{
			name:      "empty bot id",
			input:     "<@U123456> hello",
			botUserID: "",
			expected:  "<@U123456> hello",
		},
		{
			name:      "mention in middle",
			input:     "hello <@U123456> world",
			botUserID: "U123456",
			expected:  "hello  world",
		},
		{
			name:      "multiple mentions",
			input:     "<@U123456> hello <@U123456> world",
			botUserID: "U123456",
			expected:  "hello  world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveBotMentionFromText(tt.input, tt.botUserID)
			if got != tt.expected {
				t.Errorf("RemoveBotMentionFromText(%q, %q) = %q, want %q", tt.input, tt.botUserID, got, tt.expected)
			}
		})
	}
}

// MockSessionManager implements SessionManager for testing
type MockSessionManager struct {
	createSessionCalls       []createSessionCall
	createSessionWithResume  []createSessionWithResumeCall
	sendMessageCalls         []sendMessageCall
	getSessionByThreadCalls  []getSessionByThreadCall
	getSessionByThreadReturn *Session
	getSessionByThreadError  error
}

type createSessionCall struct {
	channelID string
	threadTS  string
	workDir   string
}

type createSessionWithResumeCall struct {
	channelID     string
	threadTS      string
	workDir       string
	initialPrompt string
}

type sendMessageCall struct {
	sessionID string
	message   string
}

type getSessionByThreadCall struct {
	channelID string
	threadTS  string
}

func (m *MockSessionManager) GetSessionByThread(channelID, threadTS string) (*Session, error) {
	m.getSessionByThreadCalls = append(m.getSessionByThreadCalls, getSessionByThreadCall{
		channelID: channelID,
		threadTS:  threadTS,
	})
	return m.getSessionByThreadReturn, m.getSessionByThreadError
}

func (m *MockSessionManager) CreateSession(channelID, threadTS, workDir string) (*Session, error) {
	m.createSessionCalls = append(m.createSessionCalls, createSessionCall{
		channelID: channelID,
		threadTS:  threadTS,
		workDir:   workDir,
	})
	return &Session{
		SessionID: "test-session",
		ChannelID: channelID,
		ThreadTS:  threadTS,
		WorkDir:   workDir,
	}, nil
}

func (m *MockSessionManager) CreateSessionWithResume(ctx context.Context, channelID, threadTS, workDir, initialPrompt string) (*Session, bool, string, error) {
	m.createSessionWithResume = append(m.createSessionWithResume, createSessionWithResumeCall{
		channelID:     channelID,
		threadTS:      threadTS,
		workDir:       workDir,
		initialPrompt: initialPrompt,
	})
	return &Session{
		SessionID: "test-session",
		ChannelID: channelID,
		ThreadTS:  threadTS,
		WorkDir:   workDir,
	}, false, "", nil
}

func (m *MockSessionManager) SendMessage(sessionID, message string) error {
	m.sendMessageCalls = append(m.sendMessageCalls, sendMessageCall{
		sessionID: sessionID,
		message:   message,
	})
	return nil
}

// createTestConfig creates a minimal config for testing
func createTestConfig() *config.Config {
	return &config.Config{
		Slack: config.SlackConfig{
			BotToken:      "test-token",
			SigningSecret: "test-secret",
			MessageFilter: config.MessageFilterConfig{
				Enabled:        true,
				RequireMention: true,
			},
			FileUpload: config.FileUploadConfig{
				Enabled:   false,
				ImagesDir: "/tmp/test-images",
			},
		},
	}
}

func TestParseApprovalMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected *ApprovalInfo
	}{
		{
			name:    "WebFetch tool",
			message: "**ツール**: WebFetch \n **URL**: https://example.com \n **内容**: test prompt",
			expected: &ApprovalInfo{
				ToolName: "WebFetch",
				URL:      "https://example.com",
				Prompt:   "test prompt",
			},
		},
		{
			name:    "Bash tool",
			message: "**ツール**: Bash \n **コマンド**: ls -la \n **説明**: List files",
			expected: &ApprovalInfo{
				ToolName:    "Bash",
				Command:     "ls -la",
				Description: "List files",
			},
		},
		{
			name:    "Write tool",
			message: "**ツール**: Write \n **ファイルパス**: /tmp/test.txt",
			expected: &ApprovalInfo{
				ToolName: "Write",
				FilePath: "/tmp/test.txt",
			},
		},
		{
			name:    "Mixed content with extra spaces",
			message: "  **ツール**: WebFetch  \n  **URL**: https://example.com  ",
			expected: &ApprovalInfo{
				ToolName: "WebFetch",
				URL:      "https://example.com",
			},
		},
		{
			name:     "Empty message",
			message:  "",
			expected: &ApprovalInfo{},
		},
		{
			name:    "Unknown fields",
			message: "**Unknown**: value \n **ツール**: Test",
			expected: &ApprovalInfo{
				ToolName: "Test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseApprovalMessage(tt.message)
			if got.ToolName != tt.expected.ToolName {
				t.Errorf("ToolName = %q, want %q", got.ToolName, tt.expected.ToolName)
			}
			if got.URL != tt.expected.URL {
				t.Errorf("URL = %q, want %q", got.URL, tt.expected.URL)
			}
			if got.Prompt != tt.expected.Prompt {
				t.Errorf("Prompt = %q, want %q", got.Prompt, tt.expected.Prompt)
			}
			if got.Command != tt.expected.Command {
				t.Errorf("Command = %q, want %q", got.Command, tt.expected.Command)
			}
			if got.Description != tt.expected.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.expected.Description)
			}
			if got.FilePath != tt.expected.FilePath {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.expected.FilePath)
			}
		})
	}
}

func TestBuildApprovalMarkdownText(t *testing.T) {
	tests := []struct {
		name     string
		info     *ApprovalInfo
		expected string
	}{
		{
			name: "WebFetch tool",
			info: &ApprovalInfo{
				ToolName: "WebFetch",
				URL:      "https://example.com",
				Prompt:   "test prompt",
			},
			expected: "*ツールの実行許可が必要です*\n\n*ツール:* WebFetch\n*URL:* <https://example.com>\n*内容:*\n```\ntest prompt\n```",
		},
		{
			name: "Bash tool",
			info: &ApprovalInfo{
				ToolName:    "Bash",
				Command:     "ls -la",
				Description: "List files",
			},
			expected: "*ツールの実行許可が必要です*\n\n*ツール:* Bash\n*コマンド:*\n```\nls -la\n```\n*説明:*\n```\nList files\n```",
		},
		{
			name: "Write tool",
			info: &ApprovalInfo{
				ToolName: "Write",
				FilePath: "/tmp/test.txt",
			},
			expected: "*ツールの実行許可が必要です*\n\n*ツール:* Write\n*ファイルパス:* `/tmp/test.txt`",
		},
		{
			name:     "Empty info",
			info:     &ApprovalInfo{},
			expected: "*ツールの実行許可が必要です*\n\n",
		},
		{
			name: "Tool name only",
			info: &ApprovalInfo{
				ToolName: "CustomTool",
			},
			expected: "*ツールの実行許可が必要です*\n\n*ツール:* CustomTool\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildApprovalMarkdownText(tt.info)
			if got != tt.expected {
				t.Errorf("buildApprovalMarkdownText() = %q, want %q", got, tt.expected)
			}
		})
	}
}
