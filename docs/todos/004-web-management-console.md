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

### Step 1: バックエンドAPI実装（2時間）

#### 1.1 基本設定
- [ ] `/web/*` パスのルーティング追加
- [ ] シンプルなJSONレスポンス形式の統一
- [ ] config.goに SLACK_WORKSPACE_SUBDOMAIN 定数を追加（例: "yuyat"）
  - [ ] TODO コメント追加：「将来的に複数workspace対応時はDBに移行」

#### 1.2 最小限のAPI
- [ ] `GET /web/api/threads` - スレッド一覧（workspace_subdomain、channel_id、thread_ts、最新セッション情報）
- [ ] `GET /web/api/sessions` - セッション一覧（session_id、thread_ts、status、started_at、ended_at）

### Step 2: フロントエンドUI実装（3時間）

#### 2.1 Viteプロジェクトセットアップ
- [ ] webディレクトリ作成とpackage.json初期化
- [ ] React 19 + Vite 7 + Tailwind 4 のインストール
- [ ] vite.config.js作成（base: '/web/'設定）
- [ ] tailwind.config.jsとPostCSS設定
- [ ] ディレクトリ構造の整備（src/, public/, styles/）

#### 2.2 Reactコンポーネント実装
- [ ] main.jsx（エントリーポイント）
- [ ] App.jsx（メインコンテナ）
- [ ] ThreadList コンポーネント（スレッド一覧）
  - [ ] Slackスレッドへの直接リンク（https://{workspace_subdomain}.slack.com/archives/{channel_id}/p{thread_ts}）
  - [ ] 各スレッドのセッション数と最新ステータス表示
- [ ] SessionList コンポーネント（セッション一覧）
  - [ ] シンプルなテーブル表示
  - [ ] 基本情報のみ（session_id、thread_ts、status、開始/終了時刻）

#### 2.3 ビルドとGo統合
- [ ] npm run buildでdist/生成確認
- [ ] Go側でembed.FSを使った静的ファイル配信実装
- [ ] /web/パスでSPA、/web/api/でAPIの分離確認

### Step 3: デプロイと動作確認（30分）

- [ ] 開発サーバー（npm run dev）での動作確認
- [ ] ビルド後のGo統合での動作確認
- [ ] ハードコードされたworkspace_subdomainでSlackリンクが正しく動作することを確認

## 実装例

### Go定数設定（config.go）
```go
// internal/config/config.go

const (
    // SlackワークスペースのSubdomain
    // TODO: 将来的に複数workspace対応時はDBに移行
    SLACK_WORKSPACE_SUBDOMAIN = "yuyat"
)
```

### package.json
```json
{
  "name": "cc-slack-web",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.0.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^9.1.0",
    "tailwindcss": "^4.1.0",
    "vite": "^7.0.2"
  }
}
```

### vite.config.js
```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  base: '/web/',
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
```

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
  - React 19（最新安定版）
  - Vite 7（高速ビルドツール）
  - Tailwind CSS 4（ユーティリティファーストCSS）
  - PostCSS（CSS処理）
- **データ取得**: Fetch API
- **ビルド・配信**: embed.FSでGoバイナリに静的ファイルを埋め込み

## ディレクトリ構造

```
cc-slack/
├── internal/
│   └── web/
│       ├── handler.go      # HTTPハンドラー（embed.FS使用）
│       └── api.go          # REST APIエンドポイント
└── web/                    # フロントエンドプロジェクト
    ├── package.json
    ├── vite.config.js
    ├── tailwind.config.js
    ├── index.html
    ├── src/
    │   ├── main.jsx
    │   ├── App.jsx
    │   └── components/
    │       ├── ThreadList.jsx
    │       └── SessionList.jsx
    ├── styles/
    │   └── index.css
    └── dist/               # ビルド成果物（gitignore）
```

## 期待される成果

- スレッド一覧から Slack の元スレッドにアクセスできる
- セッション履歴を確認できる
- 最短で動作する管理画面が手に入る