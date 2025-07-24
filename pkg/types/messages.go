package types

import (
	"encoding/json"
)

// Base message structure
type BaseMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
}

// System messages
type SystemMessage struct {
	BaseMessage
	Subtype         string   `json:"subtype"`
	CWD             string   `json:"cwd"`
	Tools           []string `json:"tools"`
	MCPServers      []string `json:"mcp_servers"`
	Model           string   `json:"model"`
	PermissionMode  string   `json:"permissionMode"`
	APIKeySource    string   `json:"apiKeySource"`
}

// Assistant messages with content
type AssistantMessage struct {
	BaseMessage
	Message          ClaudeMessage `json:"message"`
	ParentToolUseID  *string       `json:"parent_tool_use_id"`
}

type ClaudeMessage struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Role        string         `json:"role"`
	Model       string         `json:"model"`
	Content     []ContentBlock `json:"content"`
	StopReason  *string        `json:"stop_reason"`
	StopSequence *string       `json:"stop_sequence"`
	Usage       Usage          `json:"usage"`
}

type ContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	Thinking  string      `json:"thinking,omitempty"`
	Signature string      `json:"signature,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
}

// User messages (tool results)
type UserMessage struct {
	BaseMessage
	Message         ClaudeUserMessage `json:"message"`
	ParentToolUseID *string           `json:"parent_tool_use_id"`
}

type ClaudeUserMessage struct {
	Role    string               `json:"role"`
	Content []ToolResultContent  `json:"content"`
}

type ToolResultContent struct {
	ToolUseID string `json:"tool_use_id"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}

// Result messages
type ResultMessage struct {
	BaseMessage
	Subtype       string `json:"subtype"`
	IsError       bool   `json:"is_error"`
	DurationMS    int    `json:"duration_ms"`
	DurationAPIMS int    `json:"duration_api_ms"`
	NumTurns      int    `json:"num_turns"`
	Result        string `json:"result"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	Usage         Usage   `json:"usage"`
}

type Usage struct {
	InputTokens               int           `json:"input_tokens"`
	CacheCreationInputTokens  int           `json:"cache_creation_input_tokens"`
	CacheReadInputTokens      int           `json:"cache_read_input_tokens"`
	OutputTokens              int           `json:"output_tokens"`
	ServerToolUse             ServerToolUse `json:"server_tool_use"`
	ServiceTier               string        `json:"service_tier"`
}

type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
}

// Input message to Claude Code
type InputMessage struct {
	Type    string      `json:"type"`
	Message HumanMessage `json:"message"`
}

type HumanMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// Parse JSON Lines message
func ParseMessage(data []byte) (interface{}, error) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}
	
	switch base.Type {
	case "system":
		var msg SystemMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "user":
		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "result":
		var msg ResultMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return base, nil
	}
}