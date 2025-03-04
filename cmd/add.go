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

var verbose bool

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a command to history",
	Long:  `Add a command to the history database. Use -- to separate the command.`,
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")
		command = strings.TrimSpace(command)
		if command == "" {
			if verbose {
				fmt.Println("Empty command, skipping")
			}
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			log.Fatal(err)
		}

		manager, err := history.NewManager(cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		if err := manager.AddCommand(command); err != nil {
			log.Fatal(err)
		}

		if verbose {
			fmt.Printf("Command added to history: %s\n", command)
		}
	},
}

func init() {
	addCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.AddCommand(addCmd)
}
