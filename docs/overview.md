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

- `duckhist add -- <command>`: コマンドをヒストリーに追加
  - ULID を内部で生成し、UUID として DuckDB に保存
  - コマンドの実行時刻、ホスト名、ディレクトリ、ユーザー名も記録
- `duckhist list`: 保存されたヒストリーを時系列順（新しい順）に表示

## 設定ファイル

設定ファイル（デフォルト: `~/.config/duckhist/duckhist.toml`）では以下の項目を設定できます：

```toml
# DuckDBのデータベースファイルのパス
database_path = "~/.duckhist.duckdb"
```

## 実装の詳細

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
2. zsh の設定ファイル（`~/.zshrc`）に以下を追加:
   ```zsh
   zshaddhistory() {
       /path/to/duckhist add -- "$1"
   }
   ```
3. コマンド履歴の表示: `duckhist list`
