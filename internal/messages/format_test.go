package messages

import (
	"strings"
	"testing"
	"time"
)

func TestFormatSessionStartMessage(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		cwd       string
		model     string
		want      string
	}{
		{
			name:      "standard session",
			sessionID: "session-123",
			cwd:       "/home/user/project",
			model:     "claude-3.5-sonnet",
			want: "✨ Claude Code session started\n" +
				"Session ID: `session-123`\n" +
				"Working directory: `/home/user/project`\n" +
				"Model: `claude-3.5-sonnet`",
		},
		{
			name:      "with spaces in path",
			sessionID: "session-456",
			cwd:       "/Users/name/My Documents/project",
			model:     "claude-3.5-sonnet",
			want: "✨ Claude Code session started\n" +
				"Session ID: `session-456`\n" +
				"Working directory: `/Users/name/My Documents/project`\n" +
				"Model: `claude-3.5-sonnet`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSessionStartMessage(tt.sessionID, tt.cwd, tt.model)
			if got != tt.want {
				t.Errorf("FormatSessionStartMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatSessionCompleteMessage(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		duration     time.Duration
		turns        int
		cost         float64
		inputTokens  int
		outputTokens int
		want         string
	}{
		{
			name:         "short session",
			sessionID:    "session-123",
			duration:     5 * time.Second,
			turns:        3,
			cost:         0.05,
			inputTokens:  1000,
			outputTokens: 500,
			want: "✅ Session completed\n" +
				"Session ID: `session-123`\n" +
				"Duration: 5s\n" +
				"Turns: 3\n" +
				"Cost: $0.050000 USD\n" +
				"Tokens used: input=1000, output=500",
		},
		{
			name:         "long session with high cost",
			sessionID:    "d423f0ad-9ba7-46e4-8afb-869f70a89fff",
			duration:     125 * time.Second,
			turns:        20,
			cost:         1.5,
			inputTokens:  50000,
			outputTokens: 25000,
			want: "✅ Session completed\n" +
				"Session ID: `d423f0ad-9ba7-46e4-8afb-869f70a89fff`\n" +
				"Duration: 2m5s\n" +
				"Turns: 20\n" +
				"Cost: $1.500000 USD\n" +
				"Tokens used: input=50000, output=25000\n" +
				"⚠️ High cost session",
		},
		{
			name:         "very long session",
			sessionID:    "5238c540-08c9-42e3-b7fa-7b74385e9c88",
			duration:     3665 * time.Second,
			turns:        50,
			cost:         0.8,
			inputTokens:  100000,
			outputTokens: 50000,
			want: "✅ Session completed\n" +
				"Session ID: `5238c540-08c9-42e3-b7fa-7b74385e9c88`\n" +
				"Duration: 1h1m5s\n" +
				"Turns: 50\n" +
				"Cost: $0.800000 USD\n" +
				"Tokens used: input=100000, output=50000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSessionCompleteMessage(tt.sessionID, tt.duration, tt.turns, tt.cost, tt.inputTokens, tt.outputTokens)
			if got != tt.want {
				t.Errorf("FormatSessionCompleteMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTimeoutMessage(t *testing.T) {
	tests := []struct {
		name        string
		idleMinutes int
		sessionID   string
		want        string
	}{
		{
			name:        "short idle",
			idleMinutes: 15,
			sessionID:   "session-123",
			want: "⏰ Session timed out\n" +
				"Idle time: 15 minutes\n" +
				"Session ID: `session-123`\n\n" +
				"To start a new session, please mention me again.",
		},
		{
			name:        "long idle",
			idleMinutes: 120,
			sessionID:   "session-456",
			want: "⏰ Session timed out\n" +
				"Idle time: 120 minutes\n" +
				"Session ID: `session-456`\n\n" +
				"To start a new session, please mention me again.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimeoutMessage(tt.idleMinutes, tt.sessionID)
			if got != tt.want {
				t.Errorf("FormatTimeoutMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatBashToolMessage(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "simple command",
			command: "ls -la",
			want:    "```\nls -la\n```",
		},
		{
			name:    "command with triple backticks",
			command: "echo '```code```'",
			want:    "```\necho '\\`\\`\\`code\\`\\`\\`'\n```",
		},
		{
			name:    "multiline command",
			command: "git commit -m \"fix: something\n\nDetailed description\"",
			want:    "```\ngit commit -m \"fix: something\n\nDetailed description\"\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBashToolMessage(tt.command)
			if got != tt.want {
				t.Errorf("FormatBashToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatReadToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		offset   int
		limit    int
		want     string
	}{
		{
			name:     "simple path - entire file",
			filePath: "main.go",
			offset:   0,
			limit:    0,
			want:     "Reading `main.go`",
		},
		{
			name:     "path with spaces - entire file",
			filePath: "my file.txt",
			offset:   0,
			limit:    0,
			want:     "Reading `my file.txt`",
		},
		{
			name:     "absolute path - entire file",
			filePath: "/Users/name/project/src/main.go",
			offset:   0,
			limit:    0,
			want:     "Reading `/Users/name/project/src/main.go`",
		},
		{
			name:     "with offset and limit",
			filePath: "main.go",
			offset:   100,
			limit:    50,
			want:     "Reading `main.go` (lines 100-149)",
		},
		{
			name:     "with offset only",
			filePath: "config.json",
			offset:   200,
			limit:    0,
			want:     "Reading `config.json` (from line 200)",
		},
		{
			name:     "with limit only",
			filePath: "README.md",
			offset:   0,
			limit:    100,
			want:     "Reading `README.md` (first 100 lines)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatReadToolMessage(tt.filePath, tt.offset, tt.limit)
			if got != tt.want {
				t.Errorf("FormatReadToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatGrepToolMessage(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    string
	}{
		{
			name:    "pattern only",
			pattern: "TODO",
			path:    "",
			want:    "Searching for `TODO`",
		},
		{
			name:    "pattern with path",
			pattern: "func main",
			path:    "cmd/main.go",
			want:    "Searching for `func main` in `cmd/main.go`",
		},
		{
			name:    "regex pattern",
			pattern: "error.*handler",
			path:    "internal/",
			want:    "Searching for `error.*handler` in `internal/`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatGrepToolMessage(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("FormatGrepToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatEditToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "simple file",
			filePath: "main.go",
			want:     "Editing `main.go`",
		},
		{
			name:     "nested file",
			filePath: "internal/server/handler.go",
			want:     "Editing `internal/server/handler.go`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEditToolMessage(tt.filePath)
			if got != tt.want {
				t.Errorf("FormatEditToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatWriteToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "new file",
			filePath: "README.md",
			want:     "Writing `README.md`",
		},
		{
			name:     "config file",
			filePath: "config/settings.json",
			want:     "Writing `config/settings.json`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatWriteToolMessage(tt.filePath)
			if got != tt.want {
				t.Errorf("FormatWriteToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatLSToolMessage(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "current directory",
			path: ".",
			want: "Listing `.`",
		},
		{
			name: "subdirectory",
			path: "internal/",
			want: "Listing `internal/`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLSToolMessage(tt.path)
			if got != tt.want {
				t.Errorf("FormatLSToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatGlobToolMessage(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{
			name:    "simple pattern",
			pattern: "*.go",
			want:    "`*.go`",
		},
		{
			name:    "complex pattern",
			pattern: "**/*.{js,ts}",
			want:    "`**/*.{js,ts}`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatGlobToolMessage(tt.pattern)
			if got != tt.want {
				t.Errorf("FormatGlobToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTaskToolMessage(t *testing.T) {
	tests := []struct {
		name        string
		description string
		prompt      string
		want        string
	}{
		{
			name:        "simple task",
			description: "コードベース全体を分析",
			prompt:      "Analyze the codebase and find issues",
			want:        "Task: コードベース全体を分析\n```\nAnalyze the codebase and find issues\n```",
		},
		{
			name:        "long prompt truncated",
			description: "Complex analysis",
			prompt:      strings.Repeat("a", 600),
			want:        "Task: Complex analysis\n```\n" + strings.Repeat("a", 500) + "...\n```",
		},
		{
			name:        "prompt with triple backticks",
			description: "Code generation",
			prompt:      "Generate code with ```python\nprint('hello')\n```",
			want:        "Task: Code generation\n```\nGenerate code with \\`\\`\\`python\nprint('hello')\n\\`\\`\\`\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTaskToolMessage(tt.description, tt.prompt)
			if got != tt.want {
				t.Errorf("FormatTaskToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatWebFetchToolMessage(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		prompt string
		want   string
	}{
		{
			name:   "simple web fetch",
			url:    "https://example.com",
			prompt: "Extract the main content",
			want:   "Fetching: <https://example.com>\n```\nExtract the main content\n```",
		},
		{
			name:   "blog post fetch",
			url:    "https://blog.yuyat.jp/post/how-i-learn-about-cloud-services/",
			prompt: "Extract the article title, content summary, date, key points about learning cloud services, and any specific techniques or resources mentioned",
			want:   "Fetching: <https://blog.yuyat.jp/post/how-i-learn-about-cloud-services/>\n```\nExtract the article title, content summary, date, key points about learning cloud services, and any specific techniques or resources mentioned\n```",
		},
		{
			name:   "long prompt truncated",
			url:    "https://docs.example.com/api",
			prompt: strings.Repeat("a", 400),
			want:   "Fetching: <https://docs.example.com/api>\n```\n" + strings.Repeat("a", 300) + "...\n```",
		},
		{
			name:   "prompt with triple backticks",
			url:    "https://github.com/user/repo",
			prompt: "Extract code samples ```javascript\nconsole.log('test');\n```",
			want:   "Fetching: <https://github.com/user/repo>\n```\nExtract code samples \\`\\`\\`javascript\nconsole.log('test');\n\\`\\`\\`\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatWebFetchToolMessage(tt.url, tt.prompt)
			if got != tt.want {
				t.Errorf("FormatWebFetchToolMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCompletionMessage(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		turns     int
		cost      float64
		want      string
	}{
		{
			name:      "normal session",
			sessionID: "session-123",
			turns:     5,
			cost:      0.05,
			want: "✅ Session completed\n" +
				"Session ID: `session-123`\n" +
				"Turns: 5\n" +
				"Cost: $0.050000 USD",
		},
		{
			name:      "high cost session",
			sessionID: "session-456",
			turns:     20,
			cost:      1.5,
			want: "✅ Session completed\n" +
				"Session ID: `session-456`\n" +
				"Turns: 20\n" +
				"Cost: $1.500000 USD\n" +
				"⚠️ High cost session",
		},
		{
			name:      "UUID session ID",
			sessionID: "d423f0ad-9ba7-46e4-8afb-869f70a89fff",
			turns:     10,
			cost:      0.25,
			want: "✅ Session completed\n" +
				"Session ID: `d423f0ad-9ba7-46e4-8afb-869f70a89fff`\n" +
				"Turns: 10\n" +
				"Cost: $0.250000 USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCompletionMessage(tt.sessionID, tt.turns, tt.cost)
			if got != tt.want {
				t.Errorf("FormatCompletionMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatErrorMessage(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{
			name:      "simple session ID",
			sessionID: "session-123",
			want:      "❌ Session ended with error\nSession ID: `session-123`",
		},
		{
			name:      "UUID session ID",
			sessionID: "d423f0ad-9ba7-46e4-8afb-869f70a89fff",
			want:      "❌ Session ended with error\nSession ID: `d423f0ad-9ba7-46e4-8afb-869f70a89fff`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorMessage(tt.sessionID)
			if got != tt.want {
				t.Errorf("FormatErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 5 * time.Second,
			want:     "5s",
		},
		{
			name:     "minutes and seconds",
			duration: 125 * time.Second,
			want:     "2m5s",
		},
		{
			name:     "exact minute",
			duration: 60 * time.Second,
			want:     "1m0s",
		},
		{
			name:     "hours, minutes and seconds",
			duration: 3665 * time.Second,
			want:     "1h1m5s",
		},
		{
			name:     "exact hour",
			duration: 3600 * time.Second,
			want:     "1h0m0s",
		},
		{
			name:     "multiple hours",
			duration: 7325 * time.Second,
			want:     "2h2m5s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
