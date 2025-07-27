# cc-slack リファクタリングプラン

## 概要

このドキュメントは、cc-slackプロジェクトのリファクタリング計画をまとめたものです。
主な目的は以下の3点です：

1. **重複ロジックの共通化** - 似たような処理を統合し、保守性を向上
2. **凝集性の向上** - 関連するロジックを同一ファイルに集約し、Claude Codeでの編集を容易に
3. **純粋関数の抽出** - テスト可能なロジックを抽出し、ユニットテストを追加

## フェーズ1: ツール表示情報の統一 ✅ **実装済み**

### 現状の問題
- ツール名と絵文字のマッピングが複数ファイルに分散
  - `internal/slack/handler.go`: `toolDisplayMap`
  - `internal/session/manager.go`: `getToolEmoji()`

### 実装計画
1. ✅ `internal/tools/display.go` を新規作成
2. ✅ ツール情報を一元管理する構造体を定義：
   ```go
   type ToolInfo struct {
       Name      string
       Emoji     string // Unicode emoji
       SlackIcon string // Slack emoji code
   }
   ```
3. ✅ 全ツール情報を定数として定義
4. ✅ 既存コードを新しい構造体を使用するよう修正

### 影響範囲
- `internal/slack/handler.go`
- `internal/session/manager.go`

## フェーズ2: 純粋関数の抽出とテスト追加

### 2.1 メッセージフォーマット関数の抽出 ✅ **実装済み**

#### 実装計画
1. ✅ `internal/messages/format.go` を新規作成
2. ✅ 以下の関数を抽出：
   - `FormatSessionStartMessage(sessionID, cwd, model string) string`
   - `FormatSessionCompleteMessage(duration time.Duration, turns int, cost float64, inputTokens, outputTokens int) string`
   - `FormatTimeoutMessage(idleMinutes int, sessionID string) string`
   - `FormatBashToolMessage(command string) string`
   - `FormatReadToolMessage(filePath string, offset, limit int) string`
   - `FormatGrepToolMessage(pattern, path string) string`
   - `FormatEditToolMessage(filePath string) string`
   - `FormatWriteToolMessage(filePath string) string`
   - `FormatLSToolMessage(path string) string`
   - `FormatGlobToolMessage(pattern string) string`
   - `FormatTaskToolMessage(description, prompt string) string`
   - `FormatWebFetchToolMessage(url, prompt string) string`
   - `FormatDuration(d time.Duration) string`

3. ✅ 各関数に対するユニットテストを `internal/messages/format_test.go` に追加

#### 影響範囲
- `internal/session/manager.go` の巨大なswitch文をリファクタリング

### 2.2 環境変数ヘルパーの移動

#### 実装計画
1. `internal/config/env.go` を新規作成
2. `cmd/cc-slack/main.go` から以下を移動：
   - `GetEnv(key, defaultValue string) string`
   - `GetDurationEnv(key string, defaultValue time.Duration) time.Duration`
3. ユニットテストを追加

#### 影響範囲
- `cmd/cc-slack/main.go`

### 2.3 承認ペイロード生成の抽出

#### 実装計画
1. `internal/slack/approval.go` を新規作成
2. 承認関連のロジックを分離：
   - `BuildApprovalPayload(channelID, threadTS, message, requestID, action string) map[string]interface{}`
   - `GenerateDebugCurlCommands(approvePayload, denyPayload map[string]interface{}) string`
3. ユニットテストを追加

#### 影響範囲
- `internal/slack/handler.go`

## フェーズ3: 凝集性の向上

### 3.1 Slackハンドラーの責務分割

#### 現状の問題
- `Handler` structが多くの責務を持ちすぎている
  - イベント処理
  - メッセージ投稿
  - 承認処理
  - ボタンインタラクション

#### 実装計画
1. 責務ごとに分離：
   - `internal/slack/event_handler.go`: イベント処理専用
   - `internal/slack/message_poster.go`: メッセージ投稿専用
   - `internal/slack/approval_handler.go`: 承認処理専用

2. インターフェースで抽象化：
   ```go
   type EventHandler interface {
       HandleMessage(event *slackevents.MessageEvent) error
   }
   
   type MessagePoster interface {
       PostToThread(channelID, threadTS, text string) error
       PostRichTextToThread(channelID, threadTS string, blocks []slack.Block) error
   }
   
   type ApprovalHandler interface {
       SendApprovalRequest(channelID, threadTS, message, requestID string) error
       HandleApprovalAction(payload slack.InteractionCallback) error
   }
   ```

### 3.2 循環依存の解消

#### 現状の問題
- 各コンポーネントが相互に依存している

#### 実装計画
1. 依存関係を整理：
   - `main.go` → 各コンポーネントのインターフェース
   - 各コンポーネント → 必要なインターフェースのみ

2. 依存性注入パターンを活用：
   ```go
   type Dependencies struct {
       SlackClient   *slack.Client
       SessionStore  SessionStore
       MessagePoster MessagePoster
       // etc...
   }
   ```

## フェーズ4: 共通ロジックの抽出

### 4.1 Slackメッセージオプションの共通化

#### 実装計画
1. `internal/slack/options.go` を作成
2. 共通のオプション設定関数を定義：
   ```go
   func WithThreadTimestamp(ts string) slack.MsgOption
   func WithUnfurlSettings(links, media bool) slack.MsgOption
   ```

#### 影響範囲
- `internal/slack/handler.go` の各Post系メソッド

## 実装順序

1. ✅ **フェーズ2.1**: メッセージフォーマット関数の抽出（最も影響が少ない）
2. ✅ **フェーズ1**: ツール表示情報の統一（依存関係が単純）
3. **フェーズ2.2, 2.3**: その他の純粋関数の抽出
4. **フェーズ4**: 共通ロジックの抽出
5. **フェーズ3**: 凝集性の向上（最も影響が大きい）

## テスト戦略

### ユニットテスト
- 全ての純粋関数に対してテーブルドリブンテストを作成
- カバレッジ目標: 80%以上

### 既存テストの更新
- `internal/process/claude_test.go`
- 新しい関数構造に合わせて更新

## リスクと対策

### リスク
1. 大規模な構造変更による既存機能への影響
2. 循環依存の解消時の複雑性

### 対策
1. フェーズごとに小さくリファクタリング
2. 各フェーズ後に動作確認
3. テストを先に書いてから実装（TDD）

## 期待される効果

1. **保守性の向上**: 重複コードの削減により、変更箇所が明確に
2. **テスタビリティの向上**: 純粋関数の増加により、ユニットテストが容易に
3. **Claude Codeでの編集効率向上**: 関連ロジックが同一ファイルに集約
4. **新機能追加の容易化**: 明確な責務分離により、拡張ポイントが明確に

## 次のステップ

1. このプランのレビューと承認
2. フェーズ1から順次実装開始
3. 各フェーズ完了後のコードレビュー
4. 全フェーズ完了後の総合テスト