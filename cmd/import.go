package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/history"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
)

var (
	importFile string
	importCmd  = &cobra.Command{
		Use:   "import",
		Short: "Import commands from a CSV file",
		Long: `Import commands from a CSV file into the history database.
The CSV file must have the following columns:
- id: ULID or UUID String representation (optional)
- command: Text (required)
- executed_at: Timestamp (optional)
- executing_host: Text (optional)
- executing_dir: Text (optional)
- executing_user: Text (optional)
- sid: Text (optional)
- tty: Text (optional)

If id is empty, a new ULID will be generated based on the current time.`,
		RunE: runImport,
	}
)

func init() {
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "CSV file to import (required)")
	importCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open CSV file
	file, err := os.Open(importFile)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Create column index map
	columnMap := make(map[string]int)
	for i, col := range header {
		columnMap[strings.ToLower(col)] = i
	}

	// Verify required columns
	if _, ok := columnMap["command"]; !ok {
		return fmt.Errorf("CSV file must have a 'command' column")
	}

	// Create history manager
	manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to create history manager: %w", err)
	}
	defer manager.Close()

	// Import records
	lineNum := 1 // 1-based line number (header is line 1)
	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV line %d: %w", lineNum, err)
		}

		// Get values from record
		id := getColumnValue(record, columnMap, "id")
		if id == "" {
			id = ulid.Make().String()
		}

		command := getColumnValue(record, columnMap, "command")
		if command == "" {
			log.Printf("Skipping empty command at line %d", lineNum)
			continue
		}

		executedAt := time.Now()
		if execTimeStr := getColumnValue(record, columnMap, "executed_at"); execTimeStr != "" {
			parsedTime, err := time.Parse(time.RFC3339, execTimeStr)
			if err != nil {
				log.Printf("Warning: Invalid timestamp at line %d, using current time: %v", lineNum, err)
			} else {
				executedAt = parsedTime
			}
		}

		hostname := getColumnValue(record, columnMap, "executing_host")
		if hostname == "" {
			var err error
			hostname, err = os.Hostname()
			if err != nil {
				log.Printf("Warning: Failed to get hostname at line %d: %v", lineNum, err)
			}
		}

		directory := getColumnValue(record, columnMap, "executing_dir")
		if directory == "" {
			var err error
			directory, err = os.Getwd()
			if err != nil {
				log.Printf("Warning: Failed to get current directory at line %d: %v", lineNum, err)
			}
		}

		username := getColumnValue(record, columnMap, "executing_user")
		if username == "" {
			username = os.Getenv("USER")
		}

		tty := getColumnValue(record, columnMap, "tty")
		sid := getColumnValue(record, columnMap, "sid")

		// Add command to history
		_, err = manager.AddCommand(command, directory, tty, sid, hostname, username, executedAt, true)
		if err != nil {
			log.Printf("Warning: Failed to import command at line %d: %v", lineNum, err)
			continue
		}
	}

	return nil
}

// getColumnValue safely gets a value from a CSV record using the column map
func getColumnValue(record []string, columnMap map[string]int, columnName string) string {
	if idx, ok := columnMap[columnName]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}
