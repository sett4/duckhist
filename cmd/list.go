package cmd

import (
	"fmt"
	"log"

	"duckhist/internal/config"
	"duckhist/internal/history"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List command history",
	Long:  `List all commands in the history database in reverse chronological order.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			log.Fatal(err)
		}

		manager, err := history.NewManagerReadOnly(cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer manager.Close()

		commands, err := manager.ListCommands()
		if err != nil {
			log.Fatal(err)
		}

		for _, command := range commands {
			fmt.Println(command)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
