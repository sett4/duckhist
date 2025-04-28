package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/embedded"
	"github.com/sett4/duckhist/internal/history"

	"github.com/spf13/cobra"
)

// InitConfig handles configuration initialization
type InitConfig struct {
	configPath string
	home       string
}

// NewInitConfig creates a new InitConfig instance
func NewInitConfig(configPath string) (*InitConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	return &InitConfig{configPath: configPath, home: home}, nil
}

// GetConfigPath returns the full path to the config file
func (ic *InitConfig) GetConfigPath() string {
	if ic.configPath == "" {
		return filepath.Join(ic.home, ".config", "duckhist", "duckhist.toml")
	}
	return ic.configPath
}

// EnsureConfigDir creates the config directory if it doesn't exist
func (ic *InitConfig) EnsureConfigDir() error {
	return os.MkdirAll(filepath.Dir(ic.GetConfigPath()), 0755)
}

// CreateDefaultConfig creates a default config file if it doesn't exist
func (ic *InitConfig) CreateDefaultConfig() error {
	configPath := ic.GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		content := `# Path to SQLite database file
database_path = "~/.duckhist.db"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		fmt.Printf("Created config file at: %s\n", configPath)
	}
	return nil
}

// InitializeDatabase loads config and initializes the database
func (ic *InitConfig) InitializeDatabase() error {
	cfg, err := config.LoadConfig(ic.GetConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create database directory
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Connect to database and create table
	manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			log.Printf("failed to close manager: %v", err)
		}
	}()

	// Run migrations to create necessary tables and indexes
	if err := RunMigrations(cfg.DatabasePath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Printf("Initialized database at: %s\n", cfg.DatabasePath)
	return nil
}

// CreateZshIntegration creates the Zsh integration script
func (ic *InitConfig) CreateZshIntegration() error {
	if ic.GetConfigPath() != filepath.Join(ic.home, ".config", "duckhist", "duckhist.toml") {
		return nil
	}

	scriptPath := filepath.Join(filepath.Dir(ic.GetConfigPath()), "zsh-duckhist.zsh")
	if err := os.WriteFile(scriptPath, []byte(embedded.GetZshIntegrationScript()), 0644); err != nil {
		return fmt.Errorf("failed to create Zsh integration script: %w", err)
	}

	fmt.Println("\nTo integrate with Zsh, add the following line to your ~/.zshrc:")
	fmt.Printf("source %s\n", scriptPath)
	fmt.Printf("\nCreated Zsh integration script at: %s\n", scriptPath)
	return nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize duckhist",
	Long:  `Initialize duckhist by creating default config file and empty database.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath := cmd.Flag("config").Value.String()

		ic, err := NewInitConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}

		if err := ic.EnsureConfigDir(); err != nil {
			log.Fatal(err)
		}

		if configPath == "" {
			if err := ic.CreateDefaultConfig(); err != nil {
				log.Fatal(err)
			}
		} else {
			// Check if custom config file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				log.Fatalf("cannot open config file: %s", configPath)
			}
		}

		// Load config and initialize database
		if err := ic.InitializeDatabase(); err != nil {
			log.Fatal(err)
		}

		if err := ic.CreateZshIntegration(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
