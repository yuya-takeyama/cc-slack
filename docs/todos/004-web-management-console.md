---
title: Webマネジメントコンソール MVP - スレッド・セッション一覧
status: draft
---

# Webマネジメントコンソール MVP - スレッド・セッション一覧

cc-slack のスレッドとセッション情報を一覧表示し、Slackの元スレッドにアクセスできるシンプルなWeb UIを実装する。

## 目的

- スレッド一覧から Slack の元スレッドを開けるようにする
- セッション一覧を確認できるようにする
- 最短距離で価値を届ける

## 前提条件

- データベース統合が完了していること（002-database-integration.md）
- セッション情報がDBに保存されていること

## タスクリスト

### Step 1: データベーススキーマ拡張（30分）

- [ ] threads テーブルに workspace_subdomain カラムを追加（yuyat.slack.com の yuyat 部分）
- [ ] migrationファイルの作成と適用

### Step 2: バックエンドAPI実装（2時間）

#### 2.1 基本設定
- [ ] `/web/*` パスのルーティング追加
- [ ] シンプルなJSONレスポンス形式の統一

#### 2.2 最小限のAPI
- [ ] `GET /web/api/threads` - スレッド一覧（workspace_subdomain、channel_id、thread_ts、最新セッション情報）
- [ ] `GET /web/api/sessions` - セッション一覧（session_id、thread_ts、status、started_at、ended_at）

### Step 3: フロントエンドUI実装（2時間）

#### 3.1 基本HTML
- [ ] 静的HTMLファイルの配信設定
- [ ] 1ファイルのシンプルなHTML（index.html）
- [ ] Tailwind CSS（CDN版）の導入

#### 3.2 スレッド一覧画面
- [ ] スレッド一覧の表示
- [ ] Slackスレッドへの直接リンク（https://{workspace_subdomain}.slack.com/archives/{channel_id}/p{thread_ts}）
- [ ] 各スレッドのセッション数と最新ステータス表示

#### 3.3 セッション一覧画面
- [ ] セッション一覧の表示（シンプルなテーブル）
- [ ] 基本情報のみ（session_id、thread_ts、status、開始/終了時刻）

### Step 4: デプロイと動作確認（30分）

- [ ] ローカルでの動作確認
- [ ] Slack workspace情報の自動取得（Slack APIから）または環境変数設定

## 実装例

### APIレスポンス例
```json
// GET /web/api/threads
{
  "threads": [
    {
      "thread_ts": "1234567890.123456",
      "channel_id": "C1234567890",
      "workspace_subdomain": "yuyat",
      "session_count": 3,
      "latest_session_status": "completed"
    }
  ]
}

// GET /web/api/sessions
{
  "sessions": [
    {
      "session_id": "f0b25458-564a-40fc-963c-21a837ac8c0e",
      "thread_ts": "1234567890.123456",
      "status": "completed",
      "started_at": "2025-01-27T10:00:00Z",
      "ended_at": "2025-01-27T10:30:00Z"
    }
  ]
}
```

### UI モックアップ

#### シンプルなスレッド・セッション一覧
```
┌─────────────────────────────────────────────┐
│ cc-slack Sessions                           │
├─────────────────────────────────────────────┤
│ Threads                                     │
│ ┌─────────────────────────────────────────┐│
│ │Thread: 1234567890.123456                ││
│ │Channel: C1234567890                     ││
│ │Sessions: 3 | Latest: completed          ││
│ │[Open in Slack ↗]                        ││
│ └─────────────────────────────────────────┘│
│                                             │
│ Sessions                                    │
│ ┌─────────────────────────────────────────┐│
│ │ID: f0b25458-... | Thread: 123456789... ││
│ │Status: completed                        ││
│ │2025-01-27 10:00 - 10:30                ││
│ └─────────────────────────────────────────┘│
└─────────────────────────────────────────────┘
```

## 技術スタック

- **バックエンド**: 既存のGo HTTPサーバーを拡張
- **フロントエンド**: 
  - Vanilla JavaScript（シンプルさ重視）
  - Tailwind CSS（CDN版）
- **データ取得**: Fetch API

## ディレクトリ構造

```
internal/
  web/
    handler.go      # HTTPハンドラー
    api.go          # REST APIエンドポイント
    static/         # 静的ファイル（HTML/CSS/JS）
      index.html    # メインページ
```

## 期待される成果

- スレッド一覧から Slack の元スレッドにアクセスできる
- セッション履歴を確認できる
- 最短で動作する管理画面が手に入る