# Multi-Repository and Multi-Worktree Support for cc-slack

## 🎯 概要

cc-slackに以下の2つの重要な機能を実装する：

1. **Multi-Repository対応**: 複数のリポジトリに対してClaude Codeを同時実行可能にする
2. **Multi-Worktree対応**: 単一リポジトリ内で複数の作業ディレクトリ（Git worktree）を使用して並列作業を可能にする

## 🔥 解決したい問題

### 現状の制限
- 単一リポジトリ、単一作業ディレクトリのみサポート
- ファイルシステムがボトルネックとなり、ファイル変更を伴う作業は同時に1つまで
- 調査系タスクは並列実行可能だが、ファイル変更が混在するとbranchの状態が不安定になる

### ユースケース
- 複数のプロジェクトを同時に管理したい
- 単一プロジェクト内で複数の機能開発を並列で進めたい
- コードレビューと新機能開発を同時に行いたい

## 💡 提案するソリューション

### 1. Multi-Repository対応

#### 基本設計
- 設定ファイル（config.yaml）でリポジトリのリストを管理
- リポジトリごとに以下を設定可能：
  - パス
  - 表示名
  - Slack設定のオーバーライド（チャンネル、ユーザー名、アイコン等）
  - デフォルトブランチ

#### リポジトリ選択方法（検討事項）
- **Option A**: チャンネルベースのマッピング
  - 特定のチャンネルを特定のリポジトリに紐付け
  - 設定例: `#project-a` → `/path/to/project-a`
- **Option B**: Slackインタラクティブメッセージ
  - セッション開始時にドロップダウンで選択
- **Option C**: メンション時のパラメータ
  - `@claude-code [repo:project-a] タスクの内容`
- **Option D**: AIによる自動判定（採用予定）
  - チャンネルに複数リポジトリが設定されている場合のみルーターを起動
  - 単一リポジトリの場合は直接実行（パフォーマンス最適化）
  - ルーター使用時：
    - 別のClaude Codeプロセスがルーターとして機能
    - Claude Code SDKのカスタムシステムプロンプト機能を活用
    - 設定情報とJSON Schemaで返却フォーマットを定義
    - モデルはコスト効率を考慮してSonnet（`--model claude-3-5-sonnet-latest`）を使用
    - メッセージ内容からリポジトリを推定し、適切なリポジトリに振り分け

### 2. Multi-Worktree対応

#### 基本設計
- Git worktree機能を活用
- 1スレッド = 1 worktreeの原則
- worktreeは専用ディレクトリ（例: `.cc-slack-worktrees/`）に作成

#### ライフサイクル管理
- **作成**: 新規スレッド開始時に自動作成
- **ベースブランチ**: デフォルトブランチまたは指定されたブランチ
- **削除**: セッション終了後、設定可能な期間（例: 24時間）後に自動削除
- **再利用**: resume機能使用時は既存のworktreeを再利用

## 📊 データモデルの変更

### 新規テーブル

#### repositories
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

#### worktrees
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

#### threads
- `repository_id INTEGER` カラムを追加
- `working_directory` を `worktree_id` に変更（または両方保持）

## 🛠 実装タスク

### Phase 1: Multi-Repository基盤
- [ ] repositoriesテーブルの作成（migration）
- [ ] Repository管理機能の実装
  - [ ] Repository構造体とインターフェース定義
  - [ ] RepositoryManagerの実装
- [ ] 設定ファイルのスキーマ拡張
- [ ] AIリポジトリルーターの実装（Option D）
  - [ ] ルーター用Claude Codeプロセスの管理
  - [ ] システムプロンプトの設計（リポジトリ設定情報を含む）
  - [ ] JSON Schemaによる応答フォーマット定義
  - [ ] Sonnetモデルを使用したルーティングロジック
  - [ ] ルーティング結果のキャッシュ機構

### Phase 2: Multi-Worktree基盤
- [ ] worktreesテーブルの作成（migration）
- [ ] Git worktree操作のラッパー実装
  - [ ] worktree作成関数
  - [ ] worktree削除関数
  - [ ] worktree状態確認関数
- [ ] WorktreeManagerの実装
- [ ] セッション作成時のworktree自動作成

### Phase 3: 統合とクリーンアップ
- [ ] SessionManagerの更新
  - [ ] リポジトリとworktreeを考慮したセッション作成
  - [ ] working directory解決ロジックの更新
- [ ] 定期クリーンアップジョブの実装
  - [ ] 古いworktreeの削除
  - [ ] ディスク容量監視
- [ ] Web管理画面の更新
  - [ ] リポジトリ一覧表示
  - [ ] worktree状態表示

### Phase 4: 高度な機能
- [ ] リポジトリ選択の高度なオプション実装（B, C, D）
- [ ] worktree再利用の最適化
- [ ] 並列実行数の制限機能
- [ ] リソース使用量のモニタリング

## 🤔 検討事項

### セキュリティ
- [ ] リポジトリアクセス権限の管理方法
- [ ] Slackユーザーとリポジトリのマッピング

### パフォーマンス
- [ ] worktree作成のオーバーヘッド
- [ ] ディスク容量の監視と制限
- [ ] 同時実行数の適切な上限

### UX
- [ ] 現在のリポジトリ/worktreeの可視化方法
- [ ] エラー時のフィードバック方法
- [ ] リポジトリ切り替えの直感的な方法

### 運用
- [ ] バックアップ戦略
- [ ] worktreeのライフサイクルポリシー
- [ ] ログとモニタリング

## 📈 成功指標

- 複数リポジトリでの同時作業が可能
- 単一リポジトリでの並列作業数が3倍以上に向上
- worktree作成/削除の自動化によるメンテナンスフリー
- ユーザーからの「ファイル競合エラー」報告がゼロに

## 🔗 参考資料

- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)
- [GitHub: Using Git Worktree](https://github.blog/2015-07-29-git-2-5-including-multiple-worktrees-and-triangular-workflows/)
- [Claude Code SDK - Custom System Prompts](https://docs.anthropic.com/en/docs/claude-code/sdk#custom-system-prompts)
- [Claude Code CLI Reference](https://docs.anthropic.com/en/docs/claude-code/cli-reference)
- Multi-tenant SaaS設計パターン

## 🚀 次のステップ

1. このissueに対するフィードバックを収集
2. Phase 1の詳細設計ドキュメント作成
3. プロトタイプ実装で技術的課題を検証
4. 段階的なリリース計画の策定