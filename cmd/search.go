package main

import (
	"fmt"
	"os"

	"duckhist/internal/config"
	"duckhist/internal/history"

	"github.com/damiendart/pathshorten"
	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Interactively search command history",
	Long: `Interactively search through command history with real-time filtering.
The initial view shows:
- Commands executed in the current directory
- Followed by commands from all other directories
As you type, the list will be filtered to match your search query.`,
	RunE: runSearch,
}

var (
	searchDirFlag string
)

func init() {
	searchCmd.Flags().StringVarP(&searchDirFlag, "directory", "d", "", "directory to search history for (default is current directory)")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	manager, err := history.NewManagerReadOnly(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to create history manager: %w", err)
	}
	defer manager.Close()

	currentDir := searchDirFlag
	if currentDir == "" {
		var err error
		currentDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Get initial history (all commands)
	allHistory, err := manager.FindHistory(currentDir, nil)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	// Create application
	app := tview.NewApplication()

	// Create table view for displaying commands
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetBorders(false)

	// Set table headers
	table.SetCell(0, 0, tview.NewTableCell("Date").SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Directory").SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Command").SetSelectable(false))

	// Create input field for search
	input := tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(0)

	// Create layout with table on top and input at bottom
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, false).
		AddItem(input, 1, 0, true)

	// Function to update table based on search query
	updateTable := func(query string) {
		// Clear table except headers
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Date").SetSelectable(false))
		table.SetCell(0, 1, tview.NewTableCell("Directory").SetSelectable(false))
		table.SetCell(0, 2, tview.NewTableCell("Command").SetSelectable(false))

		var entries []history.Entry
		var err error

		if query == "" {
			entries = allHistory
		} else {
			entries, err = manager.FindByCommand(query, currentDir)
			if err != nil {
				// Just use empty list if there's an error
				entries = []history.Entry{}
			}
		}

		// Add items in reverse order so that newer commands appear at the bottom
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			row := len(entries) - i // Account for header row

			// Format date as relative time
			dateStr := humanize.Time(entry.Timestamp)

			// Shorten directory
			dir := pathshorten.PathShorten(entry.Directory, "/", 20)

			// Add cells to the row
			table.SetCell(row, 0, tview.NewTableCell(dateStr))
			table.SetCell(row, 1, tview.NewTableCell(dir))
			table.SetCell(row, 2, tview.NewTableCell(entry.Command))
		}

		if table.GetRowCount() > 1 {
			table.Select(table.GetRowCount()-1, 0) // Select last row
		}
	}

	// Initial population of the table
	updateTable("")

	// Handle input changes
	input.SetChangedFunc(func(text string) {
		updateTable(text)
	})

	// Set up key handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEnter:
			// Output selected command and exit
			if table.GetRowCount() > 1 {
				row, _ := table.GetSelection()
				command := table.GetCell(row, 2).Text // Get command from third column
				app.Stop()
				fmt.Println(command)
			}
			return nil
		case tcell.KeyEsc:
			// Just exit without output
			app.Stop()
			return nil
		case tcell.KeyUp:
			// Move selection up
			row, _ := table.GetSelection()
			if row > 1 { // Don't select header row
				table.Select(row-1, 0)
			}
			return nil
		case tcell.KeyDown:
			// Move selection down
			row, _ := table.GetSelection()
			if row < table.GetRowCount()-1 {
				table.Select(row+1, 0)
			}
			return nil
		}
		return event
	})

	// Run application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		return fmt.Errorf("application error: %w", err)
	}

	return nil
}
