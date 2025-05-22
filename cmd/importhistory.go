package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/history"
	"github.com/spf13/cobra"
)

var getUserHomeDir = os.UserHomeDir // Can be overridden in tests

// importHistoryCmd represents the import-history command
var importHistoryCmd = &cobra.Command{
	Use:   "import-history",
	Short: "Import commands from ~/.zsh_history",
	Long:  `Reads commands from ~/.zsh_history and saves them to the history database.`,
	RunE:  runImportHistory,
}

func init() {
	rootCmd.AddCommand(importHistoryCmd)
}

func runImportHistory(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	homeDir, err := getUserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	historyFilePath := filepath.Join(homeDir, ".zsh_history")

	file, err := os.Open(historyFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("History file not found: %s\n", historyFilePath)
			return nil
		}
		return fmt.Errorf("failed to open history file %s: %w", historyFilePath, err)
	}
	defer file.Close()

	manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize history manager: %w", err)
	}
	defer manager.Close()

	scanner := bufio.NewScanner(file)
	importedCount := 0
	skippedCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		var commandText string
		var timestamp time.Time

		if strings.HasPrefix(line, ": ") {
			parts := strings.SplitN(line, ";", 2)
			if len(parts) < 2 {
				log.Printf("Skipping malformed zsh history line: %s", line)
				continue
			}
			commandText = strings.TrimSpace(parts[1])

			tsParts := strings.SplitN(parts[0], ":", 3) // : <timestamp>:<duration>
			if len(tsParts) < 2 {
				log.Printf("Skipping malformed zsh history line (timestamp): %s", line)
				continue
			}
			tsStr := strings.TrimSpace(tsParts[1])
			tsInt, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				log.Printf("Failed to parse timestamp '%s', using current time: %v", tsStr, err)
				timestamp = time.Now()
			} else {
				timestamp = time.Unix(tsInt, 0)
			}
		} else {
			commandText = strings.TrimSpace(line)
			timestamp = time.Now()
		}

		if commandText == "" {
			continue
		}

		hostname, _ := os.Hostname()
		directory, _ := os.Getwd()
		username := os.Getenv("USER")
		tty := ""    // Not available from zsh history
		sid := ""    // Not available from zsh history

		skipped, err := manager.AddCommand(commandText, directory, tty, sid, hostname, username, timestamp, false)
		if err != nil {
			log.Printf("Failed to import command: \"%s\": %v", commandText, err)
		} else {
			if skipped {
				skippedCount++
			} else {
				importedCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading history file: %v", err)
	}

	fmt.Printf("Imported %d commands and skipped %d duplicate commands from %s\n", importedCount, skippedCount, historyFilePath)
	return nil
}
