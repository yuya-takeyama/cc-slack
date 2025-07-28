---
title: Webマネジメントコンソール MVP - スレッド・セッション一覧
status: done
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

### Phase 1: スレッド一覧画面の実装（2.5時間）

#### 1.1 最小限のバックエンド実装
- [x] `/web/*` パスのルーティング追加
- [x] config.goに SLACK_WORKSPACE_SUBDOMAIN 定数を追加（例: "yuyat"）
  - [x] TODO コメント追加：「将来的に複数workspace対応時はDBに移行」
- [x] `GET /web/api/threads` APIのみ実装（workspace_subdomain、channel_id、thread_ts、最新セッション情報）
- [x] Go側でembed.FSを使った静的ファイル配信実装

#### 1.2 最小限のフロントエンド実装
- [x] webディレクトリ作成とpackage.json初期化
- [x] pnpmで React 18 + Vite 6 + Tailwind 3 + TypeScript + Biome + Vitest のインストール（最新安定版を使用）
- [x] vite.config.js作成（base: '/web/'設定）
- [x] tailwind.config.jsとPostCSS設定
- [x] main.jsx + App.jsx + ThreadList.jsxのみ実装
  - [x] Slackスレッドへの直接リンク（https://{workspace_subdomain}.slack.com/archives/{channel_id}/p{thread_ts}）
  - [x] 各スレッドのセッション数と最新ステータス表示（セッション数にバグあり）

#### 1.3 スレッド一覧の動作確認
- [x] pnpm buildでdist/生成（※Goビルドで対応）
- [x] cc-slackを再起動してスレッド一覧が表示されることを確認
- [x] Slackへのリンクが正しく動作することを確認

### Phase 2: セッション一覧画面の実装（2時間）

#### 2.1 セッション一覧API実装
- [x] `GET /web/api/sessions` API実装（session_id、thread_ts、status、started_at、ended_at）
- [x] シンプルなJSONレスポンス形式の統一

#### 2.2 セッション一覧UI実装
- [x] SessionList.jsx コンポーネント実装
  - [x] シンプルなテーブル表示
  - [x] 基本情報のみ（session_id、thread_ts、status、開始/終了時刻）
- [x] App.jsxにセッション一覧を追加

#### 2.3 セッション一覧の動作確認
- [x] pnpm buildでdist/再生成（※Goビルドで対応）
- [x] cc-slackを再起動してセッション一覧が表示されることを確認

### Phase 3: 最終確認と調整（1時間）

- [ ] 開発サーバー（pnpm dev）での全体動作確認（※Node.js環境が必要）
- [x] ビルド後のGo統合での全体動作確認
- [ ] UIの微調整（必要に応じて）→ 005-web-console-improvements.mdで対応予定

### Phase 3.5: ページ分割とルーティング実装（3時間）

#### 3.5.1 React Routerの導入とルーティング設定
- [ ] React Router v7をpnpmでインストール
- [ ] 以下のルーティング構成を実装:
  - [ ] `/`: ThreadListページ（スレッド一覧）
  - [ ] `/sessions`: SessionListページ（全セッション一覧）
  - [ ] `/threads/:thread_id/sessions`: ThreadSessionsページ（特定スレッドのセッション一覧）

#### 3.5.2 APIエンドポイントの追加
- [ ] `GET /web/api/threads/:thread_id/sessions` エンドポイント実装
  - [ ] thread_idに紐づくセッションのみを返す
  - [ ] thread情報も含めて返す

#### 3.5.3 UIコンポーネントの更新
- [ ] ナビゲーションメニューコンポーネントの作成
- [ ] ThreadSessionsコンポーネントの新規作成
- [ ] ThreadListからThreadSessionsへのリンク追加
- [ ] SPAのルーティングに対応するためのバックエンド調整（全てのweb/*パスでindex.htmlを返す）

### Phase 4: テストと品質保証（後日実装）

- [ ] Reactコンポーネントのユニットテスト追加
- [ ] pnpm all で全てのチェックが通ることを確認
- [ ] CLAUDE.md に「フロントエンド変更時は pnpm all を必ず実行」ルールを追加

### Phase 5: CI/CD パイプラインの設定（1時間）

#### 5.1 GitHub Actions ワークフローの更新
- [ ] `.github/workflows/test.yaml` に TypeScript ビルドステップを追加
- [ ] pnpm セットアップアクションの導入（pnpm/action-setup）
- [ ] Node.js 環境のセットアップ（actions/setup-node）
- [ ] キャッシュ戦略の実装（pnpm store と node_modules）

#### 5.2 ビルドとテストの統合
- [ ] `web/` ディレクトリでの依存関係インストール（pnpm install）
- [ ] TypeScript 型チェック（pnpm typecheck）
- [ ] Biome による lint チェック（pnpm lint）
- [ ] フロントエンドビルド（pnpm build）
- [ ] ビルド成果物の存在確認（dist/index.html の存在チェック）

#### 5.3 Go と TypeScript の統合テスト
- [ ] フロントエンドビルド後に Go のテストを実行
- [ ] embed.FS が正しく dist ディレクトリを読み込めることを確認
- [ ] 両方のテストが成功した場合のみ CI をパスさせる

#### 5.4 最適化と改善
- [ ] ビルド時間の最適化（並列実行の検討）
- [ ] エラーメッセージの改善
- [ ] ビルドアーティファクトの管理

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
    "preview": "vite preview",
    "test": "vitest run --coverage",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit",
    "lint": "biome check --unsafe .",
    "lint:fix": "biome check --write --unsafe .",
    "format": "biome format --write .",
    "format:check": "biome format .",
    "all": "pnpm typecheck && pnpm lint && pnpm format && pnpm test"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@biomejs/biome": "^1.10.0",
    "@types/react": "^19.0.2",
    "@types/react-dom": "^19.0.2",
    "@vitejs/plugin-react": "^4.0.0",
    "@vitest/coverage-v8": "^2.0.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^9.1.0",
    "tailwindcss": "^4.1.0",
    "typescript": "^5.0.0",
    "vite": "^7.0.2",
    "vitest": "^2.0.0"
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
  - TypeScript 5（型安全性）
  - Vite 7（高速ビルドツール）
  - Tailwind CSS 4（ユーティリティファーストCSS）
  - PostCSS（CSS処理）
- **開発ツール**:
  - pnpm（高速パッケージマネージャー）
  - Biome（高速Linter/Formatter）
  - Vitest（ユニットテスト）
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
    ├── pnpm-lock.yaml
    ├── tsconfig.json
    ├── biome.json
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

## 実装完了サマリー

2025-01-28: Web管理コンソールのMVP実装が完了しました。

### 実装内容
- ✅ バックエンドAPI（/web/api/threads, /web/api/sessions）
- ✅ フロントエンド（React + Vite + Tailwind）
- ✅ embed.FSによる静的ファイル配信
- ✅ Slackへの直接リンク機能

### 既知の問題
- セッション数が0と表示される（バグ）
- thread_tsが読みにくい形式で表示される
- チャンネルIDがそのまま表示される

これらの問題は 005-web-console-improvements.md で対応予定です。