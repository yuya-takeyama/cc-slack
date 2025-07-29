---
title: Migrate from app_mention to message event for better image handling
status: done
---

# 009: Migrate from app_mention to message event

## Overview

app_mention イベントから message イベントへの移行により、画像アップロード機能の効率を改善し、Slack API の rate limit 問題を解決する。

## Background

### Current Issues

1. **app_mention イベントの制限**
   - attachments フィールドを持たない
   - 画像の有無を判断できないため、全てのメッセージで GetConversationReplies/GetConversationHistory を呼ぶ必要がある
   - これが Slack API の厳しい rate limit に引っかかる原因となっている

2. **API Rate Limits**
   - conversations.history, conversations.replies は特に厳しい rate limit が設定されている
   - 頻繁な呼び出しはアプリケーションの停止につながる可能性がある

### Solution

message イベントを使用することで：
- attachments フィールドが直接含まれる
- 不要な API 呼び出しを削減できる
- より効率的な画像処理が可能になる

## Current Implementation Analysis

### Event Flow
1. `HandleEvent` (internal/slack/handler.go:123)
   - slackevents.CallbackEvent を処理
   - AppMentionEvent の場合 `handleAppMention` を呼び出す

2. `handleAppMention` (internal/slack/handler.go:179)
   - bot mention を削除
   - thread かどうかで処理を分岐

3. `fetchAndSaveImages` (internal/slack/handler.go:643)
   - GetConversationReplies または GetConversationHistory を呼び出し
   - 画像をダウンロードして保存

## Implementation Plan

### 1. Event Handler の変更

```go
// HandleEvent の switch 文に追加
case *slackevents.MessageEvent:
    h.handleMessage(ev)
```

### 2. Message Event Handler の実装

```go
func (h *Handler) handleMessage(event *slackevents.MessageEvent) {
    // フィルタリング処理
    if !h.shouldProcessMessage(event) {
        return
    }
    
    // bot mention の処理（オプション）
    text := h.extractMessageText(event)
    
    // 画像処理（直接 attachments から取得）
    imagePaths := h.processAttachments(event.Attachments)
    
    // セッション処理（既存のロジックを流用）
    // ...
}
```

### 3. Message Filtering Configuration

`internal/config/config.go` に追加：

```go
type MessageFilterConfig struct {
    Enabled         bool     `mapstructure:"enabled"`
    IncludePatterns []string `mapstructure:"include_patterns"`
    ExcludePatterns []string `mapstructure:"exclude_patterns"`
    RequireMention  bool     `mapstructure:"require_mention"`
}

type SlackConfig struct {
    // ... existing fields ...
    MessageFilter MessageFilterConfig `mapstructure:"message_filter"`
}
```

### 4. フィルタリング実装

```go
func (h *Handler) shouldProcessMessage(event *slackevents.MessageEvent) bool {
    // bot からのメッセージは除外
    if event.BotID != "" {
        return false
    }
    
    // サブタイプのチェック（編集、削除などを除外）
    if event.SubType != "" {
        return false
    }
    
    // 設定に基づくフィルタリング
    if h.messageFilter.RequireMention {
        // bot への mention が含まれているかチェック
        if !h.containsBotMention(event.Text) {
            return false
        }
    }
    
    // パターンマッチング
    if len(h.messageFilter.IncludePatterns) > 0 {
        // include パターンのチェック
    }
    
    if len(h.messageFilter.ExcludePatterns) > 0 {
        // exclude パターンのチェック
    }
    
    return true
}
```

### 5. 画像処理の簡略化

```go
func (h *Handler) processAttachments(attachments []slack.Attachment) []string {
    var imagePaths []string
    
    for _, attachment := range attachments {
        if strings.HasPrefix(attachment.MimeType, "image/") {
            // 直接ダウンロード処理
            path, err := h.downloadAttachment(attachment)
            if err == nil {
                imagePaths = append(imagePaths, path)
            }
        }
    }
    
    return imagePaths
}
```

## Migration Strategy

### Phase 1: 並行実装
1. message イベントハンドラーを追加
2. フィルタリング設定を実装
3. 既存の app_mention ハンドラーは残す

### Phase 2: テストと検証
1. message イベントでの動作確認
2. フィルタリングの動作確認
3. 画像処理の動作確認

### Phase 3: 切り替え
1. デフォルトで message イベントを使用
2. app_mention は fallback として残す
3. 設定で切り替え可能にする

