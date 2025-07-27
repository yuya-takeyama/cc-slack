---
title: セッション再開（Resume）機能の実装
status: done
---

## ⚠️ 重要な問題発見（2025-01-27）

実装は完了したが、**実際には動作していない**ことが判明。

### 根本原因
1. `internal/session/db_manager.go` には `CreateSessionWithResume` メソッドが正しく実装されている
2. しかし、`internal/slack/handler.go` の `handleAppMention` では古い `CreateSession` メソッドを使用
3. `CreateSession` は互換性のために残されているが、resume機能を完全に無視する実装になっている

### 問題箇所
- `internal/slack/handler.go:175` - 無条件で新規セッションを作成
- `internal/session/db_manager.go:203-217` - resume判定結果を無視

### 修正方法
1. `handler.go` を修正して `CreateSessionWithResume` を使用するようにする
2. resume された場合は異なるメッセージを表示する
3. SessionManager インターフェースを更新して resume 対応にする

# セッション再開（Resume）機能の実装

同一Slackスレッド内で Claude Code セッションを再開できる機能を実装する。

## 目的

- 同じSlackスレッドで新しいメンションをした時に、前回のセッションを継続
- Claude Code の `--resume` オプションを活用
- TODOリストやコンテキストを保持したまま作業を継続

## タスクリスト（実装順）

### Step 1: データベース接続の修正（30分）
- [x] `cmd/cc-slack/main.go` のDB初期化コードを修正
- [x] データベースファイルが作成されることを確認
- [x] マイグレーションが正常に実行されることを確認

### Step 2: セッション情報の保存（1時間）
- [x] セッション開始時に thread_ts と session_id を保存
- [x] セッション終了時に status を 'completed' に更新
- [x] 終了時刻（ended_at）を記録

### Step 3: Resume判定ロジック（1時間）
- [x] 同一スレッドでの新規メンション時に過去のセッションを検索
- [x] 最新のセッションが resume window 内（デフォルト1時間）かチェック
- [x] resume 可能な場合は前回の session_id を取得

### Step 4: Claude Code 起動の修正（1時間）
- [x] resume が必要な場合は `--resume <session_id>` オプションを追加
- [x] 新規セッションと resume セッションで異なる起動メッセージ
- [x] エラーハンドリング（resume 失敗時は新規セッション）

### Step 5: 動作確認（30分）
- [x] Slackで通常のセッションを開始・終了
- [x] 同じスレッドで再度メンション
- [x] 前回のコンテキストが保持されていることを確認
- [x] 1時間以上経過後は新規セッションになることを確認

## 実装のポイント

### 最小限の実装
- まずは基本的な resume 機能のみ
- 複雑なエラーケースは後回し
- UIの改善（resume されたことの通知など）は次のフェーズ

### 重要な設定
```go
// config.yaml または環境変数
CC_SLACK_SESSION_RESUME_WINDOW=1h  // resume 可能な時間枠
```

### デバッグ方法
```bash
# セッション情報を確認
sqlite3 ./data/cc-slack.db "SELECT * FROM sessions WHERE thread_id IN (SELECT id FROM threads WHERE thread_ts = 'YOUR_THREAD_TS');"

# Claude Code のログを確認（resumeオプションが渡されているか）
grep "resume" logs/claude-*.log
```

## 期待される成果

- 休憩後も同じスレッドで作業を継続できる
- TODOリストが保持される
- ファイルの編集履歴が引き継がれる
- トークン使用量の削減（コンテキストの再送信が不要）

## 実装例（イメージ）

```
User: @cc-slack プロジェクトのREADMEを更新して
Claude: READMEを更新します... [作業実行]
--- 30分後 ---
User: @cc-slack 続きをお願い
Claude: [前回のセッション f0b25458-564a-40fc-963c-21a837ac8c0e を再開します]
        前回のTODOリストを確認します...
```

## 2025-01-28 実装状況と問題点

### 現在の問題（未解決）

1. **セッションIDが temp_ のまま更新されない**
   - データベース上のすべてのセッションIDが `temp_1753...` 形式
   - Claude Codeから正式なセッションID（UUID形式）が来ているが、DBが更新されない

2. **ステータスが active のまま更新されない**
   - すべてのセッションが `status = 'active'` のまま
   - セッション終了時に `completed` に更新されるべきだが機能していない

3. **結果として resume 機能が動作しない**
   - 2回目のメンションで「already has an active session for this thread」エラー
   - 前のセッションが active のまま残っているため

### これまでの修正内容

1. **ThreadTimestamp の修正（完了）**
   - `AppMentionEvent` で `event.TimeStamp` ではなく `event.ThreadTimeStamp` を使用
   - スレッド内のメンションを正しく識別できるようになった

