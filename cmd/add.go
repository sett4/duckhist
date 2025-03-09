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
	noDedup    bool
)

// CommandAdder handles adding commands to history
type CommandAdder struct {
	config  string
	verbose bool
}

// NewCommandAdder creates a new CommandAdder instance
func NewCommandAdder(config string, verbose bool) *CommandAdder {
	return &CommandAdder{
		config:  config,
		verbose: verbose,
	}
}

// AddCommand adds a command to history
// Returns (isDuplicate, error)
func (ca *CommandAdder) AddCommand(command string, directory string, tty string, sid string, hostname string, username string, noDedup bool) (bool, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		if ca.verbose {
			fmt.Println("Empty command, skipping")
		}
		return false, fmt.Errorf("empty command")
	}

	cfg, err := config.LoadConfig(ca.config)
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}

	// if ca.verbose {
	// 	fmt.Printf("config_path: %s\n", ca.config)
	// 	fmt.Printf("database_path: %s\n", cfg.DatabasePath)
	// }

	manager, err := history.NewManagerReadWrite(cfg.DatabasePath)
	if err != nil {
		return false, fmt.Errorf("failed to create history manager: %w", err)
	}
	defer manager.Close()

	isDup, err := manager.AddCommand(command, directory, tty, sid, hostname, username, noDedup)
	if err != nil {
		return false, fmt.Errorf("failed to add command: %w", err)
	}

	if !isDup && ca.verbose {
		fmt.Printf("Command added to history: %s\n", command)
	}

	return isDup, nil
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a command to history",
	Long:  `Add a command to the history database. Use -- to separate the command.`,
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")

		// Get hostname and username
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("failed to get hostname: %v", err)
		}
		username := os.Getenv("USER")

		// If directory is not specified, use current directory
		if workingDir == "" {
			dir, err := os.Getwd()
			if err != nil {
				log.Fatalf("failed to get current directory: %v", err)
			}
			workingDir = dir
		}

		if tty == "" {
			tty = os.Getenv("TTY")
		}

		adder := NewCommandAdder(cfgFile, verbose)
		isDup, err := adder.AddCommand(command, workingDir, tty, sid, hostname, username, noDedup)
		if err != nil {
			if err.Error() == "empty command" {
				os.Exit(1)
			}
			log.Fatal(err)
		}

		if isDup && !noDedup {
			if verbose {
				fmt.Printf("Skipping duplicate command: %s\n", command)
			}
			os.Exit(0)
		} else if isDup && noDedup {
			if verbose {
				fmt.Printf("Adding duplicate command: %s\n", command)
			}
		}
	},
}

func init() {
	addCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	addCmd.Flags().BoolVar(&noDedup, "no-dedup", false, "allow duplicate commands")
	addCmd.Flags().StringVarP(&workingDir, "directory", "d", "", "directory to record (default is current directory)")
	addCmd.Flags().StringVar(&tty, "tty", "", "TTY (default is $TTY)")
	addCmd.Flags().StringVar(&sid, "sid", "", "Session ID")
	rootCmd.AddCommand(addCmd)
}
