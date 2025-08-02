package richtext

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestFormatStyledText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		style    *slack.RichTextSectionTextStyle
		expected string
	}{
		{
			name:     "plain text",
			text:     "hello",
			style:    nil,
			expected: "hello",
		},
		{
			name:     "bold text",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Bold: true},
			expected: "**hello**",
		},
		{
			name:     "italic text",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Italic: true},
			expected: "*hello*",
		},
		{
			name:     "strikethrough text",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Strike: true},
			expected: "~~hello~~",
		},
		{
			name:     "code text",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Code: true},
			expected: "`hello`",
		},
		{
			name:     "bold and italic",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Bold: true, Italic: true},
			expected: "***hello***",
		},
		{
			name:     "code with other styles",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Code: true, Bold: true},
			expected: "**`hello`**",
		},
		{
			name:     "all styles",
			text:     "hello",
			style:    &slack.RichTextSectionTextStyle{Bold: true, Italic: true, Strike: true, Code: true},
			expected: "~~***`hello`***~~",
		},
		{
			name:     "empty text",
			text:     "",
			style:    &slack.RichTextSectionTextStyle{Bold: true},
			expected: "****",
		},
		{
			name:     "text with special markdown characters",
			text:     "hello *world* **test**",
			style:    &slack.RichTextSectionTextStyle{Bold: true},
			expected: "**hello *world* **test****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStyledText(tt.text, tt.style)
			if got != tt.expected {
				t.Errorf("FormatStyledText(%q, %+v) = %q, want %q", tt.text, tt.style, got, tt.expected)
			}
		})
	}
}

func TestConvertToString(t *testing.T) {
	tests := []struct {
		name     string
		richText *slack.RichTextBlock
		expected string
	}{
		{
			name: "simple text",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "Hello, world!",
							},
						},
					},
				},
			},
			expected: "Hello, world!",
		},
		{
			name: "styled text",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type:  "text",
								Text:  "Bold",
								Style: &slack.RichTextSectionTextStyle{Bold: true},
							},
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: " and ",
							},
							&slack.RichTextSectionTextElement{
								Type:  "text",
								Text:  "italic",
								Style: &slack.RichTextSectionTextStyle{Italic: true},
							},
						},
					},
				},
			},
			expected: "**Bold** and *italic*",
		},
		{
			name: "link",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionLinkElement{
								Type: "link",
								URL:  "https://example.com",
								Text: "Example",
							},
						},
					},
				},
			},
			expected: "[Example](https://example.com)",
		},
		{
			name: "link without text",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionLinkElement{
								Type: "link",
								URL:  "https://example.com",
								Text: "",
							},
						},
					},
				},
			},
			expected: "https://example.com",
		},
		{
			name: "channel mention",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionChannelElement{
								Type:      "channel",
								ChannelID: "C123456",
							},
						},
					},
				},
			},
			expected: "<#C123456>",
		},
		{
			name: "user mention",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionUserElement{
								Type:   "user",
								UserID: "U123456",
							},
						},
					},
				},
			},
			expected: "<@U123456>",
		},
		{
			name: "quote block",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextQuote{
						Type: "rich_text_quote",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "This is a quote",
							},
						},
					},
				},
			},
			expected: "> This is a quote",
		},
		{
			name: "quote with styled text",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextQuote{
						Type: "rich_text_quote",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type:  "text",
								Text:  "Bold quote",
								Style: &slack.RichTextSectionTextStyle{Bold: true},
							},
						},
					},
				},
			},
			expected: "> **Bold quote**",
		},
		{
			name: "quote with link",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextQuote{
						Type: "rich_text_quote",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionLinkElement{
								Type: "link",
								URL:  "https://example.com",
								Text: "Link in quote",
							},
						},
					},
				},
			},
			expected: "> [Link in quote](https://example.com)",
		},
		// TODO: Add RichTextPreformatted tests after confirming the actual structure
		// The current slack-go/slack version might have a different structure
		{
			name: "empty rich text",
			richText: &slack.RichTextBlock{
				Type:     "rich_text",
				Elements: []slack.RichTextElement{},
			},
			expected: "",
		},
		{
			name: "mixed content",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "Hello ",
							},
							&slack.RichTextSectionTextElement{
								Type:  "text",
								Text:  "world",
								Style: &slack.RichTextSectionTextStyle{Bold: true},
							},
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "! Check out ",
							},
							&slack.RichTextSectionLinkElement{
								Type: "link",
								URL:  "https://example.com",
								Text: "this link",
							},
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: ".",
							},
						},
					},
				},
			},
			expected: "Hello **world**! Check out [this link](https://example.com).",
		},
		{
			name: "multiple sections",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "First line\nSecond line",
							},
						},
					},
					&slack.RichTextQuote{
						Type: "rich_text_quote",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "A quote",
							},
						},
					},
					&slack.RichTextSection{
						Type: "rich_text_section",
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{
								Type: "text",
								Text: "After quote",
							},
						},
					},
				},
			},
			expected: "First line\nSecond line> A quote\nAfter quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToString(tt.richText)
			if got != tt.expected {
				t.Errorf("ConvertToString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestConvertToString_Lists(t *testing.T) {
	tests := []struct {
		name     string
		richText *slack.RichTextBlock
		expected string
	}{
		{
			name: "simple bullet list",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextList{
						Type:  "rich_text_list",
						Style: "bullet",
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: "text",
										Text: "Item 1",
									},
								},
							},
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: "text",
										Text: "Item 2",
									},
								},
							},
						},
					},
				},
			},
			expected: "- Item 1\n- Item 2",
		},
		{
			name: "ordered list",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextList{
						Type:  "rich_text_list",
						Style: "ordered",
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: "text",
										Text: "First",
									},
								},
							},
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: "text",
										Text: "Second",
									},
								},
							},
						},
					},
				},
			},
			expected: "1. First\n2. Second",
		},
		{
			name: "list with styled text",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextList{
						Type:  "rich_text_list",
						Style: "bullet",
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type:  "text",
										Text:  "Bold item",
										Style: &slack.RichTextSectionTextStyle{Bold: true},
									},
								},
							},
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type:  "text",
										Text:  "Italic item",
										Style: &slack.RichTextSectionTextStyle{Italic: true},
									},
								},
							},
						},
					},
				},
			},
			expected: "- **Bold item**\n- *Italic item*",
		},
		{
			name: "list with links",
			richText: &slack.RichTextBlock{
				Type: "rich_text",
				Elements: []slack.RichTextElement{
					&slack.RichTextList{
						Type:  "rich_text_list",
						Style: "bullet",
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: "rich_text_section",
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: "text",
										Text: "Check ",
									},
									&slack.RichTextSectionLinkElement{
										Type: "link",
										URL:  "https://example.com",
										Text: "this link",
									},
								},
							},
						},
					},
				},
			},
			expected: "- Check [this link](https://example.com)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToString(tt.richText)
			if got != tt.expected {
				t.Errorf("ConvertToString() = %q, want %q", got, tt.expected)
			}
		})
	}
}
