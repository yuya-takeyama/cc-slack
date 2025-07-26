# cc-slack

Slack で Claude Code と会話する

## 概要

cc-slack は Claude Code と Slack 上でインタラクションするためのソフトウェアです。Slack から直接 Claude Code に指示を出し、作業の進捗を確認できます。

## 主な機能

- Slack メンションで Claude Code セッションを開始
- スレッド内での継続的な対話
- MCP (Model Context Protocol) サーバーとして動作
- approval_prompt による Slack 承認統合（開発中）

## セットアップ

### 必要なもの

- Go 1.24.4 以上
- Claude Code CLI
- Slack Bot Token と Signing Secret

### 環境変数

```bash
# 必須
export SLACK_BOT_TOKEN=xoxb-your-bot-token
export SLACK_SIGNING_SECRET=your-signing-secret

# オプション
export CC_SLACK_PORT=8080
export CC_SLACK_BASE_URL=http://localhost:8080
export CC_SLACK_DEFAULT_WORKDIR=/tmp/cc-slack-workspace
```

### ビルドと実行

```bash
# 依存関係のインストール
go mod download

# ビルド
go build -o cc-slack cmd/cc-slack/main.go

# 実行
./cc-slack
```

### Slack Bot の設定

1. Slack App を作成
2. Bot User を追加
3. 以下の OAuth Scopes を設定:
   - `chat:write`
   - `app_mentions:read`
   - `channels:history`
   - `groups:history`
   - `im:history`
   - `mpim:history`
4. Event Subscriptions を有効化し、Request URL に `https://your-domain/slack/events` を設定
5. 以下のイベントをサブスクライブ:
   - `app_mention`
   - `message.channels`
   - `message.groups`
   - `message.im`
   - `message.mpim`
6. Interactive Components を有効化し、Request URL に `https://your-domain/slack/interactive` を設定

## 使い方

1. Slack チャンネルで Bot をメンション
   ```
   @cc-slack READMEを作成して
   ```

2. Claude Code が新しいスレッドでセッションを開始

3. スレッド内で追加の指示
   ```
   インストール手順も追加して
   ```

## アーキテクチャ

- **統合HTTPサーバー**: Slack webhookとMCPリクエストを単一ポートで処理
- **MCP Server**: Streamable HTTP transportでClaude Codeと通信
- **Slack Bot**: イベント処理とメッセージ投稿
- **セッション管理**: スレッドごとのClaude Codeプロセス管理

## 開発状況

MVP実装完了。以下の機能は開発中:
- approval_prompt のSlack統合
- チャンネルごとの設定管理
- ファイル共有機能