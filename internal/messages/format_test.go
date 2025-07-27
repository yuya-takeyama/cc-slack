package messages

import (
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
			want: "🚀 Claude Code セッション開始\n" +
				"セッションID: session-123\n" +
				"作業ディレクトリ: /home/user/project\n" +
				"モデル: claude-3.5-sonnet",
		},
		{
			name:      "with spaces in path",
			sessionID: "session-456",
			cwd:       "/Users/name/My Documents/project",
			model:     "claude-3.5-sonnet",
			want: "🚀 Claude Code セッション開始\n" +
				"セッションID: session-456\n" +
				"作業ディレクトリ: /Users/name/My Documents/project\n" +
				"モデル: claude-3.5-sonnet",
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
		duration     time.Duration
		turns        int
		cost         float64
		inputTokens  int
		outputTokens int
		want         string
	}{
		{
			name:         "short session",
			duration:     5 * time.Second,
			turns:        3,
			cost:         0.05,
			inputTokens:  1000,
			outputTokens: 500,
			want: "✅ セッション完了\n" +
				"実行時間: 5秒\n" +
				"ターン数: 3\n" +
				"コスト: $0.050000 USD\n" +
				"使用トークン: 入力=1000, 出力=500",
		},
		{
			name:         "long session with high cost",
			duration:     125 * time.Second,
			turns:        20,
			cost:         1.5,
			inputTokens:  50000,
			outputTokens: 25000,
			want: "✅ セッション完了\n" +
				"実行時間: 2分5秒\n" +
				"ターン数: 20\n" +
				"コスト: $1.500000 USD\n" +
				"使用トークン: 入力=50000, 出力=25000\n" +
				"⚠️ 高コストセッション",
		},
		{
			name:         "very long session",
			duration:     3665 * time.Second,
			turns:        50,
			cost:         0.8,
			inputTokens:  100000,
			outputTokens: 50000,
			want: "✅ セッション完了\n" +
				"実行時間: 1時間1分5秒\n" +
				"ターン数: 50\n" +
				"コスト: $0.800000 USD\n" +
				"使用トークン: 入力=100000, 出力=50000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSessionCompleteMessage(tt.duration, tt.turns, tt.cost, tt.inputTokens, tt.outputTokens)
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
			want: "⏰ セッションがタイムアウトしました\n" +
				"アイドル時間: 15分\n" +
				"セッションID: session-123\n\n" +
				"新しいセッションを開始するには、再度メンションしてください。",
		},
		{
			name:        "long idle",
			idleMinutes: 120,
			sessionID:   "session-456",
			want: "⏰ セッションがタイムアウトしました\n" +
				"アイドル時間: 120分\n" +
				"セッションID: session-456\n\n" +
				"新しいセッションを開始するには、再度メンションしてください。",
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
		want     string
	}{
		{
			name:     "simple path",
			filePath: "main.go",
			want:     "`main.go`",
		},
		{
			name:     "path with spaces",
			filePath: "my file.txt",
			want:     "`my file.txt`",
		},
		{
			name:     "absolute path",
			filePath: "/Users/name/project/src/main.go",
			want:     "`/Users/name/project/src/main.go`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatReadToolMessage(tt.filePath)
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

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 5 * time.Second,
			want:     "5秒",
		},
		{
			name:     "minutes and seconds",
			duration: 125 * time.Second,
			want:     "2分5秒",
		},
		{
			name:     "exact minute",
			duration: 60 * time.Second,
			want:     "1分0秒",
		},
		{
			name:     "hours, minutes and seconds",
			duration: 3665 * time.Second,
			want:     "1時間1分5秒",
		},
		{
			name:     "exact hour",
			duration: 3600 * time.Second,
			want:     "1時間0分0秒",
		},
		{
			name:     "multiple hours",
			duration: 7325 * time.Second,
			want:     "2時間2分5秒",
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