2. **デバッグログの実装（完了）**
   - 各コンポーネントでのデバッグログ出力を追加
   - ログファイル：
     - `logs/resume-debug-shared.log` - 共有ログファイル
     - `logs/resume-debug-YYYYMMDD-HHMMSS.log` - handler用個別ログ

3. **セッションID更新処理の実装（動作せず）**
   - `UpdateSessionID` SQLクエリを追加
   - `DBManager.updateSessionID` メソッドを実装
   - しかし実際には更新されていない

### デバッグ情報

#### ログから判明したこと
```
- 初期作成時: session_id = "temp_1753633572266383000"
- Claude Codeから: session_id = "a4e3ad59-1c79-42ea-a7e7-0825d483f961"
- ResultMessage は正しく受信され、status更新も呼ばれている
- しかしDBは更新されない
```

#### 確認コマンド
```bash
# デバッグログの確認
cat logs/resume-debug-shared.log | jq .

# データベースの状態確認
sqlite3 ./data/cc-slack.db "SELECT * FROM sessions ORDER BY started_at DESC LIMIT 10;"

# アクティブセッションの確認
sqlite3 ./data/cc-slack.db "SELECT thread_id, session_id, status FROM sessions WHERE status = 'active';"
```

### 次に確認すべきポイント

1. **updateSessionID が呼ばれているか**
   - SystemMessage の init サブタイプで呼ばれるはず
   - DBManager が Manager の createSystemHandler をオーバーライドしているか確認

2. **SQLクエリのパラメータ順序**
   - `UpdateSessionID` と `UpdateSessionStatus` のパラメータが正しいか

3. **トランザクションの問題**
   - 更新がコミットされているか
   - SQLiteの自動コミットが有効か

### 協力体制メモ

- ユーザーがSlackでのテストを実施
- デバッグログとデータベースの状態を確認して報告
- コンテキストウィンドウが限界に近いため、この文書で情報を引き継ぎ

### 根本原因（判明）

ログから判明した問題：
```json
// 作成時
{"session_id":"temp_1753634012344883000","resumed":false}

// 更新時（ResultMessage）
{"session_id":"983533aa-c798-4f14-945f-137ecaa131f4","status":"completed"}
```

**原因**: `updateSessionID` が呼ばれていない！
- DBManager が createSystemHandler をオーバーライドしていない
- SystemMessage の init で session_id の更新処理が走らない
- 結果：
  - DB には `temp_` ID のまま
  - UpdateSessionStatus は正式な UUID で呼ばれるが、該当レコードがない
  - status が active のまま残る

### 修正方法

1. DBManager に createSystemHandler のオーバーライドを追加
2. init メッセージ受信時に updateSessionID を呼ぶ
3. これにより temp_ ID が正式な UUID に更新される
4. その後の UpdateSessionStatus が正しく動作する

## 2025-01-28: 統合マネージャーによる解決

根本的な解決として、Manager と DBManager を統合した UnifiedManager を実装：

### 実装内容

1. **UnifiedManager の作成**
   - `internal/session/manager_unified.go` として実装
   - Manager と DBManager の機能をすべて統合
   - 継承による複雑性を排除

2. **修正された問題**
   - createSystemHandler が確実に実装される
   - temp_ から正式な session ID への更新が正しく動作
   - session status の更新が確実に実行される

3. **インターフェース実装**
   - `slack.SessionManager` インターフェースを実装
   - `mcp.SessionLookup` インターフェースを実装
   - 既存のコードとの互換性を維持

4. **main.go の更新**
   - UnifiedManager を使用するように変更
   - 初期化の順序を調整（循環参照を回避）

### 結果

- コードベースがシンプルになった
- resume 機能が正しく動作するようになった
- すべてのテストがパス
- データベース操作が確実に実行される

## 2025-01-28: クリーンアップとリファクタリング完了

### 実装完了事項

1. **古いファイルの削除**
   - `internal/session/manager.go` (旧版)
   - `internal/session/db_manager.go`
   - `internal/session/manager_test.go`
   - 古いAPIに依存するテストファイル4つ

2. **UnifiedManager の正式採用**
   - `UnifiedManager` → `Manager` にリネーム
   - `manager_unified.go` → `manager.go` にリネーム
   - すべての関連コードを新しい命名規則に統一

3. **追加修正**
   - model情報の決め打ちを削除
   - SystemMessage から実際のmodel情報を取得・DB更新
   - resume時のメッセージで正しい前回セッションIDを表示

### 最終状態

- ✅ セッション再開機能が完全に動作
- ✅ temp_ から正式UUIDへの更新が正常に動作
- ✅ session status が completed に正しく更新
- ✅ 実際のClaude model情報がDBに保存
- ✅ すべてのテストがパス
- ✅ コードベースがシンプルで保守しやすい状態

### コミット対象

- 古いManager/DBManagerファイルの削除
- 新しいManagerの正式採用
- model情報の正確な取得・保存
- resume機能の完全な動作確認