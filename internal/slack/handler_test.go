package slack

import (
	"context"
	"fmt"
	"testing"

	"github.com/slack-go/slack/slackevents"
	"github.com/yuya-takeyama/cc-slack/internal/config"
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

func TestHandleAppMention_InThread(t *testing.T) {
	mockSession := &MockSessionManager{
		getSessionByThreadError: fmt.Errorf("not found"), // Simulate no existing session
	}
	h, err := NewHandler(createTestConfig(), mockSession)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	// Verify GetSessionByThread was called first
	if len(mockSession.getSessionByThreadCalls) != 1 {
		t.Errorf("Expected 1 getSessionByThreadCalls, got %d", len(mockSession.getSessionByThreadCalls))
	}

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
	h, err := NewHandler(createTestConfig(), mockSession)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

func TestHandleAppMention_ExistingSession(t *testing.T) {
	mockSession := &MockSessionManager{
		getSessionByThreadReturn: &Session{
			SessionID: "existing-session",
			ChannelID: "C123456",
			ThreadTS:  "1234567890.000000",
			WorkDir:   "/test/dir",
		},
	}
	h, err := NewHandler(createTestConfig(), mockSession)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// App mention in a thread with existing session
	event := &slackevents.AppMentionEvent{
		Type:            "app_mention",
		Channel:         "C123456",
		TimeStamp:       "1234567890.999999",
		ThreadTimeStamp: "1234567890.000000", // In a thread
		Text:            "<@U123456> interrupt message",
	}

	// Handle mention
	h.handleAppMention(event)

	// Verify GetSessionByThread was called
	if len(mockSession.getSessionByThreadCalls) != 1 {
		t.Errorf("Expected 1 getSessionByThreadCalls, got %d", len(mockSession.getSessionByThreadCalls))
	}
	call := mockSession.getSessionByThreadCalls[0]
	if call.channelID != "C123456" || call.threadTS != "1234567890.000000" {
		t.Errorf("Expected GetSessionByThread to be called with correct params")
	}

	// Verify SendMessage was called instead of CreateSessionWithResume
	if len(mockSession.sendMessageCalls) != 1 {
		t.Errorf("Expected 1 sendMessageCalls, got %d", len(mockSession.sendMessageCalls))
	}
	sendCall := mockSession.sendMessageCalls[0]
	if sendCall.sessionID != "existing-session" {
		t.Errorf("Expected sessionID to be %s, got %s", "existing-session", sendCall.sessionID)
	}
	if sendCall.message != "interrupt message" {
		t.Errorf("Expected message to be %s, got %s", "interrupt message", sendCall.message)
	}

	// Verify CreateSessionWithResume was NOT called
	if len(mockSession.createSessionWithResume) != 0 {
		t.Errorf("Expected 0 createSessionWithResume calls, got %d", len(mockSession.createSessionWithResume))
	}
}
