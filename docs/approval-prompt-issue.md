# MCP Tool approval_prompt の不具合調査レポート

## 概要

cc-slack の approval_prompt ツールにおいて、Slack で「Allow」をクリックした場合にエラーが発生する問題の調査結果と修正案をまとめる。

## 問題の症状

- **Deny の場合**: 正常に動作
- **Allow の場合**: `Cannot read properties of undefined (reading 'length')` エラーが発生

## 調査結果

### 1. Claude Code SDK の仕様

公式ドキュメントによると、permission prompt tool は以下の形式の **JSON-stringified payload** を返す必要がある：

```json
// Allow の場合
{
  "behavior": "allow",
  "updatedInput": {...}  // 元のinputまたは修正されたinput
}

// Deny の場合
{
  "behavior": "deny",
  "message": "Human-readable explanation of denial"
}
```

重要なポイント：
- **JSON文字列**として返す必要がある
- `updatedInput` は Allow の場合は必須

### 2. 現在の実装の問題点

`internal/mcp/server.go` の HandleApprovalPrompt メソッド（198-210行目）で、以下のように実装されている：

```go
result := &mcpsdk.CallToolResultFor[PermissionPromptResponse]{
    Content: []mcpsdk.Content{
        &mcpsdk.TextContent{
            Text: string(jsonData),  // JSON文字列
        },
    },
    StructuredContent: promptResp,  // 構造体オブジェクト
}
```

**問題**: ContentとStructuredContentの両方を設定している。これにより、レスポンスが二重にエンコードされている可能性がある。

### 3. Allow 時の updatedInput の扱い

現在の実装では Allow 時に空の map を設定している：

```go
// internal/mcp/server.go 190-192行目
if promptResp.Behavior == "allow" && promptResp.UpdatedInput == nil {
    promptResp.UpdatedInput = map[string]interface{}{}
}

// internal/slack/handler.go 268行目
response.UpdatedInput = map[string]interface{}{} // Empty map for no changes
```

これは仕様に沿っているが、実際には元の input をそのまま返すべきかもしれない。

## 問題の原因（推測）

`Cannot read properties of undefined (reading 'length')` というエラーは JavaScript のエラーであり、Claude Code 側で approval_prompt のレスポンスを処理する際に発生していると考えられる。

考えられる原因：
1. ContentとStructuredContentの両方が設定されているため、Claude Code が期待する形式と異なる
2. permission prompt tool のレスポンスは特殊な処理が必要だが、通常のツールレスポンスと同じように処理されている

## 修正案

### 案1: Content のみを使用する（推奨）

```go
// StructuredContent を使わず、Content のみで JSON 文字列を返す
result := &mcpsdk.CallToolResultFor[PermissionPromptResponse]{
    Content: []mcpsdk.Content{
        &mcpsdk.TextContent{
            Text: string(jsonData),
        },
    },
    // StructuredContent は設定しない
}
```

### 案2: updatedInput に元の input を設定する

```go
// Allow の場合、元の input をそのまま返す
if promptResp.Behavior == "allow" && promptResp.UpdatedInput == nil {
    promptResp.UpdatedInput = params.Arguments.Input
}
```

## 推奨されるログ追加箇所

デバッグを容易にするため、以下の箇所にログを追加することを推奨：

### 1. MCP サーバー側

```go
// internal/mcp/server.go HandleApprovalPrompt メソッド内

// リクエスト受信時
log.Debug().
    Str("component", "mcp_server").
    Str("method", "HandleApprovalPrompt").
    Str("tool_name", params.Arguments.ToolName).
    Interface("input", params.Arguments.Input).
    Msg("Received approval prompt request")

// レスポンス送信前
log.Debug().
    Str("component", "mcp_server").
    Str("method", "HandleApprovalPrompt").
    Str("behavior", promptResp.Behavior).
    Interface("updated_input", promptResp.UpdatedInput).
    Str("json_response", string(jsonData)).
    Msg("Sending approval prompt response")
```

### 2. Slack ハンドラー側

```go
// internal/slack/handler.go handleApprovalAction メソッド内

// 承認/拒否アクション受信時
log.Info().
    Str("component", "slack_handler").
    Str("method", "handleApprovalAction").
    Str("request_id", requestID).
    Bool("approved", approved).
    Str("user", payload.User.Name).
    Msg("Received approval action from Slack")

// MCP サーバーへの送信時
log.Debug().
    Str("component", "slack_handler").
    Str("method", "handleApprovalAction").
    Str("request_id", requestID).
    Interface("response", response).
    Msg("Sending approval response to MCP server")
```

