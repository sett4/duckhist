# プロジェクト概要

このプロジェクトは`duckhist`と呼ばれます。

このプロジェクト duckhist は、zsh のヒストリーを DuckDB に保存する Go 言語のプログラムを開発することを目的としています。以下にプロジェクトのスケルトンを示します。

## プロジェクトのディレクトリ構造

- `cmd/`: コマンドラインツールのエントリーポイント
  - `main.go`: メインのエントリーポイント
- `internal/`: 内部ロジック
  - `history/`: ヒストリー管理のロジック
    - `manager.go`: ヒストリーの読み込み、保存、管理
- `pkg/`: 外部に公開するパッケージ（必要に応じて）
- `scripts/`: スクリプトやユーティリティ
- `test/`: テストコード

## 必要な依存関係

- `github.com/duckdb/duckdb-go`: DuckDB の Go バインディング
- `github.com/spf13/cobra`: コマンドラインインターフェースの構築に使用

## コマンドライン

### グローバルオプション

- `--config CONFIG_FILE`: 設定ファイルのパス（デフォルト: `~/.config/duckhist/duckhist.toml`）

### サブコマンド

- `duckhist init`: 初期設定を行う
  - デフォルトの設定ファイル（`~/.config/duckhist/duckhist.toml`）を作成
  - 空のデータベースファイル（`~/.duckhist.duckdb`）を作成
- `duckhist add -- <command>`: コマンドをヒストリーに追加
  - ULID を内部で生成し、UUID として DuckDB に保存
  - コマンドの実行時刻、ホスト名、ディレクトリ、ユーザー名も記録
- `duckhist list`: 保存されたヒストリーを時系列順（新しい順）に表示
- `duckhist history`: コマンド履歴をインクリメンタルサーチツール（peco/fzf）向けに出力
  - 最新の N 件: カレントディレクトリで実行されたコマンドのみを表示（N は設定ファイルで指定可能、デフォルト 5 件）
  - それ以降: 全てのディレクトリのコマンド履歴を表示
  - peco や fzf と組み合わせることで、効率的なコマンド履歴の検索が可能
- `duckhist schema-migrate`: データベースのスキーマを最新バージョンに更新
  - マイグレーションファイルによる安全なスキーマ更新
  - ロールバック機能のサポート
- `duckhist force-version`: データベースのスキーマバージョンを強制的に設定
  - `--config`: 設定ファイルのパスを指定
  - `--update-to`: 設定するバージョン番号を指定（デフォルト: 2）

## 設定ファイル

設定ファイル（デフォルト: `~/.config/duckhist/duckhist.toml`）では以下の項目を設定できます：

```toml
# DuckDBのデータベースファイルのパス
database_path = "~/.duckhist.duckdb"

# カレントディレクトリの履歴表示件数（デフォルト: 5）
current_directory_history_limit = 5
```

## 実装の詳細

### スキーマ管理

- `internal/migrations/`: データベーススキーマのマイグレーション
  - `000001_create_history_table.up.sql`: 初期スキーマ作成（history テーブル）
  - `000001_create_history_table.down.sql`: ロールバック用
  - `000002_add_primary_key_and_index.up.sql`: インデックス追加
  - `000002_add_primary_key_and_index.down.sql`: ロールバック用
- マイグレーションファイルによる安全なスキーマ更新
  - 各マイグレーションファイルはバージョン番号を持ち、順番に適用
  - スキーマバージョンはデータベース内の`schema_migrations`テーブルで管理
  - `force-version`コマンドでスキーマバージョンを強制的に設定可能

### ディレクトリ構造

- `cmd/`: コマンドラインインターフェース
  - `main.go`: エントリーポイント
  - `root.go`: ルートコマンドの定義
  - `add.go`: add サブコマンドの実装
  - `list.go`: list サブコマンドの実装
- `internal/history/`: ヒストリー管理のロジック
  - `manager.go`: DuckDB への接続とクエリの実行
    - ULID を生成して UUID に変換し、時系列でソート可能な ID を実現
    - `~/.duckhist.duckdb`にデータを保存

## 使用方法

1. プロジェクトをビルド: `go build -o duckhist ./cmd`
2. 初期設定を実行: `duckhist init`
3. zsh の設定ファイル（`~/.zshrc`）に以下を追加:
   ```zsh
   zshaddhistory() {
       /path/to/duckhist add -- "$1"
   }
   ```
4. コマンド履歴の表示: `duckhist list`
5. インクリメンタルサーチでコマンド履歴を検索:

   ```zsh
   # pecoを使用する場合
   duckhist history | peco

   # fzfを使用する場合
   duckhist history | fzf
   ```
