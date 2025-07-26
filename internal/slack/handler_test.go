package slack

import (
	"testing"
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
