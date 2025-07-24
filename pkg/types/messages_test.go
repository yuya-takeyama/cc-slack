package types

import (
	"encoding/json"
	"testing"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "parse system message",
			input: `{
				"type": "system",
				"subtype": "init",
				"session_id": "test-123",
				"cwd": "/test/dir",
				"tools": ["Task", "Bash"],
				"model": "claude-opus-4"
			}`,
			wantErr: false,
		},
		{
			name: "parse assistant message",
			input: `{
				"type": "assistant",
				"session_id": "test-123",
				"message": {
					"id": "msg_123",
					"type": "message",
					"role": "assistant",
					"model": "claude-opus-4",
					"content": [
						{
							"type": "text",
							"text": "Hello!"
						}
					]
				}
			}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputMessage_Marshal(t *testing.T) {
	msg := InputMessage{
		Type: "message",
		Message: HumanMessage{
			Type:    "human",
			Content: "Hello, Claude!",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal InputMessage: %v", err)
	}

	// Check that it marshals to expected format
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result["type"] != "message" {
		t.Errorf("Expected type 'message', got %v", result["type"])
	}

	if message, ok := result["message"].(map[string]interface{}); ok {
		if message["type"] != "human" {
			t.Errorf("Expected message type 'human', got %v", message["type"])
		}
		if message["content"] != "Hello, Claude!" {
			t.Errorf("Expected content 'Hello, Claude!', got %v", message["content"])
		}
	} else {
		t.Error("Message field is not a map")
	}
}