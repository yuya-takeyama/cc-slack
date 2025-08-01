# cc-slack

Interact with Claude Code via Slack

## Prerequisites

- Go 1.24.4+
- Claude Code CLI
- Slack Bot Token and Signing Secret

## Setup

### Environment Variables

```bash
# Required
export CC_SLACK_SLACK_BOT_TOKEN=xoxb-your-bot-token
export CC_SLACK_SLACK_SIGNING_SECRET=your-signing-secret

# Optional
export CC_SLACK_PORT=8080                              # default: 8080
export CC_SLACK_BASE_URL=http://localhost:8080         # default: http://localhost:8080
```

### Build and Run

```bash
# Build
go build -o cc-slack cmd/cc-slack/main.go

# Run
./cc-slack
```

### Slack App Configuration

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
5. Install the app to your workspace

### Exposing Local Development to Slack

Since Slack requires HTTPS endpoints for webhooks, you need to expose your local cc-slack instance.

#### ngrok

```bash
ngrok http 8080
```

Use the provided HTTPS URL for Slack configuration.

### Claude Code Configuration

cc-slack runs an MCP server for approval prompts. Configure Claude Code to connect:

```bash
# Add the MCP server
claude mcp add --transport http cc-slack http://localhost:8080/mcp

# Or with custom base URL
claude mcp add --transport http cc-slack ${CC_SLACK_BASE_URL}/mcp
```

## Usage

1. Mention the bot in any channel:
   ```
   @cc-slack create a hello world script
   ```

2. Claude Code will start a new session in a thread

3. Continue the conversation in the thread:
   ```
   add error handling please
   ```

Note: Claude Code sessions use the current working directory where cc-slack is running.

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
