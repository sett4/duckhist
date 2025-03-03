package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "duckhist",
	Short: "A command history manager using DuckDB",
	Long: `duckhist is a command history manager that stores command history in DuckDB.
It provides functionality to add and list command history.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
