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
		// Get home directory
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}

		// Get config file path from flag or use default
		configPath := cmd.Flag("config").Value.String()
		if configPath == "" {
			defaultConfigDir := filepath.Join(home, ".config", "duckhist")
			configPath = filepath.Join(defaultConfigDir, "duckhist.toml")

			// Create config directory
			if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
				log.Fatal(err)
			}

			// Create default config file if it doesn't exist
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				content := `# Path to DuckDB database file
database_path = "~/.duckhist.duckdb"
`
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					log.Fatal(err)
				}
				fmt.Printf("Created config file at: %s\n", configPath)
			}
		} else {
			// Create config directory for custom path
			configDir := filepath.Dir(configPath)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				log.Fatal(err)
			}

			// Create custom config file if it doesn't exist
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				log.Fatal(fmt.Sprintf("cannot open config file: %s", configPath))
			}
		}

		// Load config and initialize database
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}

		// Create database directory
		dbDir := filepath.Dir(cfg.DatabasePath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatal(err)
		}

		// Connect to database and create table
		manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		fmt.Printf("Initialized database at: %s\n", cfg.DatabasePath)

		// Only create Zsh integration script for default config
		if configPath == filepath.Join(home, ".config", "duckhist", "duckhist.toml") {
			fmt.Println("\nTo integrate with Zsh, add the following line to your ~/.zshrc:")
			scriptPath := filepath.Join(filepath.Dir(configPath), "zsh-duckhist.zsh")
			fmt.Printf("source %s\n", scriptPath)

			// Copy zsh-duckhist.zsh
			scriptContent := `# duckhist zsh integration
duckhist_add_history() {
    duckhist add -- "$1"
}
zshaddhistory_functions+=duckhist_add_history
`
			if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("\nCreated Zsh integration script at: %s\n", scriptPath)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