### 3. Claude プロセス管理側

```go
// internal/process/claude.go

// approval_prompt の呼び出しを検知
if toolName == "mcp__cc-slack__approval_prompt" {
    log.Info().
        Str("component", "claude_process").
        Str("tool_name", toolName).
        Interface("tool_input", toolInput).
        Msg("Approval prompt tool called")
}

// approval_prompt のレスポンスを検知
if isApprovalPromptResponse {
    log.Info().
        Str("component", "claude_process").
        Str("response_type", "approval_prompt").
        Interface("response_content", content).
        Msg("Approval prompt response sent to Claude")
}
```

## 作業の進め方と協力体制

### 修正プロセス

1. **Claude Code（きらり）の作業**
   - cc-slack を使わない通常の Claude プロセスで修正を実施
   - 修正完了後、`./scripts/restart` を実行して cc-slack を再起動

2. **ゆうやの作業**
   - Slack から WebFetch などのツールを実行して動作確認
   - Allow/Deny 両方のケースでテスト
   - 結果を Claude Code に報告

3. **フィードバックループ**
   - 問題が解決しない場合は、ログを確認して次の修正を検討
   - 必要に応じてログ出力を追加して再度テスト

## 効果的なログ戦略（ゼロベース設計）

### 1. トレーシング用の一意なIDの付与

```go
// 各リクエストに一意なトレースIDを付与
type TraceContext struct {
    TraceID   string // UUID形式
    SessionID string
    ToolName  string
}

// 全てのログにトレースIDを含める
log.Info().
    Str("trace_id", ctx.TraceID).
    Str("session_id", ctx.SessionID).
    Msg("Processing approval request")
```

### 2. レイヤー間の境界でのログ

```go
// MCP → Claude Code への送信時
log.Info().
    Str("layer", "mcp_to_claude").
    Str("direction", "outbound").
    Str("content_type", "application/json").
    Int("content_length", len(jsonData)).
    Str("raw_content", string(jsonData)).
    Msg("Sending response to Claude Code")

// Slack → MCP への通信時
log.Info().
    Str("layer", "slack_to_mcp").
    Str("direction", "inbound").
    Interface("payload", payload).
    Msg("Received approval action from Slack")
```

### 3. エラーコンテキストの詳細化

```go
// エラー発生時は、その時点の全ての状態を記録
log.Error().
    Err(err).
    Str("trace_id", traceID).
    Interface("request", originalRequest).
    Interface("response", partialResponse).
    Interface("state", currentState).
    Stack().  // スタックトレースを含める
    Msg("Failed to process approval prompt")
```

### 4. JSONパースの前後でのログ

```go
// JSONマーシャリング前
log.Debug().
    Str("stage", "pre_marshal").
    Interface("object", promptResp).
    Msg("Object before JSON marshaling")

// JSONマーシャリング後
log.Debug().
    Str("stage", "post_marshal").
    Str("json", string(jsonData)).
    Int("length", len(jsonData)).
    Msg("JSON after marshaling")

// 受信側でのパース前
log.Debug().
    Str("stage", "pre_parse").
    Str("raw_data", string(rawData)).
    Msg("Raw data before parsing")
```

### 5. タイミングとパフォーマンスログ

```go
// 処理時間の計測
start := time.Now()
defer func() {
    log.Info().
        Str("operation", "approval_prompt").
        Dur("duration", time.Since(start)).
        Msg("Operation completed")
}()
```

### 6. 実際の Claude Code のレスポンス確認

```go
// Claude Code からの実際のレスポンスを詳細に記録
log.Info().
    Str("component", "claude_response_parser").
    Str("raw_response", string(response)).
    Bool("is_json", json.Valid(response)).
    Interface("parsed", parsedResponse).
    Msg("Claude Code response analysis")
```

## 今後の検証手順

1. Content のみを使用する修正を適用
2. トレーシングIDを含むログを追加
3. Allow/Deny 両方のケースでテスト
4. ログを追跡してエラーの正確な発生箇所を特定
5. 必要に応じて追加の修正を実施

## 関連ファイル

- `internal/mcp/server.go`: HandleApprovalPrompt メソッド（205行目、233行目が修正対象）
- `internal/slack/handler.go`: handleApprovalAction メソッド
- `internal/process/claude.go`: Claude プロセスとの通信処理
- `logs/claude-20250727-190411.log`: Deny 時のログ（正常動作）
- `logs/claude-20250727-190507.log`: Allow 時のログ（エラー発生）