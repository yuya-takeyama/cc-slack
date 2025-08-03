package session

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestTrimNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No newlines",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "Leading newlines",
			input:    "\n\nHello World",
			expected: "Hello World",
		},
		{
			name:     "Trailing newlines",
			input:    "Hello World\n\n",
			expected: "Hello World",
		},
		{
			name:     "Both leading and trailing newlines",
			input:    "\n\nHello World\n\n",
			expected: "Hello World",
		},
		{
			name:     "Massive amount of newlines like in the bug",
			input:    "\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n良い！テストが成功しました。次に、configのテストも実行します。",
			expected: "良い！テストが成功しました。次に、configのテストも実行します。",
		},
		{
			name:     "Mixed newlines (LF and CRLF)",
			input:    "\r\n\r\nHello World\r\n\r\n",
			expected: "Hello World",
		},
		{
			name:     "Only newlines",
			input:    "\n\n\n\n",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Newlines in the middle are preserved",
			input:    "\n\nHello\nWorld\n\n",
			expected: "Hello\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimNewlines(tt.input)
			if result != tt.expected {
				t.Errorf("trimNewlines(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatThreadKey(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		threadTS  string
		expected  string
	}{
		{
			name:      "Normal key",
			channelID: "C1234567890",
			threadTS:  "1234567890.123456",
			expected:  "C1234567890:1234567890.123456",
		},
		{
			name:      "Empty channel ID",
			channelID: "",
			threadTS:  "1234567890.123456",
			expected:  ":1234567890.123456",
		},
		{
			name:      "Empty thread TS",
			channelID: "C1234567890",
			threadTS:  "",
			expected:  "C1234567890:",
		},
		{
			name:      "Both empty",
			channelID: "",
			threadTS:  "",
			expected:  ":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatThreadKey(tt.channelID, tt.threadTS)
			if result != tt.expected {
				t.Errorf("formatThreadKey(%q, %q) = %q, want %q", tt.channelID, tt.threadTS, result, tt.expected)
			}
		})
	}
}

func TestGetPriorityTextStyle(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		expected *slack.RichTextSectionTextStyle
	}{
		{
			name:     "High priority",
			priority: "high",
			expected: &slack.RichTextSectionTextStyle{Bold: true},
		},
		{
			name:     "Low priority",
			priority: "low",
			expected: &slack.RichTextSectionTextStyle{Italic: true},
		},
		{
			name:     "Medium priority",
			priority: "medium",
			expected: nil,
		},
		{
			name:     "Unknown priority",
			priority: "unknown",
			expected: nil,
		},
		{
			name:     "Empty priority",
			priority: "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPriorityTextStyle(tt.priority)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("getPriorityTextStyle(%q) = %v, want nil", tt.priority, result)
				}
			} else {
				if result == nil {
					t.Errorf("getPriorityTextStyle(%q) = nil, want %v", tt.priority, tt.expected)
				} else if result.Bold != tt.expected.Bold || result.Italic != tt.expected.Italic {
					t.Errorf("getPriorityTextStyle(%q) = %v, want %v", tt.priority, result, tt.expected)
				}
			}
		})
	}
}

func TestComputeRelativePath(t *testing.T) {
	tests := []struct {
		name         string
		workDir      string
		absolutePath string
		expected     string
	}{
		{
			name:         "Normal relative path",
			workDir:      "/home/user/project",
			absolutePath: "/home/user/project/src/main.go",
			expected:     "src/main.go",
		},
		{
			name:         "Same directory",
			workDir:      "/home/user/project",
			absolutePath: "/home/user/project",
			expected:     ".",
		},
		{
			name:         "Parent directory",
			workDir:      "/home/user/project/src",
			absolutePath: "/home/user/project",
			expected:     "..",
		},
		{
			name:         "Different root",
			workDir:      "/home/user/project",
			absolutePath: "/var/log/app.log",
			expected:     "../../../var/log/app.log",
		},
		{
			name:         "Work dir with trailing slash",
			workDir:      "/home/user/project/",
			absolutePath: "/home/user/project/src/main.go",
			expected:     "src/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeRelativePath(tt.workDir, tt.absolutePath)
			if result != tt.expected {
				t.Errorf("computeRelativePath(%q, %q) = %q, want %q", tt.workDir, tt.absolutePath, result, tt.expected)
			}
		})
	}
}

func TestGetSessionInfoByToolUseID(t *testing.T) {
	// Create a test manager
	manager := &Manager{
		sessions:         make(map[string]*Session),
		threadToSession:  make(map[string]string),
		toolUseToSession: make(map[string]string),
	}

	// Add test session
	testSession := &Session{
		ID:              "test-session-123",
		ChannelID:       "C123456",
		ThreadTS:        "1234567890.123456",
		InitiatorUserID: "U987654",
	}
	manager.sessions["test-session-123"] = testSession

	// Add tool use mappings
	manager.toolUseToSession["tool-use-1"] = "test-session-123"
	manager.toolUseToSession["tool-use-2"] = "test-session-123"
	manager.toolUseToSession["tool-use-other"] = "other-session-456"

	tests := []struct {
		name          string
		toolUseID     string
		wantChannelID string
		wantThreadTS  string
		wantUserID    string
		wantExists    bool
	}{
		{
			name:          "Existing tool use ID",
			toolUseID:     "tool-use-1",
			wantChannelID: "C123456",
			wantThreadTS:  "1234567890.123456",
			wantUserID:    "U987654",
			wantExists:    true,
		},
		{
			name:          "Another existing tool use ID",
			toolUseID:     "tool-use-2",
			wantChannelID: "C123456",
			wantThreadTS:  "1234567890.123456",
			wantUserID:    "U987654",
			wantExists:    true,
		},
		{
			name:          "Tool use ID for non-existent session",
			toolUseID:     "tool-use-other",
			wantChannelID: "",
			wantThreadTS:  "",
			wantUserID:    "",
			wantExists:    false,
		},
		{
			name:          "Non-existent tool use ID",
			toolUseID:     "tool-use-unknown",
			wantChannelID: "",
			wantThreadTS:  "",
			wantUserID:    "",
			wantExists:    false,
		},
		{
			name:          "Empty tool use ID",
			toolUseID:     "",
			wantChannelID: "",
			wantThreadTS:  "",
			wantUserID:    "",
			wantExists:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID, threadTS, userID, exists := manager.GetSessionInfoByToolUseID(tt.toolUseID)

			if channelID != tt.wantChannelID {
				t.Errorf("GetSessionInfoByToolUseID() channelID = %v, want %v", channelID, tt.wantChannelID)
			}
			if threadTS != tt.wantThreadTS {
				t.Errorf("GetSessionInfoByToolUseID() threadTS = %v, want %v", threadTS, tt.wantThreadTS)
			}
			if userID != tt.wantUserID {
				t.Errorf("GetSessionInfoByToolUseID() userID = %v, want %v", userID, tt.wantUserID)
			}
			if exists != tt.wantExists {
				t.Errorf("GetSessionInfoByToolUseID() exists = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}
