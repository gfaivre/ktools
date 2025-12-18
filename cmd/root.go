package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gfaivre/ktools/internal/config"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/spf13/cobra"
)

var cfg *config.Config
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "ktools",
	Short: "CLI tool to manage files on Infomaniak kDrive",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logging.SetVerbose(verbose)

		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return cfg.Validate()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}

func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.Debug("signal received, cancelling context")
		cancel()
	}()

	logging.Debug("starting command execution")
	err := rootCmd.ExecuteContext(ctx)
	logging.Debug("command returned", "err", err)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
