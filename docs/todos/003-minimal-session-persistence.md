---
title: セッション再開（Resume）機能の実装
status: done
---

# セッション再開（Resume）機能の実装

同一Slackスレッド内で Claude Code セッションを再開できる機能を実装する。

## 目的

- 同じSlackスレッドで新しいメンションをした時に、前回のセッションを継続
- Claude Code の `--resume` オプションを活用
- TODOリストやコンテキストを保持したまま作業を継続

## タスクリスト（実装順）

### Step 1: データベース接続の修正（30分）
- [ ] `cmd/cc-slack/main.go` のDB初期化コードを修正
- [ ] データベースファイルが作成されることを確認
- [ ] マイグレーションが正常に実行されることを確認

### Step 2: セッション情報の保存（1時間）
- [ ] セッション開始時に thread_ts と session_id を保存
- [ ] セッション終了時に status を 'completed' に更新
- [ ] 終了時刻（ended_at）を記録

### Step 3: Resume判定ロジック（1時間）
- [ ] 同一スレッドでの新規メンション時に過去のセッションを検索
- [ ] 最新のセッションが resume window 内（デフォルト1時間）かチェック
- [ ] resume 可能な場合は前回の session_id を取得

### Step 4: Claude Code 起動の修正（1時間）
- [ ] resume が必要な場合は `--resume <session_id>` オプションを追加
- [ ] 新規セッションと resume セッションで異なる起動メッセージ
- [ ] エラーハンドリング（resume 失敗時は新規セッション）

### Step 5: 動作確認（30分）
- [ ] Slackで通常のセッションを開始・終了
- [ ] 同じスレッドで再度メンション
- [ ] 前回のコンテキストが保持されていることを確認
- [ ] 1時間以上経過後は新規セッションになることを確認

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