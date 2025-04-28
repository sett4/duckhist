package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/history"

	"github.com/spf13/cobra"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Display command history optimized for incremental search tools",
	Long: `Display command history in a format optimized for incremental search tools like peco and fzf.
The output shows:
- Last N commands executed in the current directory (N is configurable in settings)
- Followed by the full command history from all directories`,
	RunE: runHistory,
}

var (
	historyDirFlag string
)

func init() {
	historyCmd.Flags().StringVarP(&historyDirFlag, "directory", "d", "", "directory to show history for (default is current directory)")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
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

	currentDir := historyDirFlag
	if currentDir == "" {
		var err error
		currentDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Get current directory history
	limit := cfg.CurrentDirectoryHistLimit
	currentDirHistory, err := manager.Query().
		InDirectory(currentDir).
		Limit(limit).
		OrderByCurrentDirFirst(currentDir).
		GetEntries()
	if err != nil {
		return fmt.Errorf("failed to get current directory history: %w", err)
	}

	// Get full history excluding current directory entries
	fullHistory, err := manager.FindHistory(currentDir, nil)
	if err != nil {
		return fmt.Errorf("failed to get full history: %w", err)
	}

	// Keep track of printed commands to avoid duplicates
	printedCommands := make(map[string]bool)

	// Print current directory history
	for _, entry := range currentDirHistory {
		if !printedCommands[entry.Command] {
			fmt.Printf("%s\n", entry.Command)
			printedCommands[entry.Command] = true
		}
	}

	// Add delimiter between current directory history and full history
	fmt.Println("---")

	// Print full history, skipping duplicates
	for _, entry := range fullHistory {
		if !printedCommands[entry.Command] {
			fmt.Printf("%s\n", entry.Command)
			printedCommands[entry.Command] = true
		}
	}

	return nil
}
