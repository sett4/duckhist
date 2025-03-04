package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"duckhist/internal/config"
	"duckhist/internal/history"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize duckhist",
	Long:  `Initialize duckhist by creating default config file and empty database.`,
	Run: func(cmd *cobra.Command, args []string) {
		// デフォルトの設定ファイルのパスを取得
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		defaultConfigDir := filepath.Join(home, ".config", "duckhist")
		defaultConfigPath := filepath.Join(defaultConfigDir, "duckhist.toml")

		// 設定ディレクトリを作成
		if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
			log.Fatal(err)
		}

		// 設定ファイルが存在しない場合のみ作成
		if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
			content := `# DuckDBのデータベースファイルのパス
database_path = "~/.duckhist.duckdb"
`
			if err := os.WriteFile(defaultConfigPath, []byte(content), 0644); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Created config file: %s\n", defaultConfigPath)
		}

		// 設定を読み込んでデータベースを初期化
		cfg, err := config.LoadConfig(defaultConfigPath)
		if err != nil {
			log.Fatal(err)
		}

		// データベースディレクトリを作成
		dbDir := filepath.Dir(cfg.DatabasePath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatal(err)
		}

		// データベースに接続してテーブルを作成
		manager, err := history.NewManager(cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		fmt.Printf("Initialized database: %s\n", cfg.DatabasePath)
		fmt.Println("\nTo integrate with zsh, add the following to your ~/.zshrc:")
		fmt.Printf("source %s\n", filepath.Join(defaultConfigDir, "zsh-duckhist.zsh"))

		// zsh-duckhist.zshをコピー
		scriptContent := `# duckhist zsh integration
duckhist_add_history() {
    duckhist add -- "$1"
}
zshaddhistory_functions+=duckhist_add_history
`
		scriptPath := filepath.Join(defaultConfigDir, "zsh-duckhist.zsh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nCreated zsh integration script: %s\n", scriptPath)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
