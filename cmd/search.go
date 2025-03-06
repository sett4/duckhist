package main

import (
	"fmt"
	"os"

	"duckhist/internal/config"
	"duckhist/internal/history"

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
	allHistory, err := manager.GetAllHistory(currentDir)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	// Create application
	app := tview.NewApplication()

	// Create list view for displaying commands
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetWrapAround(true)

	// Create input field for search
	input := tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(0)

	// Create layout with list on top and input at bottom
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, false).
		AddItem(input, 1, 0, true)

	// Function to update list based on search query
	updateList := func(query string) {
		list.Clear()

		var entries []history.Entry
		var err error

		if query == "" {
			entries = allHistory
		} else {
			entries, err = manager.SearchCommands(query, currentDir)
			if err != nil {
				// Just use empty list if there's an error
				entries = []history.Entry{}
			}
		}

		for i, entry := range entries {
			list.AddItem(entry.Command, "", rune('a'+i%26), nil)
		}

		if list.GetItemCount() > 0 {
			list.SetCurrentItem(0)
		}
	}

	// Initial population of the list
	updateList("")

	// Handle input changes
	input.SetChangedFunc(func(text string) {
		updateList(text)
	})

	// Set up key handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEnter:
			// Output selected command and exit
			if list.GetItemCount() > 0 {
				_, command := list.GetItemText(list.GetCurrentItem())
				if command == "" {
					// If secondary text is empty, use primary text
					command, _ = list.GetItemText(list.GetCurrentItem())
				}
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
			current := list.GetCurrentItem()
			if current > 0 {
				list.SetCurrentItem(current - 1)
			}
			return nil
		case tcell.KeyDown:
			// Move selection down
			current := list.GetCurrentItem()
			if current < list.GetItemCount()-1 {
				list.SetCurrentItem(current + 1)
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
