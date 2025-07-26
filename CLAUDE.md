# cc-slack

Claude Code integration for Slack - enables running Claude Code sessions through Slack threads.

## Project Overview

This project provides a bridge between Slack and Claude Code, allowing users to interact with Claude through Slack threads. Each thread creates a dedicated Claude Code session with isolated context.

## Architecture

- **HTTP Server**: Handles Slack events and MCP protocol
- **Process Manager**: Manages Claude Code processes per Slack thread
- **MCP Integration**: Provides custom tools for Slack-specific operations

## Development

### Prerequisites

- Go 1.24.4+
- Claude Code CLI installed
- Slack app credentials

### Build

```bash
go build -o cc-slack ./cmd/cc-slack
```

### Run

```bash
./cc-slack
```

### Testing

```bash
go test ./...
```

### Code Quality Checks

**Always run these commands after making changes:**

```bash
# Run static analysis
go vet ./...

# Run tests
go test ./...

# Clean up dependencies
go mod tidy
```

## Design Principles

### Pure Functions and Unit Testing

We actively extract pure functions from complex logic to improve testability and maintainability:

1. **Extract Pure Functions**: Any logic that doesn't depend on external state should be extracted as a pure function
2. **Test Coverage**: All pure functions must have comprehensive unit tests
3. **Examples**:
   - `generateLogFileName()`: Generates log filenames with timestamps
   - `buildMCPConfig()`: Builds MCP configuration objects
   - `removeBotMention()`: Removes bot mentions from Slack messages
   - `getEnv()`: Gets environment variables with defaults

When adding new features, always consider:
- Can this logic be extracted as a pure function?
- Is the function easily testable?
- Does it have clear inputs and outputs without side effects?

## Key Components

- `internal/process/claude.go`: Claude Code process management
- `internal/slack/`: Slack event handling
- `internal/mcp/`: MCP server implementation
- `cmd/cc-slack/`: Main application entry point

## Logging

Logs are written to `logs/` directory with timestamp:
- Format: `claude-YYYYMMDD-HHMMSS.log`
- Using zerolog for structured logging
- All Claude process communication is logged

## Environment Variables

- `SLACK_BOT_TOKEN`: Slack bot user OAuth token
- `SLACK_APP_TOKEN`: Slack app-level token for Socket Mode
- `SLACK_SIGNING_SECRET`: For request verification

## MCP Tools

- `approval_prompt`: Handles permission requests in Slack threads
