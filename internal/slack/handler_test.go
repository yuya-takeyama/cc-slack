package slack

import (
	"context"
	"testing"

	"github.com/slack-go/slack/slackevents"
)

func TestRemoveBotMention(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple mention",
			input:    "<@U123456> hello",
			expected: "hello",
		},
		{
			name:     "mention with no space",
			input:    "<@U123456>hello",
			expected: "hello",
		},
		{
			name:     "no mention",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "mention only",
			input:    "<@U123456>",
			expected: "",
		},
		{
			name:     "mention with extra spaces",
			input:    "<@U123456>   hello world",
			expected: "hello world",
		},
		{
			name:     "multiple words after mention",
			input:    "<@U123456> hello world test",
			expected: "hello world test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "",
		},
		{
			name:     "mention with newline",
			input:    "<@U123456>\nhello",
			expected: "hello",
		},
		{
			name:     "incomplete mention",
			input:    "<@U123456 hello",
			expected: "<@U123456 hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.removeBotMention(tt.input)
			if got != tt.expected {
				t.Errorf("removeBotMention(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// MockSessionManager implements SessionManager for testing
type MockSessionManager struct {
	createSessionCalls      []createSessionCall
	createSessionWithResume []createSessionWithResumeCall
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

func (m *MockSessionManager) GetSessionByThread(channelID, threadTS string) (*Session, error) {
	// Not used in current tests
	return nil, nil
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
	// Not used in current tests
	return nil
}

func TestHandleAppMention_InThread(t *testing.T) {
	mockSession := &MockSessionManager{}
	h := NewHandler("test-token", "test-secret", mockSession)

	// App mention in a thread
	event := &slackevents.AppMentionEvent{
		Type:            "app_mention",
		Channel:         "C123456",
		TimeStamp:       "1234567890.123456",
		ThreadTimeStamp: "1234567890.000000", // In a thread
		Text:            "<@U123456> hello from thread",
	}

	// Handle mention
	h.handleAppMention(event)

	// Verify session was created with thread_ts
	if len(mockSession.createSessionWithResume) != 1 {
		t.Errorf("Expected 1 createSessionWithResume call, got %d", len(mockSession.createSessionWithResume))
	}

	call := mockSession.createSessionWithResume[0]
	if call.threadTS != "1234567890.000000" {
		t.Errorf("Expected threadTS to be %s, got %s", "1234567890.000000", call.threadTS)
	}
	if call.initialPrompt != "hello from thread" {
		t.Errorf("Expected initialPrompt to be %s, got %s", "hello from thread", call.initialPrompt)
	}
}

func TestHandleAppMention_OutsideThread(t *testing.T) {
	mockSession := &MockSessionManager{}
	h := NewHandler("test-token", "test-secret", mockSession)

	// App mention outside a thread
	event := &slackevents.AppMentionEvent{
		Type:            "app_mention",
		Channel:         "C123456",
		TimeStamp:       "1234567890.123456",
		ThreadTimeStamp: "", // Not in a thread
		Text:            "<@U123456> hello from channel",
	}

	// Handle mention
	h.handleAppMention(event)

	// Verify session was created with message ts as thread_ts
	if len(mockSession.createSessionWithResume) != 1 {
		t.Errorf("Expected 1 createSessionWithResume call, got %d", len(mockSession.createSessionWithResume))
	}

	call := mockSession.createSessionWithResume[0]
	if call.threadTS != "1234567890.123456" {
		t.Errorf("Expected threadTS to be %s, got %s", "1234567890.123456", call.threadTS)
	}
	if call.initialPrompt != "hello from channel" {
		t.Errorf("Expected initialPrompt to be %s, got %s", "hello from channel", call.initialPrompt)
	}
}
