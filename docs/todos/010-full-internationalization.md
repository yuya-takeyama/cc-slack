---
title: Full Internationalization to English
status: draft
---

# Full Internationalization to English

This document outlines all the Japanese text that needs to be translated to English across the entire cc-slack codebase.

## Goal

Make cc-slack fully accessible to international users by translating all Japanese text to English in:
- Source code messages
- Comments
- Documentation
- UI elements
- Configuration

## Phase 1: Core Message Translation

### 1.1 Session Messages (`internal/messages/format.go`)

**Session Start Message:**
- `🚀 Claude Code セッション開始` → `🚀 Claude Code session started`
- `セッションID:` → `Session ID:`
- `作業ディレクトリ:` → `Working directory:`
- `モデル:` → `Model:`

**Session Complete Message:**
- `✅ セッション完了` → `✅ Session completed`
- `セッションID:` → `Session ID:`
- `実行時間:` → `Duration:`
- `ターン数:` → `Turns:`
- `コスト:` → `Cost:`
- `使用トークン: 入力=%d, 出力=%d` → `Tokens used: input=%d, output=%d`
- `⚠️ 高コストセッション` → `⚠️ High cost session`

**Timeout Message:**
- `⏰ セッションがタイムアウトしました` → `⏰ Session timed out`
- `アイドル時間: %d分` → `Idle time: %d minutes`
- `新しいセッションを開始するには、再度メンションしてください。` → `To start a new session, please mention me again.`

**Error Message:**
- `❌ セッションがエラーで終了しました` → `❌ Session ended with error`

**Duration Format (`FormatDuration`):**
- `%d秒` → `%ds`
- `%d分%d秒` → `%dm%ds`
- `%d時間%d分%d秒` → `%dh%dm%ds`

### 1.2 Slack Handler Messages (`internal/slack/handler.go`)

**Approval Prompt:**
- `ツールの実行許可が必要です` → `Tool execution permission required`
- `ツール:` → `Tool:`
- `承認` → `Approve`
- `拒否` → `Deny`
- `コマンド:` → `Command:`
- `説明:` → `Description:`
- `内容:` → `Content:`
- `ファイルパス:` → `File path:`

### 1.3 Error Messages (`internal/session/manager.go`)

- `⚠️ エラー: %v` → `⚠️ Error: %v`

## Phase 2: Code Comments Translation

### 2.1 Japanese Comments in Code

**`internal/config/config.go`:**
- Line 11-12: `// SlackワークスペースのSubdomain` → `// Slack workspace subdomain`
- Line 12: `// TODO: 将来的に複数workspace対応時はDBに移行` → `// TODO: Migrate to DB when supporting multiple workspaces`

**`internal/process/resume.go`:**
- Line 28: `// 1. threads テーブルから thread_id を取得` → `// 1. Get thread_id from threads table`
- Line 40: `// 2. sessions テーブルから最新の completed セッションを取得` → `// 2. Get latest completed session from sessions table`

**`internal/messages/format.go`:**
- Line 164-166: Example comments showing Japanese output → Update to show English output

## Phase 3: Documentation Translation

### 3.1 TODO Documents (`docs/todos/`)

**004-web-management-console.md:**
- Title and all Japanese sections need translation
- Technical terms and code samples remain unchanged

### 3.2 Test Descriptions

All test descriptions that include Japanese text need to be updated to English in:
- `internal/messages/format_test.go`
- `internal/slack/handler_test.go`

## Phase 4: Web Console Translation

### 4.1 Check for Any Japanese Text
- Currently no Japanese text found in web/src components
- Verify all UI elements are in English

## Implementation Plan

### Step 1: Message Format Updates
- [ ] Update all message formatting functions in `internal/messages/format.go`
- [ ] Update corresponding tests in `internal/messages/format_test.go`

### Step 2: Slack Handler Updates
- [ ] Update approval prompt messages in `internal/slack/handler.go`
- [ ] Update test expectations in `internal/slack/handler_test.go`

### Step 3: Error Message Updates
- [ ] Update error handler in `internal/session/manager.go`

### Step 4: Comment Translation
- [ ] Translate all Japanese comments in Go source files
- [ ] Update example comments showing output format

### Step 5: Documentation Updates
- [ ] Translate or replace Japanese TODO documents
- [ ] Update any Japanese text in design/requirements docs

### Step 6: Testing
- [ ] Run all tests to ensure they pass with new messages
- [ ] Manually test Slack integration to verify message display
- [ ] Check web console for any missed translations

## Notes

- Keep emoji usage consistent (🚀, ✅, ⏰, ❌, ⚠️)
- Maintain professional tone in English messages
- Use standard English time format (e.g., "5s" instead of "5 seconds")
- Ensure all user-facing messages are clear and concise

## Verification Checklist

- [ ] No Japanese characters remain in source code (except test data if needed)
- [ ] All comments are in English
- [ ] Documentation is in English
- [ ] Test descriptions are in English
- [ ] Web UI is fully in English
- [ ] Error messages are in English
- [ ] Configuration comments are in English