---
title: データベース統合と拡張機能の実装
status: done
---

# データベース統合と拡張機能の実装

PR #17 で基盤は作成されたが、実際の統合がまだ完了していない。このタスクでデータベース機能と関連する拡張機能を依存関係を考慮して実装する。

## Phase 1: データベース基盤の完成（前提条件）

### 1.1 データベース基盤の動作確認と修正
- [x] SQLiteデータベース統合の動作確認（internal/database/）✅ database.go 実装済み
- [x] 独自マイグレーション実装（internal/database/migrate.go）✅ golang-migrate の代わりに独自実装を使用
- [x] sqlcによる型安全なクエリ生成の動作確認（internal/db/）✅ 生成されたコード存在
- [x] データベース初期化とマイグレーション実行の動作確認 ✅ 実装・動作確認済み

### 1.2 main.go の統合 ✅ 実装完了
- [x] データベース初期化処理の追加（database.Open と migrate.Migrate の呼び出し）✅ 実装済み
- [x] エラーハンドリングの実装 ✅ エラー時は log.Fatalf で適切に処理
- [x] 起動時のデータベース接続確認 ✅ Ping 実行済み
- [x] db.Queries と ResumeManager の初期化 ✅ 実装済み
- [x] SessionManager を DBManager に置き換え ✅ 実装済み

### 1.3 基本的なテスト
- [x] データベース操作のユニットテスト ✅ database_test.go 実装済み
- [x] マイグレーションのテスト ✅ migrate_test.go 実装済み
- [x] ResumeManager のテスト ✅ resume_test.go 存在

## Phase 2: セッション永続化（Phase 1 に依存）

### 2.1 DBManagerの完全実装
- [x] `internal/session/db_manager.go` の実装完了 ✅ 基本実装完了
- [x] SessionManager インターフェースの完全実装 ✅ 主要メソッド実装済み
- [x] メモリベースのManagerとの統合 ✅ DBManager が Manager をラップして実装済み

### 2.2 セッション情報の永続化
- [x] セッション情報のDB保存 ✅ CreateSession 実装済み
- [x] スレッドとセッションの紐付け管理 ✅ threads/sessions テーブル構造実装済み
- [x] セッション統計情報の記録（コスト、トークン数、実行時間）✅ UpdateSessionStatus 実装済み

### 2.3 テスト
- [x] DBManagerの統合テスト ✅ db_integration_test.go 実装済み
- [x] セッション永続化のE2Eテスト ✅ session_persistence_test.go 実装済み

## Phase 3: セッション再開機能（Phase 2 に依存）

### 3.1 ResumeManagerの実装
- [x] ResumeManagerによる再開判定ロジック（internal/process/resume.go）✅ 実装済み
- [x] 再開可能時間の設定（デフォルト1時間）✅ resumeWindow フィールドで管理
- [x] 既存セッションの自動検出 ✅ ShouldResume メソッド実装済み

### 3.2 Claude Code との統合
- [x] 同一スレッド内でのセッション継続（--resume オプション）✅ db_manager.go で実装済み
- [x] 再開時のコンテキスト復元 ✅ ResumeSessionID オプション実装済み
- [x] エラーハンドリング ✅ resume 失敗時の新規セッション作成実装済み

### 3.3 テスト
- [x] セッション再開のE2Eテスト ✅ session_resume_test.go 実装済み
- [x] タイムアウト境界のテスト ✅ session_timeout_test.go 実装済み（一部タイミング依存テストはスキップ）

## Phase 4: 複数ワーキングディレクトリ対応（Phase 2 に依存）

### 4.1 データモデルの拡張 ⚠️ 未実装
- [ ] working_directories テーブルの作成（マイグレーション追加が必要）
- [ ] チャンネルとワーキングディレクトリの紐付け

### 4.2 選択ロジックの実装 ⚠️ 未実装
- [ ] WorkspaceSelector の実装
- [ ] メンション時のproject指定パース
- [ ] デフォルト処理

### 4.3 設定との統合
- [ ] Viper設定との連携
- [ ] 環境変数サポート

## Phase 5: Webマネジメントコンソール（Phase 2 に依存）⚠️ 未実装

### 5.1 バックエンドAPI
- [ ] RESTful API の設計と実装
- [ ] セッション一覧エンドポイント
- [ ] 統計情報エンドポイント
- [ ] Server-Sent Events for リアルタイム更新

### 5.2 フロントエンドUI
- [ ] 基本的なHTML/CSSレイアウト
- [ ] セッション履歴の表示
- [ ] 統計情報のビジュアライゼーション
- [ ] リアルタイムモニタリング

### 5.3 認証とセキュリティ
- [ ] Basic認証の実装
- [ ] CORS設定
- [ ] Rate limiting

## 実装上の注意事項

### 実装完了したコンポーネント
- `cmd/cc-slack/main.go` にデータベース初期化のコード ✅ 実装完了
- `internal/session/db_manager.go` と main.go との統合 ✅ 実装完了

### 実装済みだが未統合のコンポーネント
- `internal/database/database.go` - DB接続処理 ✅
- `internal/database/migrate.go` - マイグレーション処理（独自実装）✅
- `internal/db/` - sqlc 生成コード ✅
- `internal/process/resume.go` - Resume 判定ロジック ✅
- `internal/session/db_manager.go` - DB永続化対応のSessionManager ✅

### 実装の優先順位と現在の状況
1. **必須**: Phase 1-3（データベース基盤 → セッション永続化 → セッション再開）
   - Phase 1: 100% 完了 ✅
   - Phase 2: 100% 完了 ✅
   - Phase 3: 90% 完了 ⚠️ **実装はあるが統合に問題あり（2025-01-27）**
     - `handler.go` が古い `CreateSession` を使用しているため resume 機能が動作しない
     - 詳細は `003-minimal-session-persistence.md` の問題発見セクションを参照
2. **推奨**: Phase 4（複数ワーキングディレクトリ）- 0% 未着手
3. **オプション**: Phase 5（Webコンソール）- 0% 未着手

### 次のアクション
1. **最優先**: main.go でのデータベース初期化とDBManager への切り替え
2. デフォルトの working_directory 設定の確認
3. 実際の動作確認とテスト

### 既存実装との整合性
- Viper設定管理は実装済みなので活用する
- メモリベースのSessionManagerと互換性を保つ
- 既存のテストが通ることを確認しながら進める