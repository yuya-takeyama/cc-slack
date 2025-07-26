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

### Exposing Local Development to Slack

Since Slack requires HTTPS endpoints for webhooks, you need to expose your local cc-slack instance. We recommend using Tailscale Funnel:

#### Using Tailscale Funnel (Recommended)

1. Enable Funnel in your Tailnet settings
2. Run cc-slack locally (default port 8080)
3. Expose it with Tailscale:
   ```bash
   tailscale funnel 8080
   ```
4. Use the provided `https://<machine-name>.<tailnet-name>.ts.net` URL for Slack webhook URLs

#### Alternative: ngrok

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

## License

MIT
