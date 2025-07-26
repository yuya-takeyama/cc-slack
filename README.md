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
export CC_SLACK_DEFAULT_WORKDIR=/tmp/cc-slack          # default: /tmp/cc-slack-workspace
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
   - `app_mentions:read`
   - `chat:write`
   - `channels:history`
   - `groups:history`
3. Enable Event Subscriptions:
   - Request URL: `https://your-domain/slack/events`
   - Subscribe to bot events: `app_mention`, `message.channels`
4. Enable Interactive Components:
   - Request URL: `https://your-domain/slack/interactive`
5. Install the app to your workspace

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

## License

MIT