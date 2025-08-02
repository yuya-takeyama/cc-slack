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
	sendMessageCalls         []sendMessageCall
	getSessionByThreadCalls  []getSessionByThreadCall
	getSessionByThreadReturn *Session
	getSessionByThreadError  error
}

type createSessionCall struct {
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

func (m *MockSessionManager) CreateSession(ctx context.Context, channelID, threadTS, workDir, initialPrompt string) (bool, string, error) {
	m.createSessionCalls = append(m.createSessionCalls, createSessionCall{
		channelID:     channelID,
		threadTS:      threadTS,
		workDir:       workDir,
		initialPrompt: initialPrompt,
	})
	return false, "", nil
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

// createTestConfigWithWorkingDirs creates a config with working directories for testing
func createTestConfigWithWorkingDirs(dirs []config.WorkingDirectoryConfig) *config.Config {
	cfg := createTestConfig()
	cfg.WorkingDirs = dirs
	return cfg
}

func TestParseApprovalMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected *ApprovalInfo
	}{
		{
			name:    "WebFetch tool",
			message: "**Tool**: WebFetch \n **URL**: https://example.com \n **Content**: test prompt",
			expected: &ApprovalInfo{
				ToolName: "WebFetch",
				URL:      "https://example.com",
				Prompt:   "test prompt",
			},
		},
		{
			name:    "Bash tool",
			message: "**Tool**: Bash \n **Command**: ls -la \n **Description**: List files",
			expected: &ApprovalInfo{
				ToolName:    "Bash",
				Command:     "ls -la",
				Description: "List files",
			},
		},
		{
			name:    "Write tool",
			message: "**Tool**: Write \n **File path**: /tmp/test.txt",
			expected: &ApprovalInfo{
				ToolName: "Write",
				FilePath: "/tmp/test.txt",
			},
		},
		{
			name:    "Mixed content with extra spaces",
			message: "  **Tool**: WebFetch  \n  **URL**: https://example.com  ",
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
			message: "**Unknown**: value \n **Tool**: Test",
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
			expected: "*Tool execution permission required*\n\n*Tool:* WebFetch\n*URL:* <https://example.com>\n*Content:*\n```\ntest prompt\n```",
		},
		{
			name: "Bash tool",
			info: &ApprovalInfo{
				ToolName:    "Bash",
				Command:     "ls -la",
				Description: "List files",
			},
			expected: "*Tool execution permission required*\n\n*Tool:* Bash\n*Command:*\n```\nls -la\n```\n*Description:*\n```\nList files\n```",
		},
		{
			name: "Write tool",
			info: &ApprovalInfo{
				ToolName: "Write",
				FilePath: "/tmp/test.txt",
			},
			expected: "*Tool execution permission required*\n\n*Tool:* Write\n*File path:* `/tmp/test.txt`",
		},
		{
			name:     "Empty info",
			info:     &ApprovalInfo{},
			expected: "*Tool execution permission required*\n\n",
		},
		{
			name: "Tool name only",
			info: &ApprovalInfo{
				ToolName: "CustomTool",
			},
			expected: "*Tool execution permission required*\n\n*Tool:* CustomTool\n",
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

func TestDetermineWorkDir(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		channelID string
		expected  string
	}{
		{
			name: "single directory mode",
			config: &config.Config{
				WorkingDirs: []config.WorkingDirectoryConfig{
					{
						Name: "default",
						Path: "/home/user/project",
					},
				},
			},
			channelID: "C12345",
			expected:  "/home/user/project",
		},
		{
			name: "multi-directory mode returns empty",
			config: &config.Config{
				WorkingDirs: []config.WorkingDirectoryConfig{
					{
						Name: "project1",
						Path: "/home/user/project1",
					},
					{
						Name: "project2",
						Path: "/home/user/project2",
					},
				},
			},
			channelID: "C12345",
			expected:  "",
		},
		{
			name: "empty config returns empty in multi-directory mode",
			config: &config.Config{
				WorkingDirs: []config.WorkingDirectoryConfig{},
			},
			channelID: "C12345",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				config: tt.config,
			}
			got := handler.determineWorkDir(tt.channelID)
			if got != tt.expected {
				t.Errorf("determineWorkDir() = %q, want %q", got, tt.expected)
			}
		})
	}
}
