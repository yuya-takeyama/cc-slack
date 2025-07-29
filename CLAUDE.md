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

**Prerequisites:**
```bash
# Install frontend dependencies first
cd web
pnpm install
cd ..
```

**Build:**
```bash
./scripts/build
```

This script will:
1. Build the frontend (React/Vite)
2. Copy the frontend dist to internal/web/dist
3. Build the Go binary with embedded frontend

**Development mode:**
```bash
./scripts/dev
```

This will watch for frontend changes and rebuild automatically (requires fswatch).

### Run

```bash
./cc-slack
```

### Testing and Code Quality Checks

```bash
./scripts/check-all
```

This script runs all necessary checks including:
- Frontend and backend builds
- Go static analysis (`go vet`)
- All tests
- Dependency cleanup (`go mod tidy`)
- Frontend checks (`pnpm all`)

The script works from any directory within the project.

### Database Development (SQLite + sqlc + golang-migrate)

#### Database Schema Management

**Creating new migrations:**

```bash
# Create a new migration
migrate create -ext sql -dir migrations -seq create_threads_table

# This creates two files:
# - migrations/000001_create_threads_table.up.sql
# - migrations/000001_create_threads_table.down.sql
```

**Migration execution:**

cc-slack automatically runs migrations on startup. No manual intervention required!

```go
// This happens automatically in main.go:
if err := database.Migrate(sqlDB, cfg.Database.MigrationsPath); err != nil {
    log.Fatalf("Failed to run migrations: %v", err)
}
```

#### sqlc Workflow

**Writing queries:**

1. Create SQL query files in `internal/db/queries/`:
   ```sql
   -- internal/db/queries/sessions.sql
   
   -- name: GetSession :one
   SELECT * FROM sessions
   WHERE session_id = ? LIMIT 1;
   
   -- name: ListActiveSessions :many
   SELECT * FROM sessions
   WHERE status = 'active'
   ORDER BY started_at DESC;
   ```

2. Generate Go code:
   ```bash
   sqlc generate
   ```

3. Use generated code:
   ```go
   queries := db.New(sqliteDB)
   session, err := queries.GetSession(ctx, sessionID)
   ```

**sqlc naming conventions:**
- `:one` - Returns single row, error if not found
- `:many` - Returns slice of rows
- `:exec` - Executes query, returns no rows
- `:execrows` - Executes query, returns affected rows count

#### Development Workflow for Database Changes

1. **Adding a new table/column:**
   ```bash
   # 1. Create migration
   migrate create -ext sql -dir migrations -seq add_user_id_to_sessions
   
   # 2. Write migration SQL
   # migrations/XXXXXX_add_user_id_to_sessions.up.sql:
   ALTER TABLE sessions ADD COLUMN user_id TEXT;
   
   # migrations/XXXXXX_add_user_id_to_sessions.down.sql:
   ALTER TABLE sessions DROP COLUMN user_id;
   
   # 3. Migration will be applied automatically on next cc-slack startup
   # (Or apply manually for immediate testing: migrate -database "sqlite3://./data/cc-slack.db" -path ./migrations up)
   
   # 4. Update sqlc queries if needed
   # 5. Regenerate sqlc code
   sqlc generate
   ```

2. **Adding a new query:**
   ```bash
   # 1. Add query to internal/db/queries/*.sql
   # 2. Run sqlc generate
   sqlc generate
   # 3. Use the generated function in your code
   ```

#### Directory Structure

