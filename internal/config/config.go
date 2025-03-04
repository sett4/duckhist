package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	DatabasePath string `mapstructure:"database_path"`
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configPath = filepath.Join(home, ".config", "duckhist", "duckhist.toml")
	}

	// デフォルト値の設定
	viper.SetDefault("database_path", "~/.duckhist.duckdb")

	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml")

	// 設定ファイルが存在しない場合は、デフォルト値を使用
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// チルダ展開
	if config.DatabasePath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		config.DatabasePath = filepath.Join(home, config.DatabasePath[2:])
	}

	return &config, nil
}
