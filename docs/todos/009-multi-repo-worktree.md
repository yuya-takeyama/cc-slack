---
title: マルチリポジトリ・マルチWorktreeサポートの実装
status: in_progress
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

### ~~新規テーブル: repositories~~ (削除済み)
設定ファイルベースの管理に移行しました。リポジトリ情報は config.yaml で管理されます。

```yaml
repositories:
  - name: cc-slack
    path: /Users/yuya/src/github.com/yuya-takeyama/cc-slack
    default_branch: main
    channels:
      - C097NAK7Q8L
    slack_override:
      username: Custom Bot
      icon_emoji: :robot_face:
```

### 新規テーブル: worktrees (更新済み)
```sql
CREATE TABLE worktrees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_path TEXT NOT NULL,  -- repository_idから変更
    repository_name TEXT NOT NULL,  -- 新規追加
    thread_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    current_branch TEXT,
    status TEXT NOT NULL DEFAULT 'active', -- active, archived, deleted
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    UNIQUE(thread_id)
);
```

### 既存テーブルの変更
- threads: `working_directory` を保持（worktreeのパスを格納）

## 実装タスク

### Phase 1: マルチリポジトリ基盤
- [x] ~~repositoriesテーブルの作成（migration）~~ → 設定ファイルベースに変更
- [x] Repository管理機能の実装
  - [x] ~~Repository構造体とインターフェース定義~~ → config.RepositoryConfigを使用
  - [x] ~~RepositoryManagerの実装~~ → 設定ファイルから直接読み込み
- [x] 設定ファイルのスキーマ拡張
  - [x] Viper設定にリポジトリリストを追加
  - [x] YAML設定の検証ロジック
- [x] AIリポジトリルーターの実装
  - [x] ルーター用Claude Codeプロセスの管理
  - [x] システムプロンプトの設計（リポジトリ設定情報を含む）
  - [x] JSON Schemaによる応答フォーマット定義
  - [x] Sonnetモデルを使用したルーティングロジック
  - [ ] ルーティング結果のキャッシュ機構

### Phase 2: マルチWorktree基盤
- [x] worktreesテーブルの作成（migration）
- [x] Git worktree操作のラッパー実装
  - [x] worktree作成関数
  - [x] worktree削除関数
  - [x] worktree状態確認関数
- [x] WorktreeManagerの実装
  - [x] worktreeディレクトリの管理
  - [x] 並行性制御（mutex）
- [x] セッション作成時のworktree自動作成

### Phase 3: 統合とクリーンアップ
- [x] SessionManagerの更新
  - [x] リポジトリとworktreeを考慮したセッション作成
  - [x] working directory解決ロジックの更新
- [x] 定期クリーンアップジョブの実装
  - [x] 古いworktreeの削除
  - [ ] ディスク容量監視
  - [x] goroutineによる定期実行
- [x] Web管理画面の更新
  - [x] リポジトリ一覧表示
  - [x] worktree状態表示
  - [x] APIエンドポイントの追加

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

## 変更履歴

### 2024-01-XX: 設定ファイルベースへの移行
- リポジトリ管理をデータベースから設定ファイル（config.yaml）に移行
- repositoriesテーブルを削除し、config.RepositoryConfigを使用
- worktreesテーブルをrepository_idからrepository_path/repository_nameに変更
- 理由: フォルダ名の変更に対する柔軟性とメンテナンスの簡素化