## Configuration Examples

### Bot Mention のみを処理する設定
```yaml
slack:
  message_filter:
    enabled: true
    require_mention: true
```

### 特定のパターンを含むメッセージを処理
```yaml
slack:
  message_filter:
    enabled: true
    include_patterns:
      - "analyze"
      - "help"
    require_mention: false
```

## Testing Checklist

- [ ] message イベントが正しく受信される
- [ ] bot からのメッセージが除外される
- [ ] mention フィルタリングが動作する
- [ ] 画像が attachments から直接取得される
- [ ] GetConversationHistory/Replies が呼ばれない
- [ ] 既存のセッション管理が動作する
- [ ] thread での動作が正しい
- [ ] 設定の切り替えが動作する

## Backward Compatibility

- app_mention イベントハンドラーは残す
- 設定で event type を選択可能にする
- デフォルトは message イベントを使用

## Security Considerations

- message イベントは全てのメッセージを受信するため、適切なフィルタリングが重要
- bot token の権限確認（channels:history, groups:history, im:history, mpim:history が必要）
- 意図しないメッセージの処理を防ぐため、フィルタリング設定のデフォルトは厳しくする

## References

- [Slack Events API - message event](https://api.slack.com/events/message)
- [Slack Events API - app_mention event](https://api.slack.com/events/app_mention)
- [Slack API Rate Limits](https://api.slack.com/docs/rate-limits)

## Implementation Progress

### Completed ✅

1. **Message Event Handler**
   - Added MessageEvent case to HandleEvent switch
   - Implemented handleMessage function
   - Created separate handlers for thread messages and new sessions

2. **Configuration**
   - Added MessageFilterConfig to config structure
   - Implemented filtering options (require_mention, include/exclude patterns)
   - Added environment variable bindings

3. **Filtering Logic**
   - Implemented shouldProcessMessage with bot message filtering
   - Added SubType filtering (to exclude edits, deletes)
   - Bot mention detection using bot user ID

4. **Image Processing**
   - Implemented processMessageAttachments for direct image handling
   - Eliminated need for conversations.history/replies API calls
   - Images are processed directly from MessageEvent.Message.Files

5. **Bot Mention Detection**
   - Added bot user ID detection via AuthTest in NewHandler
   - Implemented containsBotMention to check for bot mentions
   - Made require_mention default to true for backward compatibility

6. **Error Handling**
   - Changed NewHandler to return (*Handler, error)
   - Added proper error handling for AuthTest failures
   - Updated main.go to handle initialization errors

### Testing Status ⚠️

- Unit tests are failing due to AuthTest being called in NewHandler
- Need to implement proper mocking strategy for Slack client

### Remaining Tasks 📝

1. **Fix Unit Tests**
   - Create SlackClient interface to enable mocking
   - Update tests to use mock client
   - Ensure all tests pass

2. **Integration Testing**
   - Test with actual Slack workspace
   - Verify message event subscription works
   - Confirm image processing without API calls

3. **Documentation**
   - Update README with message event subscription requirements
   - Add config.yaml.example with message_filter options
   - Document breaking changes (if any)

### Notes for Next Session

The implementation is functionally complete but tests need fixing. The main issue is that NewHandler now calls AuthTest to get the bot user ID, which fails in test environment with "invalid_auth".

Possible solutions:
1. Extract Slack client interface for mocking
2. Add test-specific constructor
3. Use dependency injection for AuthTest

The first option (interface extraction) is recommended as it follows Go best practices and enables proper unit testing.

### Latest Changes (Uncommitted)

1. **Bot Mention Validation**
   - Added proper bot user ID detection via AuthTest
   - Implemented containsBotMention to check for specific bot mentions  
   - Made NewHandler return error for fail-fast behavior

2. **Started AppMention Removal**
   - Removed AppMentionEvent case from HandleEvent switch
   - Started removing handleAppMention function
   - Plan to fully migrate to message events only

### Next Steps

1. **Complete AppMention Removal**
   - Remove handleThreadMessage(AppMentionEvent)
   - Remove handleNewSession(AppMentionEvent)  
   - Clean up any remaining AppMention references

2. **Fix Tests**
   - Implement Slack client interface for mocking
   - Update all tests to use mock client
   - Ensure tests pass with new error handling

3. **Update Documentation**
   - Remove app_mention from README
   - Update Event Subscriptions requirements
   - Document migration steps for users