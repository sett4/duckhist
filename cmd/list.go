package cmd

import (
	"fmt"
	"log"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/history"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List command history",
	Long:  `List all commands in the history database in reverse chronological order.`,
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	manager, err := history.NewManagerReadOnly(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to create history manager: %w", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			log.Printf("failed to close manager: %v", err)
		}
	}()

	commands, err := manager.ListCommands()
	if err != nil {
		return fmt.Errorf("failed to list commands: %w", err)
	}

	for _, command := range commands {
		fmt.Println(command)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
}
