package main

import (
	"fmt"
	"log"
	"strings"

	"duckhist/internal/history"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a command to history",
	Long:  `Add a command to the history database. Use -- to separate the command.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("No command provided")
			return
		}

		manager, err := history.NewManager()
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		command := strings.Join(args, " ")
		if err := manager.AddCommand(command); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Command added to history: %s\n", command)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
