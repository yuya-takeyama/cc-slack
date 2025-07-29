package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// SlackEventLog represents a structured log entry for Slack events
type SlackEventLog struct {
	Time       string                 `json:"time"`
	Message    string                 `json:"message"`
	SessionID  string                 `json:"session_id,omitempty"`
	EventID    string                 `json:"event_id,omitempty"`
	EventTS    string                 `json:"event_ts,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	File       string                 `json:"file"`
	Function   string                 `json:"function"`
	LineNumber int                    `json:"line"`
}

// SlackLogger handles structured logging for Slack events
type SlackLogger struct {
	file *os.File
}

// NewSlackLogger creates a new logger instance
func NewSlackLogger(logPath string) (*SlackLogger, error) {
	// Ensure logs directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file for append
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &SlackLogger{file: file}, nil
}

// Close closes the log file
func (l *SlackLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Log writes a structured log entry with event information
func (l *SlackLogger) LogWithEvent(message string, sessionID string, eventID string, eventTS string, data map[string]interface{}) {
	// Get caller information
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	functionName := "unknown"
	if fn != nil {
		functionName = filepath.Base(fn.Name())
	}

	entry := SlackEventLog{
		Time:       time.Now().Format(time.RFC3339Nano),
		Message:    message,
		SessionID:  sessionID,
		EventID:    eventID,
		EventTS:    eventTS,
		Data:       data,
		File:       filepath.Base(file),
		Function:   functionName,
		LineNumber: line,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple logging if JSON marshaling fails
		fmt.Fprintf(l.file, "ERROR: Failed to marshal log entry: %v\n", err)
		return
	}

	// Write as JSONL (one JSON object per line)
	fmt.Fprintf(l.file, "%s\n", jsonData)
}

// Log writes a structured log entry (backward compatible)
func (l *SlackLogger) Log(message string, sessionID string, data map[string]interface{}) {
	l.LogWithEvent(message, sessionID, "", "", data)
}

// LogMessage logs a Slack message event with full context
func (l *SlackLogger) LogMessage(sessionID string, eventID string, eventTS string, messageTS string, threadTS string, userID string, text string, subtype string, hasFiles bool, fileCount int) {
	data := map[string]interface{}{
		"user_id":    userID,
		"text":       text,
		"subtype":    subtype,
		"has_files":  hasFiles,
		"file_count": fileCount,
		"ts":         messageTS,
		"thread_ts":  threadTS,
	}
	l.LogWithEvent("Slack message received", sessionID, eventID, eventTS, data)
}

// LogSession logs session-related events
func (l *SlackLogger) LogSession(sessionID string, action string, details map[string]interface{}) {
	data := map[string]interface{}{
		"action": action,
	}
	for k, v := range details {
		data[k] = v
	}
	l.Log(fmt.Sprintf("Session %s", action), sessionID, data)
}

// LogFileProcessing logs file processing events
func (l *SlackLogger) LogFileProcessing(sessionID string, action string, fileName string, mimeType string, success bool) {
	data := map[string]interface{}{
		"action":    action,
		"file_name": fileName,
		"mime_type": mimeType,
		"success":   success,
	}
	l.Log("File processing", sessionID, data)
}

// LogFilter logs message filtering decisions
func (l *SlackLogger) LogFilter(reason string, details map[string]interface{}) {
	data := map[string]interface{}{
		"reason": reason,
	}
	for k, v := range details {
		data[k] = v
	}
	l.Log("Message filtered", "", data)
}