```
cc-slack/
├── migrations/                    # Database migration files
│   ├── 000001_init.up.sql
│   ├── 000001_init.down.sql
│   └── ...
├── internal/
│   ├── db/                       # Generated sqlc code
│   │   ├── db.go                 # Interface definitions
│   │   ├── models.go             # Generated models
│   │   ├── sessions.sql.go       # Generated queries
│   │   └── queries/              # SQL query definitions
│   │       ├── sessions.sql
│   │       ├── threads.sql
│   │       └── ...
│   └── ...
├── sqlc.yaml                     # sqlc configuration
└── data/                         # Database files (git-ignored)
    └── cc-slack.db

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

### Development Workflow with Restart

During development, use the cc-slack-manager for restarting cc-slack:

**To restart cc-slack when explicitly requested:**
```bash
./scripts/restart
```

**Important:** Claude Code should only run the restart script when explicitly requested by the user. Do not automatically restart after code changes, as this will terminate cc-slack and end the current session.

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
- **Session Resume**: Sessions can be resumed within a configurable time window (default: 1 hour)

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
- `internal/process/resume.go`: Session resume functionality
- `internal/slack/`: Slack event handling
- `internal/mcp/`: MCP server implementation
- `cmd/cc-slack/`: Main application entry point
- `internal/db/`: Database access layer (sqlc generated)
- `internal/database/`: Database connection and migration utilities
- `internal/config/`: Configuration management (Viper)
- `internal/session/db_manager.go`: Session manager with database persistence
- `internal/web/`: Web management console
- `internal/workspace/`: Working directory management
- `migrations/`: Database schema migrations

## Logging

Logs are written to `logs/` directory with timestamp:
- Format: `claude-YYYYMMDD-HHMMSS.log`
- Using zerolog for structured logging
- All Claude process communication is logged

## Environment Variables

**Slack Configuration:**
- `CC_SLACK_SLACK_BOT_TOKEN`: Slack bot user OAuth token
- `CC_SLACK_SLACK_APP_TOKEN`: Slack app-level token for Socket Mode
- `CC_SLACK_SLACK_SIGNING_SECRET`: For request verification
- `CC_SLACK_SLACK_ASSISTANT_USERNAME`: Claude response username (optional)
- `CC_SLACK_SLACK_ASSISTANT_ICON_EMOJI`: Claude response emoji (optional)
- `CC_SLACK_SLACK_ASSISTANT_ICON_URL`: Claude response icon URL (optional)

**Server Configuration:**
- `CC_SLACK_SERVER_PORT`: HTTP server port (default: 8080)
- `CC_SLACK_SERVER_BASE_URL`: Base URL for MCP connection

**Claude Configuration:**
- `CC_SLACK_CLAUDE_EXECUTABLE`: Claude CLI path (default: claude)
- `CC_SLACK_CLAUDE_PERMISSION_PROMPT_TOOL`: Permission prompt tool name

**Database Configuration:**
- `CC_SLACK_DATABASE_PATH`: SQLite database path (default: ./data/cc-slack.db)
- `CC_SLACK_DATABASE_MIGRATIONS_PATH`: Migrations directory (default: ./migrations)

**Session Configuration:**
- `CC_SLACK_SESSION_TIMEOUT`: Session timeout duration (default: 30m)
- `CC_SLACK_SESSION_CLEANUP_INTERVAL`: Cleanup interval (default: 5m)
- `CC_SLACK_SESSION_RESUME_WINDOW`: Resume window duration (default: 1h)

**Working Directory Configuration:**
- `CC_SLACK_WORKING_DIRECTORIES_DEFAULT`: Default working directory

**Note:** All environment variables can also be set in `config.yaml` file using Viper

## Session Resume Feature

The session resume feature allows users to continue previous Claude Code sessions within the same Slack thread:

### How it works:

1. When a new mention is made in a thread that had a previous session
2. The system checks if the previous session ended within the resume window (default: 1 hour)
3. If eligible, Claude Code is started with `--resume` option using the previous session ID
4. The conversation context is preserved and continues from where it left off

### Configuration:

- **Resume Window**: Set via `CC_SLACK_SESSION_RESUME_WINDOW` (default: 1h)
- **Database**: Sessions are persisted in SQLite database
- **Automatic**: No user action required - resume happens automatically when conditions are met

### Benefits:

- Seamless continuation of work across breaks
- Preserves conversation context and memory
- Reduces token usage by avoiding repetitive context
- Maintains TODO lists and project state

## MCP Tools

- `approval_prompt`: Handles permission requests in Slack threads

## Documentation Structure

The project documentation is organized as follows:

### Core Documentation
- **`docs/requirements.md`**: Product Requirements Document (PRD) - Defines what we're building and why
- **`docs/design.md`**: Software Design Document - Describes how the system is architected

### Task Tracking
- **`docs/todos/`**: Directory containing task tracking documents
  - Each file follows the pattern `NNN-description.md` (e.g., `001-initial.md`)
  - Files include YAML front matter with:
    - `title`: Brief description of what the tasks accomplish
    - `status`: Current status (`draft`, `in_progress`, or `done`)
  - The body contains task checklists and progress updates

### Documentation Guidelines
- **Requirements and Design**: These documents represent the stable specifications and are updated only when requirements or architecture changes
- **Task Documents**: Track implementation progress and are actively updated during development
- **Separation of Concerns**: This structure separates "what and how" (requirements/design) from "progress tracking" (todos)

## Language and Communication Guidelines

### Software Language Policy
- **All software components must be in English**: This includes but is not limited to:
  - User-facing messages and UI text
  - Log messages and error messages
  - Code comments and documentation
  - Git commit messages (already specified above)
  - Pull Request titles and descriptions
  - GitHub Issues
  - API responses and error messages
  - Configuration file comments
  - Test descriptions and assertions

### Why English?
To ensure cc-slack is accessible to the global developer community and maintains consistency across all components.
