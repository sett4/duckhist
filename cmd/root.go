package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "duckhist",
	Short: "A command history manager using DuckDB",
	Long: `duckhist is a command history manager that stores command history in DuckDB.
Command history is stored with additional context like the working directory,
allowing for more intelligent history search and filtering.`,
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/duckhist/duckhist.toml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Config initialization is handled in the commands that need it
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
