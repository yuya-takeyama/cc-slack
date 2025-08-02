package richtext

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/slack-go/slack"
)

// ConvertToString converts Slack rich text to plain string
func ConvertToString(richText *slack.RichTextBlock) string {
	var result strings.Builder
	processElements(richText.Elements, &result, 0)
	return strings.TrimSpace(result.String())
}

// FormatStyledText applies markdown formatting based on text style
func FormatStyledText(text string, style *slack.RichTextSectionTextStyle) string {
	if style == nil {
		return text
	}

	// Apply styles in a specific order to handle multiple styles
	result := text

	// Code should be applied first to avoid conflicts with other styles
	if style.Code {
		result = fmt.Sprintf("`%s`", result)
	}

	// Apply other styles
	if style.Bold {
		result = fmt.Sprintf("**%s**", result)
	}
	if style.Italic {
		result = fmt.Sprintf("*%s*", result)
	}
	if style.Strike {
		result = fmt.Sprintf("~~%s~~", result)
	}

	return result
}

// processElements recursively processes rich text elements with proper indentation
func processElements(elements []slack.RichTextElement, result *strings.Builder, baseIndent int) {
	for _, element := range elements {
		switch elem := element.(type) {
		case *slack.RichTextSection:
			// Add indentation for list items
			if baseIndent > 0 {
				result.WriteString(strings.Repeat("  ", baseIndent))
			}
			for _, e := range elem.Elements {
				switch textElem := e.(type) {
				case *slack.RichTextSectionTextElement:
					result.WriteString(FormatStyledText(textElem.Text, textElem.Style))
				case *slack.RichTextSectionChannelElement:
					result.WriteString(fmt.Sprintf("<#%s>", textElem.ChannelID))
				case *slack.RichTextSectionUserElement:
					result.WriteString(fmt.Sprintf("<@%s>", textElem.UserID))
				case *slack.RichTextSectionLinkElement:
					if textElem.Text != "" {
						result.WriteString(fmt.Sprintf("[%s](%s)", textElem.Text, textElem.URL))
					} else {
						result.WriteString(textElem.URL)
					}
				}
			}
			// Only add newline if we're not at the top level
			if baseIndent > 0 {
				result.WriteString("\n")
			}
		case *slack.RichTextList:
			// Try to get the indent level from the struct
			// Note: The Indent field might not be exported in the current SDK version
			indentLevel := baseIndent

			// Use reflection to try to access the Indent field
			elemValue := reflect.ValueOf(elem).Elem()
			indentField := elemValue.FieldByName("Indent")
			if indentField.IsValid() && indentField.Kind() == reflect.Int {
				indentLevel = int(indentField.Int())
			}

			for i, item := range elem.Elements {
				// Add indentation
				result.WriteString(strings.Repeat("  ", indentLevel))

				// Add list marker
				if elem.Style == slack.RTEListOrdered {
					result.WriteString(fmt.Sprintf("%d. ", i+1))
				} else {
					result.WriteString("- ")
				}

				// Process the list item content
				switch listItem := item.(type) {
				case *slack.RichTextSection:
					// Process text elements within the list item
					for _, e := range listItem.Elements {
						switch elem := e.(type) {
						case *slack.RichTextSectionTextElement:
							result.WriteString(FormatStyledText(elem.Text, elem.Style))
						case *slack.RichTextSectionLinkElement:
							if elem.Text != "" {
								result.WriteString(fmt.Sprintf("[%s](%s)", elem.Text, elem.URL))
							} else {
								result.WriteString(elem.URL)
							}
						}
					}
					result.WriteString("\n")
				}
			}
		case *slack.RichTextQuote:
			// Handle quote blocks
			result.WriteString("> ")
			// RichTextQuote contains RichTextSectionElements
			for _, quotedElem := range elem.Elements {
				switch textElem := quotedElem.(type) {
				case *slack.RichTextSectionTextElement:
					result.WriteString(FormatStyledText(textElem.Text, textElem.Style))
				case *slack.RichTextSectionLinkElement:
					if textElem.Text != "" {
						result.WriteString(fmt.Sprintf("[%s](%s)", textElem.Text, textElem.URL))
					} else {
						result.WriteString(textElem.URL)
					}
				}
			}
			result.WriteString("\n")
		case *slack.RichTextPreformatted:
			// Handle preformatted/code blocks
			result.WriteString("```\n")
			for _, preElem := range elem.Elements {
				if textElem, ok := preElem.(*slack.RichTextSectionTextElement); ok {
					result.WriteString(textElem.Text)
				}
			}
			// Ensure there's a newline at the end
			if !strings.HasSuffix(result.String(), "\n") {
				result.WriteString("\n")
			}
			result.WriteString("```\n")
		}
	}
}
