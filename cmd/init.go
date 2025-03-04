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
		// Get default config file path
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		defaultConfigDir := filepath.Join(home, ".config", "duckhist")
		defaultConfigPath := filepath.Join(defaultConfigDir, "duckhist.toml")

		// Create config directory
		if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
			log.Fatal(err)
		}

		// Create config file if it doesn't exist
		if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
			content := `# Path to DuckDB database file
database_path = "~/.duckhist.duckdb"
`
			if err := os.WriteFile(defaultConfigPath, []byte(content), 0644); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Created config file at: %s\n", defaultConfigPath)
		}

		// Load config and initialize database
		cfg, err := config.LoadConfig(defaultConfigPath)
		if err != nil {
			log.Fatal(err)
		}

		// Create database directory
		dbDir := filepath.Dir(cfg.DatabasePath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatal(err)
		}

		// Connect to database and create table
		manager, err := history.NewManager(cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		fmt.Printf("Initialized database at: %s\n", cfg.DatabasePath)
		fmt.Println("\nTo integrate with Zsh, add the following line to your ~/.zshrc:")
		fmt.Printf("source %s\n", filepath.Join(defaultConfigDir, "zsh-duckhist.zsh"))

		// Copy zsh-duckhist.zsh
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
		fmt.Printf("\nCreated Zsh integration script at: %s\n", scriptPath)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
