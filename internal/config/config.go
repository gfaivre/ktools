package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	APIToken string `mapstructure:"api_token"`
	DriveID  int    `mapstructure:"drive_id"`
	BaseURL  string `mapstructure:"base_url"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Config paths: ~/.config/ktools/ or ~/.ktools/
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	viper.AddConfigPath(filepath.Join(home, ".config", "ktools"))
	viper.AddConfigPath(filepath.Join(home, ".ktools"))
	viper.AddConfigPath(".")

	// Default values
	viper.SetDefault("base_url", "https://api.infomaniak.com")

	// Environment variables
	viper.SetEnvPrefix("KTOOLS")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config read error: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config parse error: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.APIToken == "" {
		return fmt.Errorf("api_token required (config or KTOOLS_API_TOKEN)")
	}
	if c.DriveID == 0 {
		return fmt.Errorf("drive_id required (config or KTOOLS_DRIVE_ID)")
	}
	return nil
}
