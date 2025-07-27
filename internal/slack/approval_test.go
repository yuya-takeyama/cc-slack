package slack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildApprovalPayload(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		threadTS  string
		message   string
		requestID string
		action    string
		validate  func(t *testing.T, payload map[string]interface{})
	}{
		{
			name:      "approve action",
			channelID: "C123456",
			threadTS:  "1234567890.123456",
			message:   "Please approve this action",
			requestID: "req_123",
			action:    "approve",
			validate: func(t *testing.T, payload map[string]interface{}) {
				if payload["type"] != "block_actions" {
					t.Errorf("Expected type to be 'block_actions', got %v", payload["type"])
				}

				actions, ok := payload["actions"].([]map[string]interface{})
				if !ok || len(actions) != 1 {
					t.Fatal("Expected actions to be a slice with one element")
				}

				if actions[0]["action_id"] != "approve_req_123" {
					t.Errorf("Expected action_id to be 'approve_req_123', got %v", actions[0]["action_id"])
				}

				if actions[0]["value"] != "approve" {
					t.Errorf("Expected value to be 'approve', got %v", actions[0]["value"])
				}

				channel, ok := payload["channel"].(map[string]string)
				if !ok || channel["id"] != "C123456" {
					t.Errorf("Expected channel.id to be 'C123456', got %v", channel["id"])
				}

				message, ok := payload["message"].(map[string]string)
				if !ok || message["ts"] != "1234567890.123456" {
					t.Errorf("Expected message.ts to be '1234567890.123456', got %v", message["ts"])
				}
			},
		},
		{
			name:      "deny action",
			channelID: "C789012",
			threadTS:  "9876543210.987654",
			message:   "Please deny this action",
			requestID: "req_456",
			action:    "deny",
			validate: func(t *testing.T, payload map[string]interface{}) {
				actions, _ := payload["actions"].([]map[string]interface{})
				if actions[0]["action_id"] != "deny_req_456" {
					t.Errorf("Expected action_id to be 'deny_req_456', got %v", actions[0]["action_id"])
				}

				if actions[0]["value"] != "deny" {
					t.Errorf("Expected value to be 'deny', got %v", actions[0]["value"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := BuildApprovalPayload(tt.channelID, tt.threadTS, tt.message, tt.requestID, tt.action)
			tt.validate(t, payload)
		})
	}
}

func TestGenerateDebugCurlCommands(t *testing.T) {
	approvePayload := map[string]interface{}{
		"type": "block_actions",
		"actions": []map[string]interface{}{
			{
				"action_id": "approve_req_123",
				"type":      "button",
				"value":     "approve",
			},
		},
		"channel": map[string]string{
			"id": "C123456",
		},
		"message": map[string]string{
			"ts":   "1234567890.123456",
			"text": "Test message",
		},
	}

	denyPayload := map[string]interface{}{
		"type": "block_actions",
		"actions": []map[string]interface{}{
			{
				"action_id": "deny_req_123",
				"type":      "button",
				"value":     "deny",
			},
		},
		"channel": map[string]string{
			"id": "C123456",
		},
		"message": map[string]string{
			"ts":   "1234567890.123456",
			"text": "Test message",
		},
	}

	result := GenerateDebugCurlCommands(approvePayload, denyPayload)

	// Check that the result contains expected elements
	if !strings.Contains(result, "【デバッグ用curlコマンド】") {
		t.Error("Expected result to contain debug command header")
	}

	if !strings.Contains(result, "承認する場合:") {
		t.Error("Expected result to contain approve section")
	}

	if !strings.Contains(result, "拒否する場合:") {
		t.Error("Expected result to contain deny section")
	}

	if !strings.Contains(result, "curl -X POST http://localhost:8080/slack/interactive") {
		t.Error("Expected result to contain curl command")
	}

	// Check that payloads are properly JSON encoded
	approveJSON, _ := json.Marshal(approvePayload)
	if !strings.Contains(result, string(approveJSON)) {
		t.Error("Expected result to contain approve payload JSON")
	}

	denyJSON, _ := json.Marshal(denyPayload)
	if !strings.Contains(result, string(denyJSON)) {
		t.Error("Expected result to contain deny payload JSON")
	}
}
