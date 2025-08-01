---
title: セッション開始時の初期プロンプトを保存し、Web管理コンソールに表示する（サーバーサイド実装完了）
status: done
---

# 007: セッション開始時の初期プロンプトを保存し、Web管理コンソールに表示する

## 設計見直し (2025-01-28 追記)

### 現在の実装の問題点

現在の実装では `CreateSessionWithResume` に `initialPrompt` 引数を追加したが、これは設計として不適切：

1. **責務の混在**: `CreateSessionWithResume` はセッション作成とClaude プロセス起動を行うが、実際のメッセージ送信は後の `SendMessage` で行われる
2. **データの不整合**: initial_prompt を保存するタイミングと実際にメッセージを送信するタイミングがずれている
3. **インターフェースの不自然さ**: セッション作成時にメッセージを渡すが、そのメッセージは使われない

### 設計オプション

#### Option A: SendMessage で最初のメッセージ送信時に保存 ✅（推奨）
- Session に `firstMessageSent` フラグを追加
- `SendMessage` で最初のメッセージかチェックし、初回なら initial_prompt として保存
- **メリット**: 既存のインターフェース変更が最小限、実装がシンプル
- **デメリット**: SendMessage 内でのステート管理が必要

#### Option B: 明示的な SetInitialPrompt メソッド
- セッション作成後、別途 `SetInitialPrompt` を呼ぶ
- **メリット**: 責務が明確
- **デメリット**: 呼び出し側で追加処理が必要、呼び忘れのリスク

#### Option C: セッション作成とメッセージ送信の一体化
- `CreateSessionAndSendInitialMessage` のような新メソッド
- **メリット**: 一体化されて分かりやすい
- **デメリット**: 大きなインターフェース変更

#### Option D: NewClaudeProcess に initialPrompt を追加 ✅（推奨 - Claude CLI の設計に合わせる）
- `NewClaudeProcess` の `Options` に `InitialPrompt` フィールドを追加
- プロセス起動直後に最初のプロンプトを送信
- セッション作成時にDBに initial_prompt を保存
- **メリット**: Claude CLI の設計思想に合致、インターフェースが自然、実装タイミングが明確
- **デメリット**: 既存のインターフェース変更が必要（ただし最小限）

### 実装方針（修正版）

Option D を採用し、以下の手順で実装：

1. `process.Options` に `InitialPrompt` フィールドを追加
2. `NewClaudeProcess` 内で初期プロンプトを送信
3. `CreateSessionWithResume` で initial_prompt をDBに保存（ただし別の方法で）
4. `handleAppMention` から `SendMessage` 呼び出しを削除

## 概要

**進捗状況**: サーバーサイドとフロントエンドの実装が完了。初期プロンプトがデータベースに保存され、Web管理コンソールに正しく表示されることを確認済み。

### 実装内容

**フロントエンド実装（2025-01-29）:**
- `truncatePrompt` 関数を実装（100文字で切り詰め、複数の空白を１つに正規化）
- `SessionList` コンポーネントに初期プロンプト表示を追加
- `ThreadSessionsPage` のテーブルにも初期プロンプトカラムを追加（50文字で切り詰め）
- NULLの場合は "No prompt available" または "No prompt" と表示
- ホバー時にtitle属性で全文表示

### 解決したい問題

現在のWeb管理コンソールのセッション一覧画面では、セッションIDとタイムスタンプのみが表示されており、どのような作業を行っているセッションなのかが判別しづらい。ユーザーがセッションの内容を把握するためには、各セッションをクリックして詳細を確認する必要があり、使い勝手が悪い。

この問題を解決するため、セッション開始時の最初のプロンプト（ユーザーがClaudeに送った最初のメッセージ）をデータベースに保存し、セッション一覧画面に表示することで、各セッションの目的や内容を一目で把握できるようにする。

## 修正実装手順（Option D に基づく）

**サーバーサイド実装: 完了 ✅**

### 1. ロールバック作業
- [x] CreateSessionWithResume から initialPrompt 引数を削除
- [x] SessionManager インターフェースを元に戻す 
- [x] handleAppMention の呼び出しを元に戻す（ただし SendMessage 呼び出しは削除）

### 2. 新規実装
- [x] `process.Options` に `InitialPrompt` フィールドを追加
- [x] `NewClaudeProcess` を修正:
  - [x] 初期プロンプトがある場合、プロセス起動後に自動送信
  - [x] 空の場合はスキップ（互換性のため）
- [x] `CreateSessionWithResume` で initial_prompt を受け取ってDBに保存:
  - [x] Options に渡して NewClaudeProcess で使用
  - [x] CreateSessionWithInitialPrompt でDBに保存（そのまま使用）
