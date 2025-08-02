# cc-slack

Interact with Claude Code via Slack

## Prerequisites

- Go 1.24.4+
- Claude Code CLI installed
- Slack workspace where you can create apps

## Setup

### 1. Create Slack App

> **Note**: For local development, you'll need to expose your local server to Slack. See [Exposing Local Development](#exposing-local-development-to-slack) section below.

1. Create a Slack App at [api.slack.com](https://api.slack.com)
2. Add Bot User OAuth Scopes:
   - `chat:write` - Always required for sending messages
   
   Choose based on where you'll use cc-slack:
   - `channels:history` - For public channels
   - `groups:history` - For private channels
   
   Additional scopes (required depending on your configuration):
   - `chat:write.customize` - Required if you set custom username or icon via `CC_SLACK_SLACK_ASSISTANT_USERNAME`, `CC_SLACK_SLACK_ASSISTANT_ICON_EMOJI`, or `CC_SLACK_SLACK_ASSISTANT_ICON_URL`
   - `groups:read` - Required for private channels when using conversations.info API
   - `channels:read` - Required for public channels when using conversations.info API
   - `files:read` - Required if you enable file upload support via `CC_SLACK_SLACK_FILE_UPLOAD_ENABLED=true` to download images from Slack messages
3. Enable Event Subscriptions:
   - Request URL: `https://your-domain/slack/events`
   - Subscribe to bot events (choose based on where you'll use cc-slack):
     - `message.channels` - For public channels
     - `message.groups` - For private channels
     - `message.im` - For direct messages
     - `message.mpim` - For multi-person direct messages
   
   Note: Only subscribe to the message events for the channel types you actually plan to use. For example, if you only use cc-slack in public channels, you only need `message.channels`.
4. Enable Interactive Components:
   - Request URL: `https://your-domain/slack/interactive`
5. Create Slash Command:
   - Command: `/cc` (recommended) or your preferred name
   - Request URL: `https://your-domain/slack/commands`
   - Short Description: Start a Claude Code session
   - Usage Hint: [prompt]
6. Install the app to your workspace and note down:
   - Bot User OAuth Token (starts with `xoxb-`)
   - Signing Secret (found in Basic Information)

### 2. Configure Environment Variables

Set the required environment variables with the values from your Slack App:

```bash
# Required
export CC_SLACK_SLACK_BOT_TOKEN=xoxb-your-bot-token
export CC_SLACK_SLACK_SIGNING_SECRET=your-signing-secret

# Optional
export CC_SLACK_PORT=8080                              # default: 8080
export CC_SLACK_BASE_URL=http://localhost:8080         # default: http://localhost:8080
export CC_SLACK_SLACK_SLASH_COMMAND_NAME=/cc          # default: /claude
```

### 3. Build and Run

```bash
# Build
go build -o cc-slack cmd/cc-slack/main.go

# Run (make sure environment variables are set)
./cc-slack
```

### Exposing Local Development to Slack

For local development, Slack requires HTTPS endpoints for webhooks. You need to expose your local cc-slack instance:

#### Using ngrok

```bash
# In another terminal, expose your local server
ngrok http 8080
```

Use the provided HTTPS URL (e.g., `https://abc123.ngrok.io`) for all Slack webhook URLs in your app configuration.

## Usage

cc-slack supports two modes of operation:

### Single Directory Mode (Quick Start)

Perfect for trying out cc-slack or when working with a single project:

```bash
# Start cc-slack with a specific working directory
./cc-slack --working-dirs /path/to/your/project

# Or with multiple directories (comma-separated)
./cc-slack --working-dirs /path/to/project1,/path/to/project2

# Or specify multiple times
./cc-slack --working-dirs /path/to/project1 --working-dirs /path/to/project2
```

In this mode:
- No configuration file needed
- Claude sessions will use the specified directory
- The `/cc` command shows only the prompt input (no directory selection)

### Multi-Directory Mode (Full Features)

For teams working with multiple projects:

1. Configure directories in `config.yaml`:
   ```yaml
   working_dirs:
     - name: frontend
       path: /Users/you/projects/web-app
       description: React frontend application
     
     - name: backend
       path: /Users/you/projects/api-server
       description: Node.js API server
   ```

2. Start cc-slack without the `--working-dirs` flag:
   ```bash
   ./cc-slack
   ```

3. Use the `/cc` slash command to start a session:
   - A modal opens with directory selection
   - Choose your working directory
   - Enter your initial prompt
   - Claude starts in the selected directory

### Interacting with Claude

Once a session is started (via either mode):

1. The bot creates a new thread with your initial prompt
2. Continue the conversation in the thread:
   ```
   add error handling please
   ```
3. Claude has access to the selected working directory (as permitted by Claude Code configuration)
4. Sessions automatically resume if you return within the resume window (default: 1 hour)

### Message Filtering

cc-slack now supports message event filtering for improved performance and flexibility:

- **Default behavior**: Only responds to bot mentions (backward compatible)
- **Image handling**: Processes images directly from message events without additional API calls
- **Pattern matching**: Configure include/exclude patterns for message processing

Configure in `config.yaml`:
```yaml
slack:
  message_filter:
    enabled: true
    require_mention: true  # Only respond to @mentions
    # include_patterns: ["analyze", "help"]  # Optional: only process matching messages
    # exclude_patterns: ["^#", "test"]      # Optional: skip matching messages
```

This feature significantly reduces Slack API rate limit issues when processing images.

## Development Tools

### Auto-Restart Manager

During development, you can use the included manager to automatically restart cc-slack when code changes:

```bash
# Start the manager (runs cc-slack and provides HTTP control endpoints)
./scripts/start

# Check status
./scripts/status

# Restart cc-slack (e.g., after code changes)
./scripts/restart

# Or use curl directly
curl -X POST http://localhost:10080/restart
```

The manager runs on port 10080 and provides:
- `GET /status` - Check if cc-slack is running
- `POST /restart` - Gracefully restart cc-slack
- `POST /stop` - Stop cc-slack
- `POST /start` - Start cc-slack

Claude Code can trigger restarts automatically by running:
```bash
./scripts/restart
```

## License

MIT
