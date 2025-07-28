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

func TestHandleAppMention_DuplicateDetection(t *testing.T) {
	mockSession := &MockSessionManager{}
	h := NewHandler("test-token", "test-secret", mockSession)

	// First app mention event
	event1 := &slackevents.AppMentionEvent{
		Type:            "app_mention",
		Channel:         "C123456",
		TimeStamp:       "1234567890.123456",
		ThreadTimeStamp: "",
		Text:            "<@U123456> hello",
	}

	// Handle first mention
	h.handleAppMention(event1)

	// Verify session was created
	if len(mockSession.createSessionWithResume) != 1 {
		t.Errorf("Expected 1 createSessionWithResume call, got %d", len(mockSession.createSessionWithResume))
	}

	// Handle duplicate mention with same timestamp
	h.handleAppMention(event1)

	// Verify session was NOT created again
	if len(mockSession.createSessionWithResume) != 1 {
		t.Errorf("Expected still 1 createSessionWithResume call after duplicate, got %d", len(mockSession.createSessionWithResume))
	}

	// Different timestamp should create new session
	event2 := &slackevents.AppMentionEvent{
		Type:            "app_mention",
		Channel:         "C123456",
		TimeStamp:       "1234567890.654321",
		ThreadTimeStamp: "",
		Text:            "<@U123456> world",
	}

	h.handleAppMention(event2)

	// Verify new session was created
	if len(mockSession.createSessionWithResume) != 2 {
		t.Errorf("Expected 2 createSessionWithResume calls for different timestamp, got %d", len(mockSession.createSessionWithResume))
	}
}

func TestHandleThreadMessage_DuplicateDetection(t *testing.T) {
	mockSession := &MockSessionManager{
		getSessionByThreadReturn: &Session{
			SessionID: "test-session",
			ChannelID: "C123456",
			ThreadTS:  "1234567890.000000",
		},
	}
	h := NewHandler("test-token", "test-secret", mockSession)

	// First message event
	event1 := &slackevents.MessageEvent{
		Type:            "message",
		Channel:         "C123456",
		TimeStamp:       "1234567890.123456",
		ThreadTimeStamp: "1234567890.000000",
		Text:            "hello",
		BotID:           "", // Not a bot message
	}

	// Handle first message
	h.handleThreadMessage(event1)

	// Verify message was sent
	if len(mockSession.sendMessageCalls) != 1 {
		t.Errorf("Expected 1 sendMessage call, got %d", len(mockSession.sendMessageCalls))
	}

	// Handle duplicate message with same timestamp
	h.handleThreadMessage(event1)

	// Verify message was NOT sent again
	if len(mockSession.sendMessageCalls) != 1 {
		t.Errorf("Expected still 1 sendMessage call after duplicate, got %d", len(mockSession.sendMessageCalls))
	}

	// Different timestamp should send new message
	event2 := &slackevents.MessageEvent{
		Type:            "message",
		Channel:         "C123456",
		TimeStamp:       "1234567890.654321",
		ThreadTimeStamp: "1234567890.000000",
		Text:            "world",
		BotID:           "", // Not a bot message
	}

	h.handleThreadMessage(event2)

	// Verify new message was sent
	if len(mockSession.sendMessageCalls) != 2 {
		t.Errorf("Expected 2 sendMessage calls for different timestamp, got %d", len(mockSession.sendMessageCalls))
	}
}
