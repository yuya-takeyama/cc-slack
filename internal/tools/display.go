package tools

// Tool names
const (
	ToolTodoWrite    = "TodoWrite"
	ToolBash         = "Bash"
	ToolRead         = "Read"
	ToolGlob         = "Glob"
	ToolEdit         = "Edit"
	ToolMultiEdit    = "MultiEdit"
	ToolWrite        = "Write"
	ToolLS           = "LS"
	ToolGrep         = "Grep"
	ToolWebFetch     = "WebFetch"
	ToolWebSearch    = "WebSearch"
	ToolTask         = "Task"
	ToolExitPlanMode = "ExitPlanMode"
	ToolNotebookRead = "NotebookRead"
	ToolNotebookEdit = "NotebookEdit"

	// Special message types
	MessageThinking = "thinking"
)

// ToolInfo holds display information for tools
type ToolInfo struct {
	Name      string
	Emoji     string // Unicode emoji for internal use
	SlackIcon string // Slack emoji code for Slack messages
}

// toolInfoMap maps tool names to their display information
var toolInfoMap = map[string]ToolInfo{
	ToolTodoWrite:    {Name: "TodoWrite", Emoji: "📋", SlackIcon: ":memo:"},
	ToolBash:         {Name: "Bash", Emoji: "💻", SlackIcon: ":computer:"},
	ToolRead:         {Name: "Read", Emoji: "📖", SlackIcon: ":open_book:"},
	ToolGlob:         {Name: "Glob", Emoji: "🔍", SlackIcon: ":mag:"},
	ToolEdit:         {Name: "Edit", Emoji: "✏️", SlackIcon: ":pencil2:"},
	ToolMultiEdit:    {Name: "MultiEdit", Emoji: "✏️", SlackIcon: ":pencil2:"},
	ToolWrite:        {Name: "Write", Emoji: "📝", SlackIcon: ":memo:"},
	ToolLS:           {Name: "LS", Emoji: "📁", SlackIcon: ":file_folder:"},
	ToolGrep:         {Name: "Grep", Emoji: "🔍", SlackIcon: ":mag:"},
	ToolWebFetch:     {Name: "WebFetch", Emoji: "🌐", SlackIcon: ":globe_with_meridians:"},
	ToolWebSearch:    {Name: "WebSearch", Emoji: "🌎", SlackIcon: ":earth_americas:"},
	ToolTask:         {Name: "Task", Emoji: "🤖", SlackIcon: ":robot_face:"},
	ToolExitPlanMode: {Name: "ExitPlanMode", Emoji: "🏁", SlackIcon: ":checkered_flag:"},
	ToolNotebookRead: {Name: "NotebookRead", Emoji: "📓", SlackIcon: ":notebook:"},
	ToolNotebookEdit: {Name: "NotebookEdit", Emoji: "📔", SlackIcon: ":notebook_with_decorative_cover:"},

	// Special message types
	MessageThinking: {Name: "Thinking", Emoji: "🤔", SlackIcon: ":thinking_face:"},
}

// GetToolInfo returns tool information for the given tool name
func GetToolInfo(toolName string) ToolInfo {
	if info, ok := toolInfoMap[toolName]; ok {
		return info
	}

	// Default for unknown tools
	return ToolInfo{
		Name:      toolName,
		Emoji:     "🔧",
		SlackIcon: ":wrench:",
	}
}

// GetToolEmoji returns the Unicode emoji for the given tool name
func GetToolEmoji(toolName string) string {
	info := GetToolInfo(toolName)
	return info.Emoji
}

// GetToolSlackIcon returns the Slack emoji code for the given tool name
func GetToolSlackIcon(toolName string) string {
	info := GetToolInfo(toolName)
	return info.SlackIcon
}

// IsMCPTool checks if the tool name is an MCP tool
func IsMCPTool(toolName string) bool {
	return len(toolName) > 5 && toolName[:5] == "mcp__"
}

// GetMCPToolInfo returns tool information for MCP tools
func GetMCPToolInfo() ToolInfo {
	return ToolInfo{
		Name:      "MCP Tool",
		Emoji:     "🔌",
		SlackIcon: ":electric_plug:",
	}
}
