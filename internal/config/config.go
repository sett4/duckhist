package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	DatabasePath              string `mapstructure:"database_path"`
	CurrentDirectoryHistLimit int    `mapstructure:"current_directory_history_limit"`
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configPath = filepath.Join(home, ".config", "duckhist", "duckhist.toml")
	}

	// Set default values
	viper.SetDefault("database_path", "~/.duckhist.duckdb")
	viper.SetDefault("current_directory_history_limit", 5)

	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml")

	// Use default values if config file does not exist
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Expand tilde
	if config.DatabasePath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		config.DatabasePath = filepath.Join(home, config.DatabasePath[2:])
	}

	return &config, nil
}
