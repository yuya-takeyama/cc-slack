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

### Go Import Management

**Development workflow to prevent import removal by goimports Hook:**

Claude Code Hooks automatically runs goimports, which removes unused imports. To prevent this issue:

1. **❌ BAD Pattern (imports get removed):**
   ```go
   import (
       "fmt"
       "encoding/json"  // Not used yet → Will be removed!
   )
   
   // Planning to use json.Marshal() later...
   ```

2. **✅ GOOD Pattern (recommended workflow):**
   ```go
   // 1. Write the actual code first
   func processData(data interface{}) (string, error) {
       bytes, err := json.Marshal(data)  // Code that uses json
       if err != nil {
           return "", err
       }
       return string(bytes), nil
   }
   
   // 2. Then add necessary imports (or let goimports add them automatically)
   import (
       "encoding/json"  // Already in use, won't be removed!
   )
   ```

**Development Guidelines:**
- Write the implementation code first
- Add imports afterwards (or let goimports auto-add them)
- When multiple imports are needed, write the code that uses them before organizing imports

### Development Workflow with Auto-Restart

During development, use the cc-slack-manager for automatic restarts:

**After making code changes, restart cc-slack:**
```bash
./scripts/restart
```

**Claude Code should automatically run the restart script after significant code changes to cc-slack.**

### Checking cc-slack Status

Use the status script to check if cc-slack is running:

```bash
./scripts/status
```

This will show:
- Whether cc-slack is running
- Process ID (PID)
- Uptime and start time
- Full JSON status details

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

### Structured Logging

- Use zerolog for all logging needs
- Log to files in `logs/` directory, not stderr/stdout
- Include structured context (component, type, direction, etc.)
- Log levels: Debug (verbose), Info (normal), Warn (issues), Error (failures)

### Process Management

- Each Slack thread gets its own Claude Code process
- Processes are managed with proper lifecycle (create, communicate, cleanup)
- Resource cleanup is mandatory (config files, log files, processes)
- Use context.Context for cancellation support

### Error Handling

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- Clean up resources on error paths
- Log errors before returning them
- Fail fast on initialization errors

### Testing Strategy

1. **Unit Tests**: Test pure functions with table-driven tests
2. **Integration Tests**: Test component interactions (future)
3. **Test Organization**: Keep tests close to code (`*_test.go` in same package)
4. **Test Naming**: Use descriptive names that explain the test scenario

### Code Organization

- **cmd/**: Entry points and configuration
- **internal/**: Business logic, not exposed to external packages
- **Pure functions**: Extract and place near their primary usage
- **Interfaces**: Define at the consumer side, not provider

### Slack Integration

- Each Slack thread maps to one Claude session
- Thread TS (timestamp) is the unique identifier
- Bot mentions are stripped before sending to Claude
- Responses are posted back to the same thread
- Use structured blocks for approval prompts

### MCP (Model Context Protocol) Design

- HTTP-based MCP server for tool integration
- Custom tools are prefixed with `mcp__cc-slack__`
- Permission prompts are handled via Slack interactive messages
- Tool responses must be JSON-serializable
- Always validate tool inputs

### Concurrency and Goroutines

- One goroutine per Claude process for stdout/stderr reading
- Use channels for inter-goroutine communication
- Proper cleanup with context cancellation
- No shared state without proper synchronization (mutex)

### Security Considerations

- Validate all Slack requests with signing secret
- Never log sensitive tokens or credentials
- Sanitize user inputs before processing
- Use temporary files with proper permissions
- Clean up temporary resources after use

### Future Extensibility

- Design interfaces for easy mocking and testing
- Keep coupling loose between components
- Use dependency injection for external services
- Support multiple MCP tools through plugin architecture
- Consider rate limiting and resource quotas
- Plan for horizontal scaling (multiple instances)

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
