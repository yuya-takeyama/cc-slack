---
title: マルチリポジトリ・マルチWorktreeサポートの実装
status: draft
---

# 009: マルチリポジトリ・マルチWorktreeサポートの実装

## 概要

cc-slackに以下の2つの重要な機能を実装し、並列Claude Code操作を可能にする：

1. **マルチリポジトリ対応**: 複数のリポジトリに対してClaude Codeを同時実行可能にする
2. **マルチWorktree対応**: 単一リポジトリ内で複数の作業ディレクトリ（Git worktree）を使用して並列作業を可能にする

## 背景と解決したい問題

### 現状の制限
- 単一リポジトリ、単一作業ディレクトリのみサポート
- ファイルシステムがボトルネックとなり、ファイル変更を伴う作業は同時に1つまで
- 調査系タスクは並列実行可能だが、ファイル変更が混在するとbranchの状態が不安定になる

### ユースケース
- 複数のプロジェクトを同時に管理したい
- 単一プロジェクト内で複数の機能開発を並列で進めたい
- コードレビューと新機能開発を同時に行いたい

## 実装方針

### マルチリポジトリ対応の設計

#### 基本設計
- 設定ファイル（config.yaml）でリポジトリのリストを管理
- リポジトリごとに以下を設定可能：
  - パス
  - 表示名
  - Slack設定のオーバーライド（チャンネル、ユーザー名、アイコン等）
  - デフォルトブランチ

#### リポジトリ選択: AIルーター方式（Option D採用）
- チャンネルに複数リポジトリが設定されている場合のみルーターを起動
- 単一リポジトリの場合は直接実行（パフォーマンス最適化）
- ルーター使用時：
  - 別のClaude Codeプロセスがルーターとして機能
  - Claude Code SDKのカスタムシステムプロンプト機能を活用
  - Sonnetモデル（`--model claude-3-5-sonnet-latest`）使用でコスト効率化
  - メッセージ内容からリポジトリを推定し、適切なリポジトリに振り分け

### マルチWorktree対応の設計

#### 基本設計
- Git worktree機能を活用
- 1スレッド = 1 worktreeの原則
- worktreeは専用ディレクトリ（例: `.cc-slack-worktrees/`）に作成

#### ライフサイクル管理
- **作成**: 新規スレッド開始時に自動作成
- **ベースブランチ**: デフォルトブランチまたは指定されたブランチ
- **削除**: セッション終了後、設定可能な期間（例: 24時間）後に自動削除
- **再利用**: resume機能使用時は既存のworktreeを再利用

## データベーススキーマ

### 新規テーブル: repositories
```sql
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    default_branch TEXT DEFAULT 'main',
    slack_channel_id TEXT,
    slack_username TEXT,
    slack_icon_emoji TEXT,
    slack_icon_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 新規テーブル: worktrees
```sql
CREATE TABLE worktrees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    current_branch TEXT,
    status TEXT NOT NULL DEFAULT 'active', -- active, archived, deleted
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (repository_id) REFERENCES repositories(id),
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    UNIQUE(thread_id)
);
```

### 既存テーブルの変更
- threads: `repository_id INTEGER` カラムを追加
- threads: `working_directory` を `worktree_id` に変更（または両方保持）

## 実装タスク

### Phase 1: マルチリポジトリ基盤
- [ ] repositoriesテーブルの作成（migration）
- [ ] Repository管理機能の実装
  - [ ] Repository構造体とインターフェース定義
  - [ ] RepositoryManagerの実装
- [ ] 設定ファイルのスキーマ拡張
  - [ ] Viper設定にリポジトリリストを追加
  - [ ] YAML設定の検証ロジック
- [ ] AIリポジトリルーターの実装
  - [ ] ルーター用Claude Codeプロセスの管理
  - [ ] システムプロンプトの設計（リポジトリ設定情報を含む）
  - [ ] JSON Schemaによる応答フォーマット定義
  - [ ] Sonnetモデルを使用したルーティングロジック
  - [ ] ルーティング結果のキャッシュ機構

### Phase 2: マルチWorktree基盤
- [ ] worktreesテーブルの作成（migration）
- [ ] Git worktree操作のラッパー実装
  - [ ] worktree作成関数
  - [ ] worktree削除関数
  - [ ] worktree状態確認関数
- [ ] WorktreeManagerの実装
  - [ ] worktreeディレクトリの管理
  - [ ] 並行性制御（mutex）
- [ ] セッション作成時のworktree自動作成

### Phase 3: 統合とクリーンアップ
- [ ] SessionManagerの更新
  - [ ] リポジトリとworktreeを考慮したセッション作成
  - [ ] working directory解決ロジックの更新
- [ ] 定期クリーンアップジョブの実装
  - [ ] 古いworktreeの削除
  - [ ] ディスク容量監視
  - [ ] goroutineによる定期実行
- [ ] Web管理画面の更新
  - [ ] リポジトリ一覧表示
  - [ ] worktree状態表示
  - [ ] APIエンドポイントの追加

### Phase 4: 高度な機能とテスト
- [ ] リポジトリ選択の追加オプション実装
  - [ ] Option B: Slackインタラクティブメッセージ
  - [ ] Option C: メンション時のパラメータ指定
- [ ] worktree再利用の最適化
- [ ] 並列実行数の制限機能
- [ ] リソース使用量のモニタリング
- [ ] 包括的なテストの実装
  - [ ] ユニットテスト
  - [ ] 統合テスト

## 検討事項

### セキュリティ
- リポジトリアクセス権限の管理方法
- Slackユーザーとリポジトリのマッピング

### パフォーマンス
- worktree作成のオーバーヘッド対策
- ディスク容量の監視と制限
- 同時実行数の適切な上限設定

### UX
- 現在のリポジトリ/worktreeの可視化方法
- エラー時のフィードバック方法
- リポジトリ切り替えの直感的な方法

### 運用
- バックアップ戦略
- worktreeのライフサイクルポリシー
- ログとモニタリング

## 成功指標

- 複数リポジトリでの同時作業が可能
- 単一リポジトリでの並列作業数が3倍以上に向上
- worktree作成/削除の自動化によるメンテナンスフリー
- ユーザーからの「ファイル競合エラー」報告がゼロに

## 参考資料

- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)
- [GitHub: Using Git Worktree](https://github.blog/2015-07-29-git-2-5-including-multiple-worktrees-and-triangular-workflows/)
- [Claude Code SDK - Custom System Prompts](https://docs.anthropic.com/en/docs/claude-code/sdk#custom-system-prompts)
- [Claude Code CLI Reference](https://docs.anthropic.com/en/docs/claude-code/cli-reference)