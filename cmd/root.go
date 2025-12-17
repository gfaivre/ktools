package cmd

import (
	"fmt"
	"os"

	"github.com/gfaivre/ktools/internal/config"
	"github.com/spf13/cobra"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "ktools",
	Short: "CLI pour gérer les fichiers sur Infomaniak kDrive",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return cfg.Validate()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("token", "", "API token (override config)")
	rootCmd.PersistentFlags().Int("drive-id", 0, "kDrive ID (override config)")
}