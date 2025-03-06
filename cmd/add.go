package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"duckhist/internal/config"
	"duckhist/internal/history"

	"github.com/spf13/cobra"
)

var (
	tty        string
	sid        string
	verbose    bool
	workingDir string
)

// CommandAdder handles adding commands to history
type CommandAdder struct {
	configPath string
	verbose    bool
}

// NewCommandAdder creates a new CommandAdder instance
func NewCommandAdder(configPath string, verbose bool) *CommandAdder {
	return &CommandAdder{
		configPath: configPath,
		verbose:    verbose,
	}
}

// AddCommand adds a command to history
func (ca *CommandAdder) AddCommand(command string, directory string, tty string, sid string) error {
	if tty == "" {
		tty = os.Getenv("TTY")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		if ca.verbose {
			fmt.Println("Empty command, skipping")
		}
		return fmt.Errorf("empty command")
	}

	cfg, err := config.LoadConfig(ca.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to create history manager: %w", err)
	}
	defer manager.Close()

	// If directory is not specified, use current directory
	if directory == "" {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = dir
	}

	if err := manager.AddCommand(command, directory, tty, sid); err != nil {
		return fmt.Errorf("failed to add command: %w", err)
	}

	if ca.verbose {
		fmt.Printf("Command added to history: %s\n", command)
	}

	return nil
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a command to history",
	Long:  `Add a command to the history database. Use -- to separate the command.`,
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")

		adder := NewCommandAdder(cfgFile, verbose)
		if err := adder.AddCommand(command, workingDir, tty, sid); err != nil {
			if err.Error() == "empty command" {
				os.Exit(1)
			}
			log.Fatal(err)
		}
	},
}

func init() {
	addCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	addCmd.Flags().StringVarP(&workingDir, "directory", "d", "", "directory to record (default is current directory)")
	addCmd.Flags().StringVar(&tty, "tty", "", "TTY (default is $TTY)")
	addCmd.Flags().StringVar(&sid, "sid", "", "Session ID")
	rootCmd.AddCommand(addCmd)
}