- [x] `handleAppMention` を修正:
  - [x] text を CreateSessionWithResume に渡す
  - [x] SendMessage 呼び出しを削除

### 3. テスト
- [x] ビルドとユニットテストの実行確認
- [x] 新規セッション作成時に初期プロンプトが正しく保存されることを確認（手動テスト）
- [x] resumeセッション時も初期プロンプトが保存されることを確認（手動テスト）
- [ ] 既存セッション（initial_prompt = NULL）の表示が崩れないことを確認（フロントエンド実装後）

## 元の手順（参考）

### 1. データベースのマイグレーション

- [x] 新しいマイグレーションファイルを作成: `000007_add_initial_prompt_to_sessions.up.sql` / `.down.sql`
- [x] `sessions` テーブルに `initial_prompt TEXT` カラムを追加
- [x] sqlc のクエリを更新して、initial_prompt を含めたセッション作成・取得処理を追加 → **CreateSessionWithInitialPrompt は不要、UpdateSessionInitialPrompt を追加**
- [x] `sqlc generate` を実行してコード生成

### 2. サーバーサイドの変更

#### 2.1 セッション作成時の初期プロンプト保存

- [ ] `internal/slack/handler.go` の `handleAppMention` メソッドを修正
  - [ ] `removeBotMention` で取得したテキストを初期プロンプトとして保存 → **設計変更により不要**
- [ ] `internal/session/manager.go` の `CreateSessionWithResume` メソッドを修正
  - [ ] 初期プロンプトを受け取る引数を追加 → **設計変更により不要**
  - [ ] データベースへのセッション作成時に初期プロンプトを保存 → **SendMessage で実装**
- [ ] `internal/process/claude.go` の `NewClaudeProcess` 関数のシグネチャを更新（必要に応じて） → **設計変更により不要**

#### 2.2 SendMessage での初期プロンプト保存（新設計）

- [ ] sqlc クエリに `UpdateSessionInitialPrompt` を追加
- [ ] Session 構造体に `firstMessageSent` フラグを追加
- [ ] `SendMessage` メソッドを修正して初回メッセージ時に initial_prompt を保存
- [ ] 既に実装済みの変更をロールバック
  - [ ] CreateSessionWithResume から initialPrompt 引数を削除
  - [ ] SessionManager インターフェースを元に戻す
  - [ ] handleAppMention の呼び出しを元に戻す

#### 2.3 API エンドポイントの更新

- [x] `/api/sessions` エンドポイントのレスポンスに `initial_prompt` フィールドを追加
- [x] `/api/threads/:thread_id/sessions` エンドポイントのレスポンスにも同様に追加
- [x] セッション取得時のクエリを更新して initial_prompt を含める

### 3. フロントエンドの変更

#### 3.1 TypeScript インターフェースの更新

- [ ] `web/src/components/SessionList.tsx` の `Session` インターフェースに `initial_prompt?: string` を追加
- [ ] 他のセッション関連コンポーネントのインターフェースも同様に更新

#### 3.2 UI の改善

- [ ] セッション一覧での初期プロンプト表示方法を決定
  - オプション1: プレーンテキストとして最初の1-2行を表示
  - オプション2: Markdownパーサーを通して整形して表示
  - オプション3: 最初の100文字程度を切り詰めて表示し、ホバーで全文表示
- [ ] セッションカードのレイアウトを調整して初期プロンプトを含める
- [ ] 初期プロンプトが長い場合の表示制御（truncate、展開/折りたたみなど）

#### 3.3 ユーザビリティの向上

- [ ] 初期プロンプトが空の既存セッションへの対応（"プロンプトなし" などの表示）
- [ ] 検索機能の追加を検討（初期プロンプトでのフィルタリング）

## 実装上の注意点

1. **既存データとの互換性**: 既存のセッションには initial_prompt が NULL になるため、フロントエンドで適切に処理する
2. **プライバシー**: 初期プロンプトには機密情報が含まれる可能性があるため、適切なアクセス制御を維持
3. **パフォーマンス**: 大量のセッションがある場合の一覧表示パフォーマンスに注意
4. **文字数制限**: 初期プロンプトが非常に長い場合の保存・表示方法を考慮
5. **セッション再開時の扱い**: resumeセッションも新しいセッションとして扱い、そのセッションを開始したプロンプトを保存する

## テスト項目

- [ ] 新規セッション作成時に初期プロンプトが正しく保存されることを確認
- [ ] 既存セッション（initial_prompt = NULL）の表示が崩れないことを確認
- [ ] 長い初期プロンプトの表示が適切に制御されることを確認
- [ ] セッション再開時の動作確認（resumeセッションでもそのセッションを開始したプロンプトが保存される）